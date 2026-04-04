package tracing

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
)

// NATSHeaderCarrier adapts nats.Header (which is http.Header) to the
// propagation.TextMapCarrier interface for injecting/extracting W3C
// trace context via NATS message headers.
type NATSHeaderCarrier nats.Header

func (c NATSHeaderCarrier) Get(key string) string {
	return nats.Header(c).Get(key)
}

func (c NATSHeaderCarrier) Set(key, value string) {
	nats.Header(c).Set(key, value)
}

func (c NATSHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceContext injects the current span's trace context into NATS headers.
// If headers is nil, a new nats.Header is created.
func InjectTraceContext(ctx context.Context, headers nats.Header) nats.Header {
	if headers == nil {
		headers = nats.Header{}
	}
	otel.GetTextMapPropagator().Inject(ctx, NATSHeaderCarrier(headers))
	return headers
}

// ExtractTraceContext extracts trace context from NATS message headers.
// Returns a context carrying the remote span context.
func ExtractTraceContext(headers nats.Header) context.Context {
	if headers == nil {
		return context.Background()
	}
	return otel.GetTextMapPropagator().Extract(context.Background(), NATSHeaderCarrier(headers))
}
