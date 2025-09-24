package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	integrationInfoMessageConstant                     = "\"msg\":\"git_scripts CLI executed\""
	integrationDebugMessageConstant                    = "\"msg\":\"git_scripts CLI diagnostics\""
	integrationLogLevelEnvKeyConstant                  = "GITSCRIPTS_LOG_LEVEL"
	integrationConfigFileNameConstant                  = "config.yaml"
	integrationConfigTemplateConstant                  = "log_level: %s\n"
	integrationDefaultCaseNameConstant                 = "default_info"
	integrationConfigCaseNameConstant                  = "config_debug"
	integrationEnvironmentCaseNameConstant             = "environment_error"
	integrationDebugLevelConstant                      = "debug"
	integrationErrorLevelConstant                      = "error"
	integrationCommandTimeout                          = 5 * time.Second
	integrationConfigFlagTemplateConstant              = "--config=%s"
	integrationEnvironmentAssignmentTemplateConstant   = "%s=%s"
	integrationSubtestNameTemplateConstant             = "%d_%s"
	integrationHelpUsagePrefixConstant                 = "Usage:"
	integrationHelpDescriptionSnippetConstant          = "git_scripts ships reusable helpers that integrate Git, GitHub CLI, and related tooling."
	integrationHelpCaseNameConstant                    = "help_output"
	integrationStructuredLogCaseNameConstant           = "structured_default"
	integrationConsoleLogCaseNameConstant              = "console_format"
	integrationConsoleLogFlagConstant                  = "--log-format=console"
	integrationConfigurationInitializedSnippetConstant = "configuration initialized"
)

func TestCLIIntegrationLogLevels(testInstance *testing.T) {
	testCases := []struct {
		name                 string
		configurationLevel   string
		environmentLevel     string
		expectedInfoVisible  bool
		expectedDebugVisible bool
	}{
		{
			name:                 integrationDefaultCaseNameConstant,
			configurationLevel:   "",
			environmentLevel:     "",
			expectedInfoVisible:  true,
			expectedDebugVisible: false,
		},
		{
			name:                 integrationConfigCaseNameConstant,
			configurationLevel:   integrationDebugLevelConstant,
			environmentLevel:     "",
			expectedInfoVisible:  true,
			expectedDebugVisible: true,
		},
		{
			name:                 integrationEnvironmentCaseNameConstant,
			configurationLevel:   "",
			environmentLevel:     integrationErrorLevelConstant,
			expectedInfoVisible:  false,
			expectedDebugVisible: false,
		},
	}

	currentWorkingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRootDirectory := filepath.Dir(currentWorkingDirectory)

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(integrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			arguments := []string{"run", "."}
			environment := os.Environ()
			tempDirectory := testInstance.TempDir()

			if len(testCase.configurationLevel) > 0 {
				configurationPath := filepath.Join(tempDirectory, integrationConfigFileNameConstant)
				configurationContent := fmt.Sprintf(integrationConfigTemplateConstant, testCase.configurationLevel)
				writeError := os.WriteFile(configurationPath, []byte(configurationContent), 0o600)
				require.NoError(testInstance, writeError)
				arguments = append(arguments, fmt.Sprintf(integrationConfigFlagTemplateConstant, configurationPath))
			}

			if len(testCase.environmentLevel) > 0 {
				environment = append(environment, fmt.Sprintf(integrationEnvironmentAssignmentTemplateConstant, integrationLogLevelEnvKeyConstant, testCase.environmentLevel))
			}

			executionContext, cancelFunction := context.WithTimeout(context.Background(), integrationCommandTimeout)
			defer cancelFunction()

			command := exec.CommandContext(executionContext, "go", arguments...)
			command.Dir = repositoryRootDirectory
			command.Env = environment

			outputBytes, runError := command.CombinedOutput()
			outputText := string(outputBytes)
			require.NoError(testInstance, runError, outputText)

			if testCase.expectedInfoVisible {
				require.Contains(testInstance, outputText, integrationInfoMessageConstant)
			} else {
				require.NotContains(testInstance, outputText, integrationInfoMessageConstant)
			}

			if testCase.expectedDebugVisible {
				require.Contains(testInstance, outputText, integrationDebugMessageConstant)
			} else {
				require.NotContains(testInstance, outputText, integrationDebugMessageConstant)
			}
		})
	}
}

func TestCLIIntegrationDisplaysHelpWhenNoArgumentsProvided(testInstance *testing.T) {
	testCases := []struct {
		name             string
		expectedSnippets []string
	}{
		{
			name: integrationHelpCaseNameConstant,
			expectedSnippets: []string{
				integrationHelpUsagePrefixConstant,
				integrationHelpDescriptionSnippetConstant,
			},
		},
	}

	currentWorkingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRootDirectory := filepath.Dir(currentWorkingDirectory)

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(integrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			commandArguments := []string{"run", "."}
			executionContext, cancelFunction := context.WithTimeout(context.Background(), integrationCommandTimeout)
			defer cancelFunction()

			command := exec.CommandContext(executionContext, "go", commandArguments...)
			command.Dir = repositoryRootDirectory
			command.Env = os.Environ()

			outputBytes, runError := command.CombinedOutput()
			outputText := string(outputBytes)
			require.NoError(testInstance, runError, outputText)

			for _, expectedSnippet := range testCase.expectedSnippets {
				require.Contains(testInstance, outputText, expectedSnippet)
			}
		})
	}
}

func TestCLIIntegrationRespectsLogFormatFlag(testInstance *testing.T) {
	testCases := []struct {
		name                string
		additionalArguments []string
		expectStructured    bool
	}{
		{
			name:                integrationStructuredLogCaseNameConstant,
			additionalArguments: []string{},
			expectStructured:    true,
		},
		{
			name:                integrationConsoleLogCaseNameConstant,
			additionalArguments: []string{integrationConsoleLogFlagConstant},
			expectStructured:    false,
		},
	}

	currentWorkingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRootDirectory := filepath.Dir(currentWorkingDirectory)

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(integrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(testInstance *testing.T) {
			commandArguments := []string{"run", "."}
			commandArguments = append(commandArguments, testCase.additionalArguments...)

			executionContext, cancelFunction := context.WithTimeout(context.Background(), integrationCommandTimeout)
			defer cancelFunction()

			command := exec.CommandContext(executionContext, "go", commandArguments...)
			command.Dir = repositoryRootDirectory
			command.Env = os.Environ()

			commandOutput, runError := command.CombinedOutput()
			outputText := string(commandOutput)
			require.NoError(testInstance, runError, outputText)

			logLineFound := false
			outputLines := strings.Split(outputText, "\n")
			for _, outputLine := range outputLines {
				trimmedLine := strings.TrimSpace(outputLine)
				if len(trimmedLine) == 0 {
					continue
				}

				if !strings.Contains(trimmedLine, integrationConfigurationInitializedSnippetConstant) {
					continue
				}

				logLineFound = true
				isStructuredLog := json.Valid([]byte(trimmedLine))
				if testCase.expectStructured {
					require.True(testInstance, isStructuredLog, trimmedLine)
				} else {
					require.False(testInstance, isStructuredLog, trimmedLine)
				}

				break
			}

			require.True(testInstance, logLineFound, outputText)
		})
	}
}
