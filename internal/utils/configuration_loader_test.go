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
	testCaseSearchPathXDGDirectoryMessageConstant     = "searches xdg configuration directory"
	testCaseSearchPathHomeDirectoryMessageConstant    = "searches home configuration directory"
	testCaseSearchPathWorkingPreferredMessageConstant = "prefers working directory when all directories contain configuration"
	testCaseSearchPathXDGPreferredMessageConstant     = "prefers xdg configuration directory when working directory lacks configuration"
	testCaseSearchPathWorkingOverHomeMessageConstant  = "prefers working directory over home configuration directory"
)

type configurationDirectoryRole string

const (
	configurationDirectoryRoleWorking configurationDirectoryRole = "working"
	configurationDirectoryRoleXDG     configurationDirectoryRole = "xdg"
	configurationDirectoryRoleHome    configurationDirectoryRole = "home"
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
		name                               string
		directoriesWithConfiguration       []configurationDirectoryRole
		expectedConfigurationDirectoryRole configurationDirectoryRole
	}{
		{
			name:                               testCaseSearchPathWorkingDirectoryMessageConstant,
			directoriesWithConfiguration:       []configurationDirectoryRole{configurationDirectoryRoleWorking},
			expectedConfigurationDirectoryRole: configurationDirectoryRoleWorking,
		},
		{
			name:                               testCaseSearchPathXDGDirectoryMessageConstant,
			directoriesWithConfiguration:       []configurationDirectoryRole{configurationDirectoryRoleXDG},
			expectedConfigurationDirectoryRole: configurationDirectoryRoleXDG,
		},
		{
			name:                               testCaseSearchPathHomeDirectoryMessageConstant,
			directoriesWithConfiguration:       []configurationDirectoryRole{configurationDirectoryRoleHome},
			expectedConfigurationDirectoryRole: configurationDirectoryRoleHome,
		},
		{
			name:                               testCaseSearchPathWorkingPreferredMessageConstant,
			directoriesWithConfiguration:       []configurationDirectoryRole{configurationDirectoryRoleWorking, configurationDirectoryRoleXDG, configurationDirectoryRoleHome},
			expectedConfigurationDirectoryRole: configurationDirectoryRoleWorking,
		},
		{
			name:                               testCaseSearchPathXDGPreferredMessageConstant,
			directoriesWithConfiguration:       []configurationDirectoryRole{configurationDirectoryRoleXDG, configurationDirectoryRoleHome},
			expectedConfigurationDirectoryRole: configurationDirectoryRoleXDG,
		},
		{
			name:                               testCaseSearchPathWorkingOverHomeMessageConstant,
			directoriesWithConfiguration:       []configurationDirectoryRole{configurationDirectoryRoleWorking, configurationDirectoryRoleHome},
			expectedConfigurationDirectoryRole: configurationDirectoryRoleWorking,
		},
	}

	logLevelByDirectoryRole := map[configurationDirectoryRole]string{
		configurationDirectoryRoleWorking: testConfiguredLogLevelConstant,
		configurationDirectoryRoleXDG:     testFileLogLevelConstant,
		configurationDirectoryRoleHome:    testOverriddenLogLevelConstant,
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(configurationLoaderSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			workingDirectoryPath := testInstance.TempDir()
			homeDirectoryPath := testInstance.TempDir()
			xdgConfigHomeDirectoryPath := filepath.Join(homeDirectoryPath, testXDGConfigHomeDirectoryNameConstant)

			testInstance.Setenv("HOME", homeDirectoryPath)
			testInstance.Setenv("XDG_CONFIG_HOME", xdgConfigHomeDirectoryPath)

			xdgConfigurationDirectoryPath := filepath.Join(xdgConfigHomeDirectoryPath, testUserConfigurationDirectoryNameConstant)
			homeConfigurationDirectoryPath := filepath.Join(homeDirectoryPath, testUserConfigurationDirectoryNameConstant)

			require.NoError(testInstance, os.MkdirAll(workingDirectoryPath, 0o755))
			require.NoError(testInstance, os.MkdirAll(xdgConfigurationDirectoryPath, 0o755))
			require.NoError(testInstance, os.MkdirAll(homeConfigurationDirectoryPath, 0o755))

			directoryPathByRole := map[configurationDirectoryRole]string{
				configurationDirectoryRoleWorking: workingDirectoryPath,
				configurationDirectoryRoleXDG:     xdgConfigurationDirectoryPath,
				configurationDirectoryRoleHome:    homeConfigurationDirectoryPath,
			}

			for _, directoryRole := range testCase.directoriesWithConfiguration {
				configurationDirectoryPath := directoryPathByRole[directoryRole]
				configurationFilePath := filepath.Join(configurationDirectoryPath, testConfigFileNameConstant)
				configurationContent := fmt.Sprintf(testConfigContentTemplateConstant, logLevelByDirectoryRole[directoryRole])
				writeConfigurationError := os.WriteFile(configurationFilePath, []byte(configurationContent), 0o600)
				require.NoError(testInstance, writeConfigurationError)
			}

			configurationLoader := utils.NewConfigurationLoader(
				testConfigurationNameConstant,
				testConfigurationTypeConstant,
				testEnvironmentPrefixConstant,
				[]string{workingDirectoryPath, xdgConfigurationDirectoryPath, homeConfigurationDirectoryPath},
			)

			defaultValues := map[string]any{
				testLogLevelKeyConstant: testDefaultLogLevelConstant,
			}

			loadedConfiguration := configurationFixture{}
			metadata, loadError := configurationLoader.LoadConfiguration("", defaultValues, &loadedConfiguration)
			require.NoError(testInstance, loadError)

			expectedLogLevel := logLevelByDirectoryRole[testCase.expectedConfigurationDirectoryRole]
			require.Equal(testInstance, expectedLogLevel, loadedConfiguration.Common.LogLevel)

			expectedConfigurationPath := filepath.Join(directoryPathByRole[testCase.expectedConfigurationDirectoryRole], testConfigFileNameConstant)
			require.Equal(testInstance, expectedConfigurationPath, metadata.ConfigFileUsed)
		})
	}
}
