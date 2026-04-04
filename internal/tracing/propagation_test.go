package tracing

import (
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	// Ensure a propagator is registered so extraction works in tests.
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func TestExtractFromTraceparent_ValidTraceparent(t *testing.T) {
	tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	ctx := ExtractFromTraceparent(tp)

	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("expected valid span context from a well-formed traceparent")
	}
	if sc.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("unexpected trace ID: %s", sc.TraceID())
	}
	if sc.SpanID().String() != "00f067aa0ba902b7" {
		t.Errorf("unexpected span ID: %s", sc.SpanID())
	}
	if !sc.IsSampled() {
		t.Error("expected sampled flag to be set")
	}
}

func TestExtractFromTraceparent_Empty(t *testing.T) {
	ctx := ExtractFromTraceparent("")

	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("expected invalid span context from empty traceparent")
	}
}

func TestExtractFromTraceparent_Malformed(t *testing.T) {
	ctx := ExtractFromTraceparent("not-a-valid-traceparent")

	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("expected invalid span context from malformed traceparent")
	}
}

func TestExtractFromTraceparent_NotSampled(t *testing.T) {
	tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00"
	ctx := ExtractFromTraceparent(tp)

	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("expected valid span context")
	}
	if sc.IsSampled() {
		t.Error("expected sampled flag to NOT be set")
	}
}
