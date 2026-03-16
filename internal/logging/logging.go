package logging

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Setup initializes the global zap logger with the given level and format.
// Level strings: "disabled", "panic", "fatal", "error", "warn", "info", "debug", "trace".
// Trace is mapped to Debug since zap has no trace level.
// If humanReadable is true, uses a development (console) encoder.
func Setup(level string, humanReadable bool) {
	zapLevel := parseLevel(level)

	var cfg zap.Config
	if humanReadable {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	logger, err := cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		// Fallback to nop logger if config fails
		logger = zap.NewNop()
	}

	zap.ReplaceGlobals(logger)
}

func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "disabled":
		// Use FatalLevel + 1 to effectively disable all logging
		return zapcore.FatalLevel + 1
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	case "error":
		return zapcore.ErrorLevel
	case "warn":
		return zapcore.WarnLevel
	case "info":
		return zapcore.InfoLevel
	case "debug", "trace":
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}
