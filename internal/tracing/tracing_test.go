package tracing

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ensurePropagator sets the W3C TraceContext propagator for the duration of the test.
func ensurePropagator(t *testing.T) {
	t.Helper()
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func TestExtractFromTraceparent(t *testing.T) {
	ensurePropagator(t)

	t.Run("valid traceparent", func(t *testing.T) {
		tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		ctx := ExtractFromTraceparent(tp)
		sc := trace.SpanContextFromContext(ctx)
		if !sc.IsValid() {
			t.Fatal("expected valid span context")
		}
		if sc.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Errorf("unexpected trace ID: %s", sc.TraceID())
		}
		if sc.SpanID().String() != "00f067aa0ba902b7" {
			t.Errorf("unexpected span ID: %s", sc.SpanID())
		}
	})

	t.Run("empty traceparent", func(t *testing.T) {
		ctx := ExtractFromTraceparent("")
		sc := trace.SpanContextFromContext(ctx)
		if sc.IsValid() {
			t.Error("expected invalid span context for empty traceparent")
		}
	})

	t.Run("malformed traceparent", func(t *testing.T) {
		ctx := ExtractFromTraceparent("not-a-traceparent")
		sc := trace.SpanContextFromContext(ctx)
		if sc.IsValid() {
			t.Error("expected invalid span context for malformed traceparent")
		}
	})
}

func TestNATSHeaderCarrier_RoundTrip(t *testing.T) {
	ensurePropagator(t)

	// Simulate receiving a traceparent, then propagating it via NATS headers.
	tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	ctx := ExtractFromTraceparent(tp)

	// Inject into NATS headers
	headers := InjectTraceContext(ctx, nil)

	// nats.Header is case-sensitive (unlike http.Header), and the W3C propagator
	// uses lowercase "traceparent"
	traceparent := headers.Get("traceparent")
	if traceparent == "" {
		t.Fatal("expected traceparent header to be set")
	}

	// Extract from NATS headers
	extractedCtx := ExtractTraceContext(headers)
	extractedSC := trace.SpanContextFromContext(extractedCtx)

	if !extractedSC.IsValid() {
		t.Fatal("expected valid span context after round-trip")
	}
	if extractedSC.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace ID mismatch: got %s", extractedSC.TraceID())
	}
}

func TestInjectTraceContext_NilHeaders(t *testing.T) {
	headers := InjectTraceContext(context.Background(), nil)
	if headers == nil {
		t.Fatal("expected non-nil headers even with no active span")
	}
}

func TestInjectTraceContext_ExistingHeaders(t *testing.T) {
	existing := nats.Header{}
	existing.Set("X-Custom", "value")

	headers := InjectTraceContext(context.Background(), existing)
	if headers.Get("X-Custom") != "value" {
		t.Error("existing header was lost")
	}
}

func TestExtractTraceContext_NilHeaders(t *testing.T) {
	ctx := ExtractTraceContext(nil)
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("expected invalid span context from nil headers")
	}
}
