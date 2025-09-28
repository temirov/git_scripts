package utils_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/utils"
)

const (
	testEnvironmentPrefixConstant                  = "TESTGITSCRIPTS"
	testCommonSectionKeyConstant                   = "common"
	testLogLevelKeyConstant                        = testCommonSectionKeyConstant + ".log_level"
	testDefaultLogLevelConstant                    = "info"
	testConfiguredLogLevelConstant                 = "debug"
	testOverriddenLogLevelConstant                 = "error"
	testFileLogLevelConstant                       = "warn"
	testConfigFileNameConstant                     = "config.yaml"
	testConfigContentTemplateConstant              = "common:\n  log_level: %s\n"
	testCaseDefaultsMessageConstant                = "defaults are applied"
	testCaseFileMessageConstant                    = "config file overrides defaults"
	testCaseEnvironmentMessageConstant             = "environment overrides file"
	testConfigurationNameConstant                  = "config"
	testConfigurationTypeConstant                  = "yaml"
	configurationLoaderSubtestNameTemplateConstant = "%d_%s"
)

type configurationFixture struct {
	Common configurationCommonFixture `mapstructure:"common"`
}

type configurationCommonFixture struct {
	LogLevel string `mapstructure:"log_level"`
}

func TestConfigurationLoaderLoadConfiguration(testInstance *testing.T) {
	testCases := []struct {
		name                string
		fileLogLevel        string
		environmentLogLevel string
		expectedLogLevel    string
	}{
		{
			name:                testCaseDefaultsMessageConstant,
			fileLogLevel:        "",
			environmentLogLevel: "",
			expectedLogLevel:    testDefaultLogLevelConstant,
		},
		{
			name:                testCaseFileMessageConstant,
			fileLogLevel:        testConfiguredLogLevelConstant,
			environmentLogLevel: "",
			expectedLogLevel:    testConfiguredLogLevelConstant,
		},
		{
			name:                testCaseEnvironmentMessageConstant,
			fileLogLevel:        testFileLogLevelConstant,
			environmentLogLevel: testOverriddenLogLevelConstant,
			expectedLogLevel:    testOverriddenLogLevelConstant,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(configurationLoaderSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			tempDirectory := testInstance.TempDir()
			configurationFilePath := ""
			if len(testCase.fileLogLevel) > 0 {
				configurationFilePath = filepath.Join(tempDirectory, testConfigFileNameConstant)
				configurationContent := fmt.Sprintf(testConfigContentTemplateConstant, testCase.fileLogLevel)
				writeError := os.WriteFile(configurationFilePath, []byte(configurationContent), 0o600)
				require.NoError(testInstance, writeError)
			}

			if len(testCase.environmentLogLevel) > 0 {
				environmentVariableName := fmt.Sprintf("%s_%s", testEnvironmentPrefixConstant, strings.ToUpper(strings.ReplaceAll(testLogLevelKeyConstant, ".", "_")))
				setError := os.Setenv(environmentVariableName, testCase.environmentLogLevel)
				require.NoError(testInstance, setError)
				testInstance.Cleanup(func() {
					unsetError := os.Unsetenv(environmentVariableName)
					require.NoError(testInstance, unsetError)
				})
			}

			configurationLoader := utils.NewConfigurationLoader(testConfigurationNameConstant, testConfigurationTypeConstant, testEnvironmentPrefixConstant, []string{tempDirectory})

			defaultValues := map[string]any{
				testLogLevelKeyConstant: testDefaultLogLevelConstant,
			}

			loadedConfiguration := configurationFixture{}
			metadata, loadError := configurationLoader.LoadConfiguration(configurationFilePath, defaultValues, &loadedConfiguration)
			require.NoError(testInstance, loadError)
			require.Equal(testInstance, testCase.expectedLogLevel, loadedConfiguration.Common.LogLevel)

			if len(configurationFilePath) > 0 {
				require.Equal(testInstance, configurationFilePath, metadata.ConfigFileUsed)
			} else {
				require.Empty(testInstance, metadata.ConfigFileUsed)
			}
		})
	}
}
