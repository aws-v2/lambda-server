package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitTelemetry initializes OpenTelemetry with a Prometheus exporter for metrics and a basic TracerProvider.
func InitTelemetry(serviceName string) (func(), error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 1. Setup Metrics with Prometheus
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// 2. Setup Tracing
	// For now, we use a simple TraceProvider that doesn't export anywhere specific (but can be extended)
	// Traces will still be available via context and can be logged.
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Shutdown function
	cleanup := func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			fmt.Printf("Error shutting down tracer provider: %v\n", err)
		}
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			fmt.Printf("Error shutting down meter provider: %v\n", err)
		}
	}

	return cleanup, nil
}
