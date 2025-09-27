package utils

import (
	"fmt"
	"os"

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
	timeFieldNameConstant                = "time"
	levelFieldNameConstant               = "level"
	structuredMessageFieldNameConstant   = "msg"
	consoleMessageFieldNameConstant      = "message"
	nameFieldNameConstant                = "logger"
	callerFieldNameConstant              = "caller"
	stacktraceFieldNameConstant          = "stacktrace"
	humanReadableTimeLayoutConstant      = "15:04:05"
	emptyStringConstant                  = ""
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

// LoggerOutputs bundles diagnostic and console loggers.
type LoggerOutputs struct {
	DiagnosticLogger *zap.Logger
	ConsoleLogger    *zap.Logger
}

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
	outputs, creationError := factory.CreateLoggerOutputs(requestedLogLevel, requestedLogFormat)
	if creationError != nil {
		return nil, creationError
	}
	return outputs.DiagnosticLogger, nil
}

// CreateLoggerOutputs builds both diagnostic and console loggers for the requested configuration.
func (factory *LoggerFactory) CreateLoggerOutputs(requestedLogLevel LogLevel, requestedLogFormat LogFormat) (LoggerOutputs, error) {
	zapLogLevel, levelExists := logLevelMapping[requestedLogLevel]
	if !levelExists {
		return LoggerOutputs{}, fmt.Errorf(unsupportedLogLevelTemplateConstant, requestedLogLevel)
	}

	if _, formatExists := logFormatEncodingMapping[requestedLogFormat]; !formatExists {
		return LoggerOutputs{}, fmt.Errorf(unsupportedLogFormatTemplateConstant, requestedLogFormat)
	}

	diagnosticLogger, diagnosticError := factory.buildDiagnosticLogger(zapLogLevel, requestedLogFormat)
	if diagnosticError != nil {
		return LoggerOutputs{}, diagnosticError
	}

	consoleLogger := zap.NewNop()
	if requestedLogFormat == LogFormatConsole {
		var consoleError error
		consoleLogger, consoleError = factory.buildConsoleLogger(zapLogLevel)
		if consoleError != nil {
			_ = diagnosticLogger.Sync()
			return LoggerOutputs{}, consoleError
		}
	}

	return LoggerOutputs{DiagnosticLogger: diagnosticLogger, ConsoleLogger: consoleLogger}, nil
}

func (factory *LoggerFactory) buildDiagnosticLogger(zapLogLevel zapcore.Level, requestedLogFormat LogFormat) (*zap.Logger, error) {
	configuration := zap.NewProductionConfig()
	configuration.Level = zap.NewAtomicLevelAt(zapLogLevel)
	configuration.DisableStacktrace = true

	switch requestedLogFormat {
	case LogFormatConsole:
		configuration.Encoding = consoleZapEncodingStringConstant
		configuration.EncoderConfig.TimeKey = timeFieldNameConstant
		configuration.EncoderConfig.LevelKey = levelFieldNameConstant
		configuration.EncoderConfig.MessageKey = consoleMessageFieldNameConstant
		configuration.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(humanReadableTimeLayoutConstant)
		configuration.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		configuration.EncoderConfig.CallerKey = emptyStringConstant
		configuration.EncoderConfig.StacktraceKey = emptyStringConstant
		configuration.EncoderConfig.NameKey = emptyStringConstant
		configuration.DisableCaller = true
	default:
		configuration.Encoding = jsonZapEncodingStringConstant
		configuration.EncoderConfig.TimeKey = timeFieldNameConstant
		configuration.EncoderConfig.LevelKey = levelFieldNameConstant
		configuration.EncoderConfig.MessageKey = structuredMessageFieldNameConstant
		configuration.EncoderConfig.NameKey = nameFieldNameConstant
		configuration.EncoderConfig.CallerKey = callerFieldNameConstant
		configuration.EncoderConfig.StacktraceKey = stacktraceFieldNameConstant
	}

	return configuration.Build()
}

func (factory *LoggerFactory) buildConsoleLogger(zapLogLevel zapcore.Level) (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:    consoleMessageFieldNameConstant,
		LevelKey:      levelFieldNameConstant,
		TimeKey:       emptyStringConstant,
		NameKey:       emptyStringConstant,
		CallerKey:     emptyStringConstant,
		StacktraceKey: emptyStringConstant,
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.Lock(os.Stderr),
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level >= zapLogLevel
		}),
	)

	return zap.New(core), nil
}
