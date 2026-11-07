//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package middleware

import (
	"context"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/metric"
)

type routeMatcher interface {
	Match(routeContext *chi.Context, method, path string) bool
}

type httpMetrics interface {
	RequestStarted(ctx context.Context, options ...metric.AddOption)
	RequestFinished(ctx context.Context, duration float64, activeOptions []metric.AddOption, durationOptions []metric.RecordOption)
}
