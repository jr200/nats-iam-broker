package tracing

import (
	"context"
	"testing"
)

func TestSetup_NoExporterConfigured_ReturnsNoop(t *testing.T) {
	// With no OTEL_EXPORTER_OTLP_ENDPOINT and no OTEL_TRACES_EXPORTER set,
	// Setup should return a no-op shutdown without error.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "")

	shutdown, err := Setup(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Calling shutdown should succeed (no-op).
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}
}

func TestSetup_ConsoleExporter(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "console")

	shutdown, err := Setup(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestSetup_WithEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "console")
	t.Setenv("OTEL_DEPLOYMENT_ENVIRONMENT", "test")

	shutdown, err := Setup(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
