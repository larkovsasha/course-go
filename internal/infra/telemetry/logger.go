package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type traceHandler struct {
	next slog.Handler
}

func newTraceHandler(next slog.Handler) slog.Handler {
	return &traceHandler{next: next}
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, record slog.Record) error {
	spanContext := trace.SpanContextFromContext(ctx)
	traceID := ""
	spanID := ""
	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
		spanID = spanContext.SpanID().String()
	}

	record.AddAttrs(
		slog.String("service", "trip-service"),
		slog.String("trace_id", traceID),
		slog.String("span_id", spanID),
	)
	return h.next.Handle(ctx, record)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{next: h.next.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{next: h.next.WithGroup(name)}
}
