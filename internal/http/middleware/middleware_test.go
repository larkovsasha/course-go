package middleware_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	api "github.com/larkovsasha/course-go/internal/generated"
	tripmiddleware "github.com/larkovsasha/course-go/internal/http/middleware"
)

func TestRecover(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := tripmiddleware.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}), logger)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var problem api.Problem
	if err := json.NewDecoder(recorder.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != "internal_error" {
		t.Errorf("problem code = %s, want internal_error", problem.Code)
	}
}

func TestTracing(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	handler := tripmiddleware.Tracing(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	routeContext := chi.NewRouteContext()
	routeContext.RoutePatterns = append(routeContext.RoutePatterns, "/api/v1/trips/{tripId}")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/123", nil)
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "GET /api/v1/trips/{tripId}" {
		t.Errorf("span name = %s", spans[0].Name)
	}
}
