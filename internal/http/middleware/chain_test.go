package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	tripmiddleware "github.com/larkovsasha/course-go/internal/http/middleware"
)

type contextLogHandler struct {
	slog.Handler
	contexts map[string]trace.SpanContext
}

func (h *contextLogHandler) Handle(ctx context.Context, record slog.Record) error {
	h.contexts[record.Message] = trace.SpanContextFromContext(ctx)
	return h.Handler.Handle(ctx, record)
}

func TestMiddlewareChain(t *testing.T) {
	const (
		tripPath  = "/trips/123"
		tripRoute = "/trips/{tripId}"
	)
	tests := []struct {
		name, method, path string
		status             int
		panic              bool
		implicit           bool
		duplicateHeader    bool
		wantStatus         int
		wantRoute          string
	}{
		{name: "success", method: http.MethodGet, path: tripPath, status: 204, wantStatus: 204, wantRoute: tripRoute},
		{name: "implicit success", method: http.MethodGet, path: "/trips/456", implicit: true, wantStatus: 200, wantRoute: tripRoute},
		{name: "first status wins", method: http.MethodGet, path: tripPath, status: 201, duplicateHeader: true, wantStatus: 201, wantRoute: tripRoute},
		{name: "client error", method: http.MethodGet, path: tripPath, status: 400, wantStatus: 400, wantRoute: tripRoute},
		{name: "server error", method: http.MethodGet, path: tripPath, status: 503, wantStatus: 503, wantRoute: tripRoute},
		{name: "panic", method: http.MethodGet, path: tripPath, panic: true, wantStatus: 500, wantRoute: tripRoute},
		{name: "not found", method: http.MethodGet, path: "/missing/123", wantStatus: 404, wantRoute: "unknown"},
		{name: "method not allowed", method: http.MethodPost, path: tripPath, wantStatus: 405, wantRoute: "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
			previous := otel.GetTracerProvider()
			otel.SetTracerProvider(provider)
			t.Cleanup(func() {
				otel.SetTracerProvider(previous)
				if err := provider.Shutdown(context.Background()); err != nil {
					t.Errorf("shutdown tracing: %v", err)
				}
			})
			var logs bytes.Buffer
			logHandler := &contextLogHandler{Handler: slog.NewJSONHandler(&logs, nil), contexts: make(map[string]trace.SpanContext)}
			logger := slog.New(logHandler)
			metrics := NewMockhttpMetrics(gomock.NewController(t))
			active := false
			gomock.InOrder(
				metrics.EXPECT().RequestStarted(gomock.Any(), gomock.Any()).
					Do(func(ctx context.Context, options ...metric.AddOption) {
						active = true
						if !trace.SpanContextFromContext(ctx).IsValid() {
							t.Error("metrics started outside HTTP span")
						}
						config := metric.NewAddConfig(options)
						assertHTTPAttributes(t, config.Attributes(), test.method, test.wantRoute)
					}),
				metrics.EXPECT().RequestFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Do(func(_ context.Context, duration float64, activeOptions []metric.AddOption, durationOptions []metric.RecordOption) {
						active = false
						if duration < 0 {
							t.Errorf("negative duration: %v", duration)
						}
						activeConfig := metric.NewAddConfig(activeOptions)
						assertHTTPAttributes(t, activeConfig.Attributes(), test.method, test.wantRoute)
						config := metric.NewRecordConfig(durationOptions)
						attrs := config.Attributes()
						assertHTTPAttributes(t, attrs, test.method, test.wantRoute)
						value, _ := attrs.Value("http.response.status_code")
						if value.AsInt64() != int64(test.wantStatus) {
							t.Errorf("metric status = %v, want %d", value, test.wantStatus)
						}
					}),
			)
			routes := chi.NewRouter()
			routes.Get("/trips/{tripId}", func(w http.ResponseWriter, _ *http.Request) {
				if !active {
					t.Error("handler called before request metric started")
				}
				if logs.Len() != 0 {
					t.Error("request logged before the handler completed")
				}
				if test.panic {
					panic("test panic")
				}
				if test.implicit {
					if _, err := io.WriteString(w, "ok"); err != nil {
						t.Errorf("write response: %v", err)
					}
					return
				}
				w.WriteHeader(test.status)
				if test.duplicateHeader {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			})
			handler := tripmiddleware.New(routes, routes, logger, metrics)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if active {
				t.Error("request remained active after handler finished")
			}
			if recorder.Code != test.wantStatus {
				t.Errorf("HTTP status = %d, want %d", recorder.Code, test.wantStatus)
			}
			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("span count = %d, want 1", len(spans))
			}
			if spans[0].Name != test.method+" "+test.wantRoute {
				t.Errorf("span name = %q", spans[0].Name)
			}
			if (spans[0].Status.Code == codes.Error) != (test.wantStatus >= 500) {
				t.Errorf("span status = %v for HTTP %d", spans[0].Status, test.wantStatus)
			}
			if test.panic && !logHandler.contexts["panic recovered"].Equal(spans[0].SpanContext) {
				t.Error("panic log lost the HTTP span context")
			}
			if !logHandler.contexts["request handled"].Equal(spans[0].SpanContext) {
				t.Error("request log lost the HTTP span context")
			}
			assertRequestLog(t, &logs, test.wantStatus, test.wantRoute)
			if test.panic {
				var problem struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil || problem.Code != "internal_error" {
					t.Errorf("panic response = %s, decode error = %v", recorder.Body.String(), err)
				}
			}
		})
	}
}

func assertHTTPAttributes(t *testing.T, attrs attribute.Set, method, route string) {
	t.Helper()
	for key, want := range map[attribute.Key]string{"http.request.method": method, "http.route": route} {
		value, ok := attrs.Value(key)
		if !ok || value.AsString() != want {
			t.Errorf("attribute %s = %v, want %q", key, value, want)
		}
	}
}

func assertRequestLog(t *testing.T, logs *bytes.Buffer, status int, route string) {
	t.Helper()
	decoder := json.NewDecoder(logs)
	count := 0
	for decoder.More() {
		var entry struct {
			Message string `json:"msg"`
			Level   string `json:"level"`
			Status  int    `json:"status"`
			Route   string `json:"route"`
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("decode log: %v", err)
		}
		if entry.Message != "request handled" {
			continue
		}
		count++
		level := "INFO"
		if status >= 500 {
			level = "ERROR"
		}
		if entry.Status != status || entry.Route != route || entry.Level != level {
			t.Errorf("request log = %+v, want %d %s %s", entry, status, route, level)
		}
	}
	if count != 1 {
		t.Errorf("request log count = %d, want 1", count)
	}
}
