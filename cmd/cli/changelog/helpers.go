package changelog

import (
	"os"
	"strings"

	"go.uber.org/zap"
)

// LoggerProvider yields a zap logger for command execution.
type LoggerProvider func() *zap.Logger

func resolveLogger(provider LoggerProvider) *zap.Logger {
	if provider == nil {
		return zap.NewNop()
	}
	logger := provider()
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

func lookupEnvironmentValue(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	return strings.TrimSpace(value), ok
}
