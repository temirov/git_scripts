package utils

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	logLevelDebugStringConstant         = "debug"
	logLevelInfoStringConstant          = "info"
	logLevelWarnStringConstant          = "warn"
	logLevelErrorStringConstant         = "error"
	unsupportedLogLevelTemplateConstant = "unsupported log level: %s"
)

// LogLevel enumerates supported logging granularities.
type LogLevel string

// Exported log level constants for reuse across packages.
const (
	LogLevelDebug LogLevel = LogLevel(logLevelDebugStringConstant)
	LogLevelInfo  LogLevel = LogLevel(logLevelInfoStringConstant)
	LogLevelWarn  LogLevel = LogLevel(logLevelWarnStringConstant)
	LogLevelError LogLevel = LogLevel(logLevelErrorStringConstant)
)

// LoggerFactory builds zap.Logger instances with consistent configuration.
type LoggerFactory struct{}

var logLevelMapping = map[LogLevel]zapcore.Level{
	LogLevelDebug: zapcore.DebugLevel,
	LogLevelInfo:  zapcore.InfoLevel,
	LogLevelWarn:  zapcore.WarnLevel,
	LogLevelError: zapcore.ErrorLevel,
}

// NewLoggerFactory constructs a new logger factory.
func NewLoggerFactory() *LoggerFactory {
	return &LoggerFactory{}
}

// CreateLogger produces a zap.Logger honoring the requested log level.
func (factory *LoggerFactory) CreateLogger(requestedLogLevel LogLevel) (*zap.Logger, error) {
	zapLogLevel, levelExists := logLevelMapping[requestedLogLevel]
	if !levelExists {
		return nil, fmt.Errorf(unsupportedLogLevelTemplateConstant, requestedLogLevel)
	}

	configuration := zap.NewProductionConfig()
	configuration.Level = zap.NewAtomicLevelAt(zapLogLevel)

	logger, buildError := configuration.Build()
	if buildError != nil {
		return nil, buildError
	}

	return logger, nil
}
