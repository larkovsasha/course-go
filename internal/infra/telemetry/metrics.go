package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

type activeTrips interface {
	CountActive(ctx context.Context) (int64, error)
}

type TripMetrics struct {
	created      metric.Int64Counter
	completed    metric.Int64Counter
	registration metric.Registration
}

func NewTripMetrics(meterProvider metric.MeterProvider, trips activeTrips) (*TripMetrics, error) {
	meter := meterProvider.Meter("trip-service/trips")
	created, err := meter.Int64Counter("tripgo.trips.created")
	if err != nil {
		return nil, fmt.Errorf("create trips created metric: %w", err)
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed")
	if err != nil {
		return nil, fmt.Errorf("create trips completed metric: %w", err)
	}
	active, err := meter.Int64ObservableGauge("tripgo.trips.active")
	if err != nil {
		return nil, fmt.Errorf("create active trips metric: %w", err)
	}

	registration, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		count, err := trips.CountActive(ctx)
		if err != nil {
			observer.ObserveInt64(active, 0)
			return nil
		}
		observer.ObserveInt64(active, count)
		return nil
	}, active)
	if err != nil {
		return nil, fmt.Errorf("register active trips metric: %w", err)
	}

	return &TripMetrics{
		created:      created,
		completed:    completed,
		registration: registration,
	}, nil
}

func (m *TripMetrics) TripCreated(ctx context.Context) {
	m.created.Add(ctx, 1)
}

func (m *TripMetrics) TripCompleted(ctx context.Context) {
	m.completed.Add(ctx, 1)
}

func (m *TripMetrics) Close() error {
	return m.registration.Unregister()
}

type HTTPMetrics struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

func NewHTTPMetrics(meterProvider metric.MeterProvider) (*HTTPMetrics, error) {
	meter := meterProvider.Meter("trip-service/http")
	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP duration metric: %w", err)
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests")
	if err != nil {
		return nil, fmt.Errorf("create HTTP active requests metric: %w", err)
	}

	return &HTTPMetrics{duration: duration, active: active}, nil
}

func (m *HTTPMetrics) RequestStarted(ctx context.Context, options ...metric.AddOption) {
	m.active.Add(ctx, 1, options...)
}

func (m *HTTPMetrics) RequestFinished(
	ctx context.Context,
	duration float64,
	activeOptions []metric.AddOption,
	durationOptions []metric.RecordOption,
) {
	m.active.Add(ctx, -1, activeOptions...)
	m.duration.Record(ctx, duration, durationOptions...)
}
