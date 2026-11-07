package telemetry

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type queryData struct {
	startedAt  time.Time
	operation  string
	collection string
}

type queryDataKey struct{}

type PGXTracer struct {
	tracer   *otelpgx.Tracer
	duration metric.Float64Histogram
}

func NewPGXTracer(
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
) (*PGXTracer, error) {
	duration, err := meterProvider.Meter("trip-service/database").Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Database operation duration"),
	)
	if err != nil {
		return nil, fmt.Errorf("create database duration metric: %w", err)
	}

	return &PGXTracer{
		tracer: otelpgx.NewTracer(
			otelpgx.WithTracerProvider(tracerProvider),
			otelpgx.WithMeterProvider(metricnoop.NewMeterProvider()),
			otelpgx.WithDisableSQLStatementInAttributes(),
		),
		duration: duration,
	}, nil
}

func (t *PGXTracer) TraceQueryStart(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	ctx = t.tracer.TraceQueryStart(ctx, conn, data)
	operation, collection := queryAttributes(data.SQL)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("db.operation.name", operation),
		attribute.String("db.collection.name", collection),
	)

	return context.WithValue(ctx, queryDataKey{}, queryData{
		startedAt:  time.Now(),
		operation:  operation,
		collection: collection,
	})
}

func (t *PGXTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	query, ok := ctx.Value(queryDataKey{}).(queryData)
	if ok {
		attributes := []attribute.KeyValue{
			attribute.String("db.operation.name", query.operation),
			attribute.String("db.collection.name", query.collection),
		}
		if data.Err != nil {
			errorType := reflect.TypeOf(data.Err).String()
			attributes = append(attributes, attribute.String("error.type", errorType))
			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, data.Err.Error())
			span.SetAttributes(attribute.String("error.type", errorType))
		}

		t.duration.Record(ctx, time.Since(query.startedAt).Seconds(), metric.WithAttributes(attributes...))
	}

	t.tracer.TraceQueryEnd(ctx, conn, data)
}

func queryAttributes(query string) (string, string) {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return "", ""
	}

	operation := strings.ToUpper(fields[0])
	collection := ""
	switch operation {
	case "SELECT", "DELETE":
		collection = wordAfter(fields, "FROM")
	case "INSERT":
		collection = wordAfter(fields, "INTO")
	case "UPDATE":
		if len(fields) > 1 {
			collection = fields[1]
		}
	}

	return operation, strings.Trim(collection, `"`)
}

func wordAfter(fields []string, value string) string {
	for index, field := range fields {
		if strings.EqualFold(field, value) && index+1 < len(fields) {
			return fields[index+1]
		}
	}

	return ""
}
