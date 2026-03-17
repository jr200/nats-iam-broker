//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthCallout_JetStream(t *testing.T) {
	cluster, oidc, _ := setupTestEnv(t)

	// Bob matches sub-based binding which includes the streaming role
	token := oidc.MintIDToken(map[string]interface{}{
		"sub":   "bob@acme.com",
		"email": "bob@acme.com",
		"name":  "Bob",
		"aud":   "mockclientid",
	})

	nc, err := cluster.ConnectWithToken(t, token)
	require.NoError(t, err)
	defer nc.Close()

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	ctx := t.Context()

	// Create a stream
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test.stream.>"},
	})
	require.NoError(t, err)

	// Publish messages
	for i := 0; i < 3; i++ {
		_, err := js.Publish(ctx, "test.stream.events", []byte("event"))
		require.NoError(t, err)
	}

	// Create a consumer and read messages
	cons, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          "test_consumer",
		FilterSubject: "test.stream.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	require.NoError(t, err)

	// Fetch messages
	batch, err := cons.Fetch(3, jetstream.FetchMaxWait(2*time.Second))
	require.NoError(t, err)

	count := 0
	for msg := range batch.Messages() {
		assert.Equal(t, "event", string(msg.Data()))
		require.NoError(t, msg.Ack())
		count++
	}
	assert.Equal(t, 3, count)

	// Cleanup
	err = js.DeleteStream(ctx, "test_stream")
	require.NoError(t, err)
}

func TestAuthCallout_JetStream_DeniedWithoutStreamingRole(t *testing.T) {
	cluster, oidc, _ := setupTestEnv(t)

	// Alice via aud binding only gets can-pubsub, NOT streaming
	token := oidc.MintIDToken(map[string]interface{}{
		"sub":   "alice@acme.com",
		"email": "alice@acme.com",
		"aud":   "mockclientid",
	})

	nc, err := cluster.ConnectWithToken(t, token)
	require.NoError(t, err)
	defer nc.Close()

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Attempting to create a stream should fail — no JS API permissions
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test.stream.>"},
	})
	assert.Error(t, err, "should not be able to create stream without streaming role")
}
