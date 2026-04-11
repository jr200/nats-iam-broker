//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/jr200-labs/nats-iam-broker/internal/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestAuthCallout_ProducesTraceSpans(t *testing.T) {
	// Register an in-memory exporter BEFORE the broker starts.
	// The broker's own Setup() will see no OTEL env vars and return no-op,
	// but the global TracerProvider is already set to our test provider.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "")

	exporter := tracetest.NewInMemoryExporter()
	shutdown, err := tracing.SetupWithExporter(
		context.Background(), "test-iam-broker", "0.0.1-test", exporter,
	)
	require.NoError(t, err)
	defer func() { _ = shutdown(context.Background()) }()

	// Start the full stack: NATS cluster, mock OIDC, broker
	cluster, oidc, _ := setupTestEnv(t)

	// Mint a valid token and connect — this triggers the auth callout
	token := oidc.MintIDToken(map[string]interface{}{
		"sub":   "bob@acme.com",
		"email": "bob@acme.com",
		"name":  "Bob",
		"aud":   "mockclientid",
	})

	nc, err := cluster.ConnectWithToken(t, token)
	require.NoError(t, err)
	nc.Close()

	// SetupWithExporter uses WithSyncer, so spans are exported immediately.

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "expected at least one span from the auth callout — "+
		"if this fails, getTracer() may be returning a no-op tracer (check init order)")

	// Verify the root span name
	spanNames := make([]string, len(spans))
	for i, s := range spans {
		spanNames[i] = s.Name
	}

	assert.Contains(t, spanNames, "auth.callout.handle",
		"expected auth.callout.handle span; got: %v", spanNames)
	assert.Contains(t, spanNames, "auth.callout.verify_idp",
		"expected auth.callout.verify_idp span; got: %v", spanNames)
	assert.Contains(t, spanNames, "auth.callout.build_claims",
		"expected auth.callout.build_claims span; got: %v", spanNames)
	assert.Contains(t, spanNames, "auth.callout.audit",
		"expected auth.callout.audit span; got: %v", spanNames)
}
