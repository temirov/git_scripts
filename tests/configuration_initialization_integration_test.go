package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	configurationInitializationLocalCaseNameConstant        = "local_scope"
	configurationInitializationUserCaseNameConstant         = "user_scope"
	configurationInitializationOverwriteCaseNameConstant    = "overwrite_protection"
	configurationInitializationForceCaseNameConstant        = "force_overwrite"
	configurationInitializationLocalArgumentConstant        = "--init"
	configurationInitializationUserArgumentConstant         = "--init=user"
	configurationInitializationForceFlagConstant            = "--force"
	configurationInitializationHomeEnvNameConstant          = "HOME"
	configurationInitializationUserDirectoryNameConstant    = ".gix"
	configurationInitializationErrorMessageFragmentConstant = "already exists"
)

type configurationInitializationEnvironment struct {
	workingDirectory          string
	environmentOverrides      map[string]string
	expectedConfigurationPath string
}

func TestCLIConfigurationInitializationCreatesFiles(testInstance *testing.T) {
	currentWorkingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRootDirectory := filepath.Dir(currentWorkingDirectory)

	binaryPath := buildIntegrationBinary(testInstance, repositoryRootDirectory)

	testCases := []struct {
		name      string
		arguments []string
		prepare   func(*testing.T) configurationInitializationEnvironment
	}{
		{
			name:      configurationInitializationLocalCaseNameConstant,
			arguments: []string{configurationInitializationLocalArgumentConstant},
			prepare: func(t *testing.T) configurationInitializationEnvironment {
				workingDirectory := t.TempDir()
				expectedPath := filepath.Join(workingDirectory, integrationConfigFileNameConstant)
				return configurationInitializationEnvironment{
					workingDirectory:          workingDirectory,
					environmentOverrides:      map[string]string{},
					expectedConfigurationPath: expectedPath,
				}
			},
		},
		{
			name:      configurationInitializationUserCaseNameConstant,
			arguments: []string{configurationInitializationUserArgumentConstant},
			prepare: func(t *testing.T) configurationInitializationEnvironment {
				workingDirectory := t.TempDir()
				homeDirectory := t.TempDir()
				expectedPath := filepath.Join(homeDirectory, configurationInitializationUserDirectoryNameConstant, integrationConfigFileNameConstant)
				return configurationInitializationEnvironment{
					workingDirectory: workingDirectory,
					environmentOverrides: map[string]string{
						configurationInitializationHomeEnvNameConstant: homeDirectory,
					},
					expectedConfigurationPath: expectedPath,
				}
			},
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(integrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(t *testing.T) {
			environmentDetails := testCase.prepare(t)

			outputText, runError := runBinaryIntegrationCommand(
				t,
				binaryPath,
				environmentDetails.workingDirectory,
				environmentDetails.environmentOverrides,
				integrationCommandTimeout,
				testCase.arguments,
			)
			require.NoError(t, runError, outputText)

			fileContent, readError := os.ReadFile(environmentDetails.expectedConfigurationPath)
			require.NoError(t, readError)
			require.NotEmpty(t, fileContent)

			configurationDirectory := filepath.Dir(environmentDetails.expectedConfigurationPath)
			directoryInfo, statError := os.Stat(configurationDirectory)
			require.NoError(t, statError)
			require.True(t, directoryInfo.IsDir())
		})
	}
}

func TestCLIConfigurationInitializationOverwriteProtection(testInstance *testing.T) {
	currentWorkingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRootDirectory := filepath.Dir(currentWorkingDirectory)

	binaryPath := buildIntegrationBinary(testInstance, repositoryRootDirectory)

	testCases := []struct {
		name            string
		secondArguments []string
		expectError     bool
	}{
		{
			name:            configurationInitializationOverwriteCaseNameConstant,
			secondArguments: []string{configurationInitializationLocalArgumentConstant},
			expectError:     true,
		},
		{
			name: configurationInitializationForceCaseNameConstant,
			secondArguments: []string{
				configurationInitializationLocalArgumentConstant,
				configurationInitializationForceFlagConstant,
			},
			expectError: false,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(integrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(t *testing.T) {
			workingDirectory := t.TempDir()

			firstOutput, firstError := runBinaryIntegrationCommand(
				t,
				binaryPath,
				workingDirectory,
				map[string]string{},
				integrationCommandTimeout,
				[]string{configurationInitializationLocalArgumentConstant},
			)
			require.NoError(t, firstError, firstOutput)

			configurationPath := filepath.Join(workingDirectory, integrationConfigFileNameConstant)
			initialContent, readError := os.ReadFile(configurationPath)
			require.NoError(t, readError)
			require.NotEmpty(t, initialContent)

			secondOutput, secondError := runBinaryIntegrationCommand(
				t,
				binaryPath,
				workingDirectory,
				map[string]string{},
				integrationCommandTimeout,
				testCase.secondArguments,
			)

			if testCase.expectError {
				require.Error(t, secondError)
				require.Contains(t, secondOutput, configurationInitializationErrorMessageFragmentConstant)

				resultingContent, verifyError := os.ReadFile(configurationPath)
				require.NoError(t, verifyError)
				require.Equal(t, initialContent, resultingContent)
				return
			}

			require.NoError(t, secondError, secondOutput)

			resultingContent, verifyError := os.ReadFile(configurationPath)
			require.NoError(t, verifyError)
			require.NotEmpty(t, resultingContent)
		})
	}
}
