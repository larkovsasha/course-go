package router

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

type httpMetrics interface {
	RequestStarted(ctx context.Context, options ...metric.AddOption)
	RequestFinished(ctx context.Context, duration float64, activeOptions []metric.AddOption, durationOptions []metric.RecordOption)
}
