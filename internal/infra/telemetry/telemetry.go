package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/larkovsasha/course-go/internal/config"
)

type Telemetry struct {
	Logger         *slog.Logger
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
}

func New(ctx context.Context, cfg *config.Config) (*Telemetry, error) {
	logLevel := new(slog.LevelVar)
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	logger := slog.New(newTraceHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	serviceResource, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.OTELServiceName),
			semconv.ServiceVersion(buildVersion()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	endpoint := strings.TrimRight(cfg.OTLPEndpoint, "/")
	traceExporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(endpoint+"/v1/traces"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	metricExporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpointURL(endpoint+"/v1/metrics"),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	sampler, err := newSampler(cfg.OTELTracesSampler)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(serviceResource),
		sdktrace.WithSampler(sampler),
	)
	metricReader := sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(serviceResource),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Telemetry{
		Logger:         logger,
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
	}, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	return errors.Join(
		t.MeterProvider.Shutdown(ctx),
		t.TracerProvider.Shutdown(ctx),
	)
}

func newSampler(name string) (sdktrace.Sampler, error) {
	switch name {
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	default:
		return nil, fmt.Errorf("unknown traces sampler %q", name)
	}
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}

	return info.Main.Version
}
