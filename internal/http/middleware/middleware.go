package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"

	api "github.com/larkovsasha/course-go/internal/generated"
	"github.com/larkovsasha/course-go/internal/infra/telemetry"
)

func Recover(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			panicValue := recover()
			if panicValue == nil {
				return
			}

			logger.ErrorContext(
				r.Context(),
				"panic recovered",
				"panic", panicValue,
				"stack", string(debug.Stack()),
			)
			detail := "Internal server error"
			instance := r.URL.Path
			problem := api.Problem{
				Type:     "https://tripgo.example/problems/internal-error",
				Title:    "Internal Server Error",
				Status:   http.StatusInternalServerError,
				Code:     "internal_error",
				Detail:   &detail,
				Instance: &instance,
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(problem); err != nil {
				logger.ErrorContext(r.Context(), "write panic response", "error", err)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func Tracing(next http.Handler) http.Handler {
	traced := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := newResponseWriter(w)
		next.ServeHTTP(writer, r)

		route := routePattern(r)
		span := trace.SpanFromContext(r.Context())
		span.SetName(r.Method + " " + route)
		span.SetAttributes(attribute.String("http.route", route))
		if writer.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(writer.status))
			span.SetAttributes(attribute.String("error.type", "http."+strconv.Itoa(writer.status)))
		}
	})

	return otelhttp.NewHandler(
		traced,
		"HTTP",
		otelhttp.WithMeterProvider(metricnoop.NewMeterProvider()),
	)
}

func Logging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		writer := newResponseWriter(w)
		next.ServeHTTP(writer, r)

		log := logger.InfoContext
		if writer.status >= http.StatusInternalServerError {
			log = logger.ErrorContext
		}
		log(
			r.Context(),
			"request handled",
			"method", r.Method,
			"route", routePattern(r),
			"status", writer.status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

func Metrics(next http.Handler, metrics *telemetry.HTTPMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		route := routePattern(r)
		activeOptions := []metric.AddOption{metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
		)}
		metrics.RequestStarted(r.Context(), activeOptions...)

		writer := newResponseWriter(w)
		defer func() {
			metrics.RequestFinished(
				r.Context(),
				time.Since(startedAt).Seconds(),
				activeOptions,
				[]metric.RecordOption{metric.WithAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("http.route", route),
					attribute.Int("http.response.status_code", writer.status),
				)},
			)
		}()

		next.ServeHTTP(writer, r)
	})
}

func routePattern(r *http.Request) string {
	route := chi.RouteContext(r.Context()).RoutePattern()
	if route == "" {
		return "unknown"
	}

	return route
}
