package utils_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/internal/utils"
)

const (
	testLoggerFactoryCaseSupportedFormatConstant   = "supported_log_level_%s_format_%s"
	testLoggerFactoryCaseUnsupportedLevelConstant  = "unsupported_log_level"
	testLoggerFactoryCaseUnsupportedFormatConstant = "unsupported_log_format"
	testLoggerFactorySubtestTemplateConstant       = "%d_%s"
	testInvalidLogLevelConstant                    = "invalid"
	testInvalidLogFormatConstant                   = "invalid"
	testLogMessageConstant                         = "logger_factory_test_message"
)

func TestLoggerFactoryCreateLogger(testInstance *testing.T) {
	testCases := []struct {
		name                string
		requestedLogLevel   utils.LogLevel
		requestedLogFormat  utils.LogFormat
		expectError         bool
		expectStructuredLog bool
	}{
		{
			name:                fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelDebug, utils.LogFormatStructured),
			requestedLogLevel:   utils.LogLevelDebug,
			requestedLogFormat:  utils.LogFormatStructured,
			expectError:         false,
			expectStructuredLog: true,
		},
		{
			name:                fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelInfo, utils.LogFormatStructured),
			requestedLogLevel:   utils.LogLevelInfo,
			requestedLogFormat:  utils.LogFormatStructured,
			expectError:         false,
			expectStructuredLog: true,
		},
		{
			name:                fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelInfo, utils.LogFormatConsole),
			requestedLogLevel:   utils.LogLevelInfo,
			requestedLogFormat:  utils.LogFormatConsole,
			expectError:         false,
			expectStructuredLog: false,
		},
		{
			name:               testLoggerFactoryCaseUnsupportedLevelConstant,
			requestedLogLevel:  utils.LogLevel(testInvalidLogLevelConstant),
			requestedLogFormat: utils.LogFormatStructured,
			expectError:        true,
		},
		{
			name:               testLoggerFactoryCaseUnsupportedFormatConstant,
			requestedLogLevel:  utils.LogLevelInfo,
			requestedLogFormat: utils.LogFormat(testInvalidLogFormatConstant),
			expectError:        true,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(testLoggerFactorySubtestTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			loggerFactory := utils.NewLoggerFactory()

			pipeReader, pipeWriter, pipeError := os.Pipe()
			require.NoError(testInstance, pipeError)

			originalStderr := os.Stderr
			os.Stderr = pipeWriter

			logger, creationError := loggerFactory.CreateLogger(testCase.requestedLogLevel, testCase.requestedLogFormat)

			os.Stderr = originalStderr

			if testCase.expectError {
				require.Error(testInstance, creationError)
				require.Nil(testInstance, logger)

				require.NoError(testInstance, pipeWriter.Close())
				require.NoError(testInstance, pipeReader.Close())
				return
			}

			require.NoError(testInstance, creationError)
			require.NotNil(testInstance, logger)

			logger.Info(testLogMessageConstant)
			syncError := logger.Sync()
			if syncError != nil {
				require.True(testInstance, errors.Is(syncError, syscall.ENOTSUP) || errors.Is(syncError, syscall.EINVAL))
			}

			require.NoError(testInstance, pipeWriter.Close())

			capturedOutput, readError := io.ReadAll(pipeReader)
			require.NoError(testInstance, readError)
			require.NoError(testInstance, pipeReader.Close())

			trimmedOutput := bytes.TrimSpace(capturedOutput)
			require.NotEmpty(testInstance, trimmedOutput)
			require.Contains(testInstance, string(trimmedOutput), testLogMessageConstant)

			isJSONLog := json.Valid(trimmedOutput)
			if testCase.expectStructuredLog {
				require.True(testInstance, isJSONLog)
			} else {
				require.False(testInstance, isJSONLog)
			}
		})
	}
}
