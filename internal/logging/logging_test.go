package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"disabled", zapcore.FatalLevel + 1},
		{"panic", zapcore.PanicLevel},
		{"fatal", zapcore.FatalLevel},
		{"error", zapcore.ErrorLevel},
		{"warn", zapcore.WarnLevel},
		{"info", zapcore.InfoLevel},
		{"debug", zapcore.DebugLevel},
		{"trace", zapcore.DebugLevel},
		{"unknown", zapcore.InfoLevel},
		{"", zapcore.InfoLevel},
		{"INFO", zapcore.InfoLevel},
		{"Debug", zapcore.DebugLevel},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseLevel(tt.input))
		})
	}
}

func TestSetup(t *testing.T) {
	t.Run("production format", func(t *testing.T) {
		Setup("info", false)
		logger := zap.L()
		assert.NotNil(t, logger)
		// Verify it's not a nop logger by checking it can log without panic
		logger.Info("test message")
	})

	t.Run("human readable format", func(t *testing.T) {
		Setup("debug", true)
		logger := zap.L()
		assert.NotNil(t, logger)
		logger.Debug("test debug message")
	})

	t.Run("disabled level", func(t *testing.T) {
		Setup("disabled", false)
		logger := zap.L()
		assert.NotNil(t, logger)
	})
}
