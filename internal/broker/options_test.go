package broker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultServerOptions(t *testing.T) {
	opts := DefaultServerOptions()
	require.NotNil(t, opts)
	assert.False(t, opts.LogSensitive)
	assert.False(t, opts.MetricsEnabled)
	assert.Equal(t, DefaultMetricsPort, opts.MetricsPort)
}

func TestNewServerContext(t *testing.T) {
	t.Run("with nil options uses defaults", func(t *testing.T) {
		ctx := NewServerContext(nil)
		require.NotNil(t, ctx)
		require.NotNil(t, ctx.Options)
		assert.Equal(t, DefaultMetricsPort, ctx.Options.MetricsPort)
	})

	t.Run("with custom options", func(t *testing.T) {
		opts := &Options{
			LogSensitive:   true,
			MetricsEnabled: true,
			MetricsPort:    9090,
		}
		ctx := NewServerContext(opts)
		require.NotNil(t, ctx)
		assert.True(t, ctx.Options.LogSensitive)
		assert.True(t, ctx.Options.MetricsEnabled)
		assert.Equal(t, 9090, ctx.Options.MetricsPort)
	})
}
