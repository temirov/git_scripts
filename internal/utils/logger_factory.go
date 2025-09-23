package utils

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	logLevelDebugStringConstant          = "debug"
	logLevelInfoStringConstant           = "info"
	logLevelWarnStringConstant           = "warn"
	logLevelErrorStringConstant          = "error"
	logFormatStructuredStringConstant    = "structured"
	logFormatConsoleStringConstant       = "console"
	jsonZapEncodingStringConstant        = "json"
	consoleZapEncodingStringConstant     = "console"
	unsupportedLogLevelTemplateConstant  = "unsupported log level: %s"
	unsupportedLogFormatTemplateConstant = "unsupported log format: %s"
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

// LogFormat enumerates supported logger output encodings.
type LogFormat string

// Exported log format constants for reuse across packages.
const (
	LogFormatStructured LogFormat = LogFormat(logFormatStructuredStringConstant)
	LogFormatConsole    LogFormat = LogFormat(logFormatConsoleStringConstant)
)

// LoggerFactory builds zap.Logger instances with consistent configuration.
type LoggerFactory struct{}

var logLevelMapping = map[LogLevel]zapcore.Level{
	LogLevelDebug: zapcore.DebugLevel,
	LogLevelInfo:  zapcore.InfoLevel,
	LogLevelWarn:  zapcore.WarnLevel,
	LogLevelError: zapcore.ErrorLevel,
}

var logFormatEncodingMapping = map[LogFormat]string{
	LogFormatStructured: jsonZapEncodingStringConstant,
	LogFormatConsole:    consoleZapEncodingStringConstant,
}

// NewLoggerFactory constructs a new logger factory.
func NewLoggerFactory() *LoggerFactory {
	return &LoggerFactory{}
}

// CreateLogger produces a zap.Logger honoring the requested log level and format.
func (factory *LoggerFactory) CreateLogger(requestedLogLevel LogLevel, requestedLogFormat LogFormat) (*zap.Logger, error) {
	zapLogLevel, levelExists := logLevelMapping[requestedLogLevel]
	if !levelExists {
		return nil, fmt.Errorf(unsupportedLogLevelTemplateConstant, requestedLogLevel)
	}

	encoding, formatExists := logFormatEncodingMapping[requestedLogFormat]
	if !formatExists {
		return nil, fmt.Errorf(unsupportedLogFormatTemplateConstant, requestedLogFormat)
	}

	configuration := zap.NewProductionConfig()
	configuration.Level = zap.NewAtomicLevelAt(zapLogLevel)
	configuration.Encoding = encoding

	logger, buildError := configuration.Build()
	if buildError != nil {
		return nil, buildError
	}

	return logger, nil
}
