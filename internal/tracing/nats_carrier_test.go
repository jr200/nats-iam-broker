package tracing

import (
	"testing"

	"github.com/nats-io/nats.go"
)

func TestNATSHeaderCarrier_Get(t *testing.T) {
	h := nats.Header{}
	h.Set("traceparent", "00-abc-def-01")
	h.Set("X-Custom", "value")
	carrier := NATSHeaderCarrier(h)

	t.Run("existing key", func(t *testing.T) {
		if got := carrier.Get("traceparent"); got != "00-abc-def-01" {
			t.Errorf("Get(traceparent) = %q, want %q", got, "00-abc-def-01")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		if got := carrier.Get("nonexistent"); got != "" {
			t.Errorf("Get(nonexistent) = %q, want empty string", got)
		}
	})

	t.Run("case mismatch returns empty", func(t *testing.T) {
		// nats.Header stores keys as-is via textproto canonicalization;
		// Get with a different casing may not match.
		if got := carrier.Get("TRACEPARENT"); got != "" {
			t.Errorf("Get(TRACEPARENT) = %q, want empty (case mismatch)", got)
		}
	})
}

func TestNATSHeaderCarrier_Set(t *testing.T) {
	h := nats.Header{}
	carrier := NATSHeaderCarrier(h)

	carrier.Set("traceparent", "00-abc-def-01")
	if got := h.Get("traceparent"); got != "00-abc-def-01" {
		t.Errorf("after Set, header value = %q, want %q", got, "00-abc-def-01")
	}

	t.Run("overwrite existing", func(t *testing.T) {
		carrier.Set("traceparent", "00-xyz-uvw-00")
		if got := h.Get("traceparent"); got != "00-xyz-uvw-00" {
			t.Errorf("after overwrite, header value = %q, want %q", got, "00-xyz-uvw-00")
		}
	})
}

func TestNATSHeaderCarrier_Keys(t *testing.T) {
	t.Run("empty headers", func(t *testing.T) {
		carrier := NATSHeaderCarrier(nats.Header{})
		keys := carrier.Keys()
		if len(keys) != 0 {
			t.Errorf("Keys() returned %d keys, want 0", len(keys))
		}
	})

	t.Run("multiple keys", func(t *testing.T) {
		h := nats.Header{}
		h.Set("traceparent", "val1")
		h.Set("tracestate", "val2")
		h.Set("X-Custom", "val3")
		carrier := NATSHeaderCarrier(h)

		keys := carrier.Keys()
		if len(keys) != 3 {
			t.Fatalf("Keys() returned %d keys, want 3: %v", len(keys), keys)
		}

		// Verify all expected keys are present (order is non-deterministic).
		keySet := make(map[string]bool, len(keys))
		for _, k := range keys {
			keySet[k] = true
		}
		for _, want := range []string{"traceparent", "tracestate", "X-Custom"} {
			if !keySet[want] {
				// nats.Header may canonicalise — also accept the canonical form.
				found := false
				for k := range keySet {
					if k == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Keys() missing %q; got %v", want, keys)
				}
			}
		}
	})
}
