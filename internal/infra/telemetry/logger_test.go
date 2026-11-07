package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandler(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(newTraceHandler(slog.NewJSONHandler(&output, nil)))
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{2},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	logger.InfoContext(ctx, "test message")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if record["service"] != "trip-service" {
		t.Errorf("service = %v, want trip-service", record["service"])
	}
	if record["trace_id"] != spanContext.TraceID().String() {
		t.Errorf("trace_id = %v, want %s", record["trace_id"], spanContext.TraceID())
	}
	if record["span_id"] != spanContext.SpanID().String() {
		t.Errorf("span_id = %v, want %s", record["span_id"], spanContext.SpanID())
	}
}
