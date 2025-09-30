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
	testEnvironmentPrefixConstant                     = "TESTGIX"
	testCommonSectionKeyConstant                      = "common"
	testLogLevelKeyConstant                           = testCommonSectionKeyConstant + ".log_level"
	testDefaultLogLevelConstant                       = "info"
	testConfiguredLogLevelConstant                    = "debug"
	testOverriddenLogLevelConstant                    = "error"
	testFileLogLevelConstant                          = "warn"
	testConfigFileNameConstant                        = "config.yaml"
	testConfigContentTemplateConstant                 = "common:\n  log_level: %s\n"
	testCaseEmbeddedMessageConstant                   = "embedded configuration merges"
	testCaseDefaultsMessageConstant                   = "defaults are applied"
	testCaseFileMessageConstant                       = "config file overrides defaults"
	testCaseEnvironmentMessageConstant                = "environment overrides file"
	testConfigurationNameConstant                     = "config"
	testConfigurationTypeConstant                     = "yaml"
	configurationLoaderSubtestNameTemplateConstant    = "%d_%s"
	testEmbeddedLogLevelConstant                      = "debug"
	testUserConfigurationDirectoryNameConstant        = ".gix"
	testXDGConfigHomeDirectoryNameConstant            = "config"
	testCaseSearchPathWorkingDirectoryMessageConstant = "searches working directory"
	testCaseSearchPathHomeDirectoryMessageConstant    = "searches home configuration directory"
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
		embeddedLogLevel    string
		fileLogLevel        string
		environmentLogLevel string
		expectedLogLevel    string
	}{
		{
			name:                testCaseEmbeddedMessageConstant,
			embeddedLogLevel:    testEmbeddedLogLevelConstant,
			fileLogLevel:        "",
			environmentLogLevel: "",
			expectedLogLevel:    testEmbeddedLogLevelConstant,
		},
		{
			name:                testCaseDefaultsMessageConstant,
			embeddedLogLevel:    testDefaultLogLevelConstant,
			fileLogLevel:        "",
			environmentLogLevel: "",
			expectedLogLevel:    testDefaultLogLevelConstant,
		},
		{
			name:                testCaseFileMessageConstant,
			embeddedLogLevel:    testDefaultLogLevelConstant,
			fileLogLevel:        testConfiguredLogLevelConstant,
			environmentLogLevel: "",
			expectedLogLevel:    testConfiguredLogLevelConstant,
		},
		{
			name:                testCaseEnvironmentMessageConstant,
			embeddedLogLevel:    testDefaultLogLevelConstant,
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

			configurationLoader.SetEmbeddedConfiguration([]byte(fmt.Sprintf(testConfigContentTemplateConstant, testCase.embeddedLogLevel)), testConfigurationTypeConstant)

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

func TestConfigurationLoaderSearchPaths(testInstance *testing.T) {
	testCases := []struct {
		name                         string
		configurationDirectorySelect func(workingDirectoryPath string, userConfigurationDirectoryPath string) string
	}{
		{
			name: testCaseSearchPathWorkingDirectoryMessageConstant,
			configurationDirectorySelect: func(workingDirectoryPath string, userConfigurationDirectoryPath string) string {
				return workingDirectoryPath
			},
		},
		{
			name: testCaseSearchPathHomeDirectoryMessageConstant,
			configurationDirectorySelect: func(workingDirectoryPath string, userConfigurationDirectoryPath string) string {
				return userConfigurationDirectoryPath
			},
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(configurationLoaderSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			workingDirectoryPath := testInstance.TempDir()
			homeDirectoryPath := testInstance.TempDir()
			xdgConfigHomeDirectoryPath := filepath.Join(homeDirectoryPath, testXDGConfigHomeDirectoryNameConstant)

			testInstance.Setenv("HOME", homeDirectoryPath)
			testInstance.Setenv("XDG_CONFIG_HOME", xdgConfigHomeDirectoryPath)

			userConfigurationBaseDirectoryPath, userConfigurationDirectoryError := os.UserConfigDir()
			require.NoError(testInstance, userConfigurationDirectoryError)
			require.NotEmpty(testInstance, userConfigurationBaseDirectoryPath)

			userConfigurationDirectoryPath := filepath.Join(userConfigurationBaseDirectoryPath, testUserConfigurationDirectoryNameConstant)
			createDirectoryError := os.MkdirAll(userConfigurationDirectoryPath, 0o755)
			require.NoError(testInstance, createDirectoryError)

			selectedConfigurationDirectoryPath := testCase.configurationDirectorySelect(workingDirectoryPath, userConfigurationDirectoryPath)
			ensureSelectedDirectoryError := os.MkdirAll(selectedConfigurationDirectoryPath, 0o755)
			require.NoError(testInstance, ensureSelectedDirectoryError)

			configurationFilePath := filepath.Join(selectedConfigurationDirectoryPath, testConfigFileNameConstant)
			configurationContent := fmt.Sprintf(testConfigContentTemplateConstant, testConfiguredLogLevelConstant)
			writeConfigurationError := os.WriteFile(configurationFilePath, []byte(configurationContent), 0o600)
			require.NoError(testInstance, writeConfigurationError)

			configurationLoader := utils.NewConfigurationLoader(
				testConfigurationNameConstant,
				testConfigurationTypeConstant,
				testEnvironmentPrefixConstant,
				[]string{workingDirectoryPath, userConfigurationDirectoryPath},
			)

			defaultValues := map[string]any{
				testLogLevelKeyConstant: testDefaultLogLevelConstant,
			}

			loadedConfiguration := configurationFixture{}
			metadata, loadError := configurationLoader.LoadConfiguration("", defaultValues, &loadedConfiguration)
			require.NoError(testInstance, loadError)
			require.Equal(testInstance, testConfiguredLogLevelConstant, loadedConfiguration.Common.LogLevel)
			require.Equal(testInstance, configurationFilePath, metadata.ConfigFileUsed)
		})
	}
}
