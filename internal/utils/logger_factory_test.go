package utils_test

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/internal/utils"
)

const (
	testLoggerFactoryCaseSupportedFormatConstant = "supported_log_level_%s"
	testLoggerFactoryCaseUnsupportedConstant     = "unsupported_log_level"
	testLoggerFactorySubtestTemplateConstant     = "%d_%s"
	testInvalidLogLevelConstant                  = "invalid"
)

func TestLoggerFactoryCreateLogger(testInstance *testing.T) {
	testCases := []struct {
		name              string
		requestedLogLevel utils.LogLevel
		expectError       bool
	}{
		{
			name:              fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelDebug),
			requestedLogLevel: utils.LogLevelDebug,
			expectError:       false,
		},
		{
			name:              fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelInfo),
			requestedLogLevel: utils.LogLevelInfo,
			expectError:       false,
		},
		{
			name:              fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelWarn),
			requestedLogLevel: utils.LogLevelWarn,
			expectError:       false,
		},
		{
			name:              fmt.Sprintf(testLoggerFactoryCaseSupportedFormatConstant, utils.LogLevelError),
			requestedLogLevel: utils.LogLevelError,
			expectError:       false,
		},
		{
			name:              testLoggerFactoryCaseUnsupportedConstant,
			requestedLogLevel: utils.LogLevel(testInvalidLogLevelConstant),
			expectError:       true,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(testLoggerFactorySubtestTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			loggerFactory := utils.NewLoggerFactory()
			logger, creationError := loggerFactory.CreateLogger(testCase.requestedLogLevel)
			if testCase.expectError {
				require.Error(testInstance, creationError)
				require.Nil(testInstance, logger)
				return
			}

			require.NoError(testInstance, creationError)
			require.NotNil(testInstance, logger)

			syncError := logger.Sync()
			if syncError != nil {
				require.True(testInstance, errors.Is(syncError, syscall.ENOTSUP) || errors.Is(syncError, syscall.EINVAL))
			}
		})
	}
}
