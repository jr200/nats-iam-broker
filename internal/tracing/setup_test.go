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

	result, err := Setup(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ZapCore != nil {
		t.Error("expected nil ZapCore when tracing is disabled")
	}

	// Calling shutdown should succeed (no-op).
	if err := result.Shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}
}

func TestSetup_ConsoleExporter(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "console")

	result, err := Setup(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Console exporter has no OTLP endpoint, so no log bridge
	if result.ZapCore != nil {
		t.Error("expected nil ZapCore for console exporter (no OTLP endpoint)")
	}

	if err := result.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestSetup_WithEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "console")
	t.Setenv("OTEL_DEPLOYMENT_ENVIRONMENT", "test")

	result, err := Setup(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if err := result.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
