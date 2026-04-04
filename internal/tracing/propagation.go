package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
)

// ExtractFromTraceparent extracts an OTel context from a raw W3C traceparent string.
// Returns a context carrying the remote span context, or a background context
// if the traceparent is empty or invalid.
func ExtractFromTraceparent(traceparent string) context.Context {
	if traceparent == "" {
		return context.Background()
	}

	carrier := make(map[string]string)
	carrier["traceparent"] = traceparent

	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(context.Background(), mapCarrier(carrier))
}

// mapCarrier adapts a map[string]string to propagation.TextMapCarrier.
type mapCarrier map[string]string

func (c mapCarrier) Get(key string) string {
	return c[key]
}

func (c mapCarrier) Set(key, value string) {
	c[key] = value
}

func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
