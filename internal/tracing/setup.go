// Package tracing provides OpenTelemetry TracerProvider setup and trace context utilities.
package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

// Setup initialises the OTel TracerProvider.
//
// Exporter selection:
//   - OTEL_EXPORTER_OTLP_ENDPOINT set → OTLP gRPC exporter
//   - OTEL_TRACES_EXPORTER=console   → stdout (pretty-printed, for local dev)
//   - Neither set                    → tracing disabled (no-op)
//
// Sampling is controlled via the OTEL_TRACES_SAMPLER env var (SDK default: always_on).
//
// Returns a shutdown function that must be called on exit.
func Setup(ctx context.Context, serviceName, serviceVersion string) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	tracesExporter := os.Getenv("OTEL_TRACES_EXPORTER")

	// If no exporter is configured, tracing is disabled — return a no-op shutdown.
	if endpoint == "" && tracesExporter != "console" {
		zap.L().Info("OTel tracing disabled (set OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_TRACES_EXPORTER=console to enable)")
		noop := func(context.Context) error { return nil }
		return noop, nil
	}

	environment := os.Getenv("OTEL_DEPLOYMENT_ENVIRONMENT")

	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.TelemetrySDKLanguageGo,
		),
	}
	if environment != "" {
		attrs = append(attrs, resource.WithAttributes(
			semconv.DeploymentEnvironmentKey.String(environment),
		))
	}

	res, err := resource.New(ctx, attrs...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	if endpoint != "" {
		exporter, err = otlptracegrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		zap.L().Info("OTel tracing configured", zap.String("exporter", "otlp-grpc"), zap.String("endpoint", endpoint))
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
		zap.L().Info("OTel tracing configured", zap.String("exporter", "console"))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// SetupWithExporter initialises a TracerProvider with the given exporter,
// bypassing env-var detection. Intended for testing.
func SetupWithExporter(ctx context.Context, serviceName, serviceVersion string, exporter sdktrace.SpanExporter) (func(context.Context) error, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.TelemetrySDKLanguageGo,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
