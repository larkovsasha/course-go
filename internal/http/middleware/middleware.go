package middleware

import (
	"context"
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
)

type routePatternKey struct{}

type recoveryContextKey struct{}

type recoveryContext struct {
	ctx context.Context
}

func New(next http.Handler, routes routeMatcher, logger *slog.Logger, metrics httpMetrics) http.Handler {
	handler := Recover(Tracing(Logging(Metrics(next, metrics), logger)), logger)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := "unknown"
		routeContext := chi.NewRouteContext()
		if routes.Match(routeContext, r.Method, r.URL.Path) {
			route = routeContext.RoutePattern()
		}
		ctx := context.WithValue(r.Context(), routePatternKey{}, route)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Recover(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := newResponseWriter(w)
		recovery := &recoveryContext{ctx: r.Context()}
		ctx := context.WithValue(r.Context(), recoveryContextKey{}, recovery)
		r = r.WithContext(ctx)
		defer func() {
			panicValue := recover()
			if panicValue == nil {
				return
			}

			logger.ErrorContext(
				recovery.ctx,
				"panic recovered",
				"panic", panicValue,
				"stack", string(debug.Stack()),
			)
			if writer.wroteHeader {
				return
			}
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
			writer.Header().Set("Content-Type", "application/problem+json")
			writer.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(writer).Encode(problem); err != nil {
				logger.ErrorContext(recovery.ctx, "write panic response", "error", err)
			}
		}()

		next.ServeHTTP(writer, r)
	})
}

func Tracing(next http.Handler) http.Handler {
	traced := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := newResponseWriter(w)
		if recovery, ok := r.Context().Value(recoveryContextKey{}).(*recoveryContext); ok {
			recovery.ctx = r.Context()
		}
		completed := false
		defer func() {
			status := responseStatus(writer, completed)
			route := routePattern(r)
			span := trace.SpanFromContext(r.Context())
			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", status),
			)
			if !completed {
				span.SetStatus(codes.Error, "panic")
				span.SetAttributes(attribute.String("error.type", "panic"))
			} else if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
				span.SetAttributes(attribute.String("error.type", "http."+strconv.Itoa(status)))
			}
		}()
		next.ServeHTTP(writer, r)
		completed = true
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
		completed := false
		defer func() {
			status := responseStatus(writer, completed)
			log := logger.InfoContext
			if status >= http.StatusInternalServerError {
				log = logger.ErrorContext
			}
			log(
				r.Context(),
				"request handled",
				"method", r.Method,
				"route", routePattern(r),
				"status", status,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		}()
		next.ServeHTTP(writer, r)
		completed = true
	})
}

func Metrics(next http.Handler, metrics httpMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		route := routePattern(r)
		activeOptions := []metric.AddOption{metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
		)}
		metrics.RequestStarted(r.Context(), activeOptions...)

		writer := newResponseWriter(w)
		completed := false
		defer func() {
			metrics.RequestFinished(
				r.Context(),
				time.Since(startedAt).Seconds(),
				activeOptions,
				[]metric.RecordOption{metric.WithAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("http.route", route),
					attribute.Int("http.response.status_code", responseStatus(writer, completed)),
				)},
			)
		}()

		next.ServeHTTP(writer, r)
		completed = true
	})
}

func routePattern(r *http.Request) string {
	if route, ok := r.Context().Value(routePatternKey{}).(string); ok {
		return route
	}
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if route := routeContext.RoutePattern(); route != "" {
			return route
		}
	}
	return "unknown"
}

func responseStatus(writer *responseWriter, completed bool) int {
	if !completed && !writer.wroteHeader {
		return http.StatusInternalServerError
	}
	return writer.status
}
