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

func TestMergeOptions(t *testing.T) {
	t.Run("defaults only", func(t *testing.T) {
		merged := MergeOptions(Options{}, &Options{}, nil)
		assert.False(t, merged.LogSensitive)
		assert.False(t, merged.MetricsEnabled)
		assert.Equal(t, DefaultMetricsPort, merged.MetricsPort)
		assert.False(t, merged.WatchConfig)
	})

	t.Run("yaml only", func(t *testing.T) {
		yamlOpts := Options{
			LogSensitive:   true,
			MetricsEnabled: true,
			MetricsPort:    9090,
			WatchConfig:    true,
		}
		merged := MergeOptions(yamlOpts, &Options{}, nil)
		assert.True(t, merged.LogSensitive)
		assert.True(t, merged.MetricsEnabled)
		assert.Equal(t, 9090, merged.MetricsPort)
		assert.True(t, merged.WatchConfig)
	})

	t.Run("cli only", func(t *testing.T) {
		cliOpts := &Options{
			LogSensitive:   true,
			MetricsEnabled: true,
			MetricsPort:    9090,
			WatchConfig:    true,
		}
		cliFlags := map[string]bool{
			"log-sensitive": true,
			"metrics":       true,
			"metrics-port":  true,
			"watch":         true,
		}
		merged := MergeOptions(Options{}, cliOpts, cliFlags)
		assert.True(t, merged.LogSensitive)
		assert.True(t, merged.MetricsEnabled)
		assert.Equal(t, 9090, merged.MetricsPort)
		assert.True(t, merged.WatchConfig)
	})

	t.Run("cli overrides yaml", func(t *testing.T) {
		yamlOpts := Options{
			LogSensitive:   true,
			MetricsEnabled: true,
			MetricsPort:    9090,
			WatchConfig:    true,
		}
		cliOpts := &Options{
			MetricsEnabled: false,
			MetricsPort:    3000,
		}
		cliFlags := map[string]bool{
			"metrics":      true,
			"metrics-port": true,
		}
		merged := MergeOptions(yamlOpts, cliOpts, cliFlags)
		assert.True(t, merged.LogSensitive)   // from YAML
		assert.False(t, merged.MetricsEnabled) // CLI override
		assert.Equal(t, 3000, merged.MetricsPort) // CLI override
		assert.True(t, merged.WatchConfig)     // from YAML
	})

	t.Run("yaml metrics port zero uses default", func(t *testing.T) {
		yamlOpts := Options{MetricsPort: 0}
		merged := MergeOptions(yamlOpts, &Options{}, nil)
		assert.Equal(t, DefaultMetricsPort, merged.MetricsPort)
	})
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
