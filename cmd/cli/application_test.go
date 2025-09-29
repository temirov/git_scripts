package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/cmd/cli"
	repos "github.com/temirov/gix/cmd/cli/repos"
	workflowcmd "github.com/temirov/gix/cmd/cli/workflow"
	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/branches"
	"github.com/temirov/gix/internal/migrate"
	"github.com/temirov/gix/internal/packages"
	"github.com/temirov/gix/internal/utils"
)

const (
	testConfigurationFileNameConstant               = "config.yaml"
	testConfigurationHeaderConstant                 = "common:\n  log_level: info\n  log_format: structured\noperations:\n"
	testConsoleConfigurationHeaderConstant          = "common:\n  log_level: info\n  log_format: console\noperations:\n"
	testOperationBlockTemplateConstant              = "  - operation: %s\n    with:\n%s"
	testOperationRootsTemplateConstant              = "      roots:\n        - %s\n"
	testOperationRootDirectoryConstant              = "/tmp/config-root"
	testConfigurationSearchPathEnvironmentName      = "GIX_CONFIG_SEARCH_PATH"
	testPackagesCommandNameConstant                 = "repo-packages-purge"
	testBranchMigrateCommandNameConstant            = "branch-migrate"
	testBranchCleanupCommandNameConstant            = "repo-prs-purge"
	testReposRemotesCommandNameConstant             = "repo-remote-update"
	testReposProtocolCommandNameConstant            = "repo-protocol-convert"
	testReposRenameCommandNameConstant              = "repo-folders-rename"
	testAuditCommandNameConstant                    = "audit"
	testWorkflowCommandNameConstant                 = "workflow"
	embeddedDefaultsBranchCleanupTestNameConstant   = "BranchCleanupDefaults"
	embeddedDefaultsPackagesTestNameConstant        = "PackagesDefaults"
	embeddedDefaultsReposRemotesTestNameConstant    = "ReposRemotesDefaults"
	embeddedDefaultsReposProtocolTestNameConstant   = "ReposProtocolDefaults"
	embeddedDefaultsReposRenameTestNameConstant     = "ReposRenameDefaults"
	embeddedDefaultsWorkflowTestNameConstant        = "WorkflowDefaults"
	embeddedDefaultsBranchMigrateTestNameConstant   = "BranchMigrateDefaults"
	embeddedDefaultsAuditTestNameConstant           = "AuditDefaults"
	embeddedDefaultRootPathConstant                 = "."
	embeddedDefaultRemoteNameConstant               = "origin"
	embeddedDefaultPullRequestLimitConstant         = 100
	configurationInitializedMessageTextConstant     = "configuration initialized"
	configurationInitializedConsoleTemplateConstant = "%s | log level=%s | log format=%s | config file=%s"
	configurationLogLevelFieldNameConstant          = "log_level"
	configurationLogFormatFieldNameConstant         = "log_format"
	configurationFileFieldNameConstant              = "config_file"
)

var requiredOperationNames = []string{
	"audit",
	"repo-packages-purge",
	"repo-prs-purge",
	"repo-folders-rename",
	"repo-remote-update",
	"repo-protocol-convert",
	"workflow",
	"branch-migrate",
}

func TestApplicationInitializeConfiguration(t *testing.T) {
	testCases := []struct {
		name                  string
		operationNames        []string
		expectedErrorSample   error
		expectedOperationName string
		commandUse            string
	}{
		{
			name:           "ValidConfiguration",
			operationNames: requiredOperationNames,
			commandUse:     testPackagesCommandNameConstant,
		},
		{
			name: "DuplicateOperationConfiguration",
			operationNames: append([]string{
				"audit",
				"Audit",
			}, requiredOperationNames[1:]...),
			expectedErrorSample:   &cli.DuplicateOperationConfigurationError{},
			expectedOperationName: "audit",
			commandUse:            testPackagesCommandNameConstant,
		},
		{
			name: "CommandConfigurationMissingForTargetCommandIgnored",
			operationNames: []string{
				"audit",
				"repo-packages-purge",
				"repo-prs-purge",
				"repo-folders-rename",
				"repo-remote-update",
				"repo-protocol-convert",
				"workflow",
			},
			commandUse: testBranchMigrateCommandNameConstant,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			temporaryDirectory := t.TempDir()
			configurationContent := buildConfigurationContent(testCase.operationNames)
			configurationPath := filepath.Join(temporaryDirectory, testConfigurationFileNameConstant)

			writeConfigurationFile(t, configurationPath, configurationContent)

			t.Setenv(testConfigurationSearchPathEnvironmentName, temporaryDirectory)

			application := cli.NewApplication()

			executionError := application.InitializeForCommand(testCase.commandUse)

			if testCase.expectedErrorSample == nil {
				require.NoError(t, executionError)
				return
			}

			require.Error(t, executionError)

			switch testCase.expectedErrorSample.(type) {
			case *cli.DuplicateOperationConfigurationError:
				var duplicateError cli.DuplicateOperationConfigurationError
				require.ErrorAs(t, executionError, &duplicateError)
				require.Equal(t, testCase.expectedOperationName, duplicateError.OperationName)
			case *cli.MissingOperationConfigurationError:
				var missingError cli.MissingOperationConfigurationError
				require.ErrorAs(t, executionError, &missingError)
				require.Equal(t, testCase.expectedOperationName, missingError.OperationName)
			default:
				t.Fatalf("unexpected error sample type %T", testCase.expectedErrorSample)
			}
		})
	}
}

func TestApplicationInitializationLoggingModes(testInstance *testing.T) {
	testCases := []struct {
		name                string
		configurationHeader string
		assertion           func(*testing.T, string, string)
	}{
		{
			name:                "StructuredLogging",
			configurationHeader: testConfigurationHeaderConstant,
			assertion: func(t *testing.T, capturedOutput string, configurationPath string) {
				t.Helper()

				trimmedOutput := strings.TrimSpace(capturedOutput)
				require.NotEmpty(t, trimmedOutput)

				logLines := strings.Split(trimmedOutput, "\n")
				require.Len(t, logLines, 1)

				var logEntry map[string]any
				require.NoError(t, json.Unmarshal([]byte(logLines[0]), &logEntry))

				messageValue, messageValueExists := logEntry["msg"].(string)
				require.True(t, messageValueExists)
				require.Equal(t, configurationInitializedMessageTextConstant, messageValue)

				logLevelValue, logLevelExists := logEntry[configurationLogLevelFieldNameConstant].(string)
				require.True(t, logLevelExists)
				require.Equal(t, string(utils.LogLevelInfo), logLevelValue)

				logFormatValue, logFormatExists := logEntry[configurationLogFormatFieldNameConstant].(string)
				require.True(t, logFormatExists)
				require.Equal(t, string(utils.LogFormatStructured), logFormatValue)

				configurationFileValue, configurationFileExists := logEntry[configurationFileFieldNameConstant].(string)
				require.True(t, configurationFileExists)
				require.Equal(t, configurationPath, configurationFileValue)
			},
		},
		{
			name:                "ConsoleLogging",
			configurationHeader: testConsoleConfigurationHeaderConstant,
			assertion: func(t *testing.T, capturedOutput string, configurationPath string) {
				t.Helper()

				trimmedOutput := strings.TrimSpace(capturedOutput)
				require.NotEmpty(t, trimmedOutput)

				expectedBanner := fmt.Sprintf(
					configurationInitializedConsoleTemplateConstant,
					configurationInitializedMessageTextConstant,
					string(utils.LogLevelInfo),
					string(utils.LogFormatConsole),
					configurationPath,
				)

				require.Contains(t, trimmedOutput, expectedBanner)
				require.NotContains(t, trimmedOutput, "\""+configurationLogLevelFieldNameConstant+"\"")
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(t *testing.T) {
			configurationDirectory := t.TempDir()
			configurationContent := buildConfigurationContentWithHeader(testCase.configurationHeader, requiredOperationNames)
			configurationPath := filepath.Join(configurationDirectory, testConfigurationFileNameConstant)

			writeConfigurationFile(t, configurationPath, configurationContent)

			t.Setenv(testConfigurationSearchPathEnvironmentName, configurationDirectory)

			application := cli.NewApplication()

			stderrCapture := startTestStderrCapture(t)
			initializationError := application.InitializeForCommand(testPackagesCommandNameConstant)
			capturedOutput := stderrCapture.Stop(t)

			require.NoError(t, initializationError)

			testCase.assertion(t, capturedOutput, configurationPath)
		})
	}
}

func TestApplicationEmbeddedDefaultsProvideCommandConfigurations(testInstance *testing.T) {
	operationIndex := buildEmbeddedOperationIndex(testInstance)

	testCases := []struct {
		name          string
		commandUse    string
		operationName string
		assertion     func(testing.TB, map[string]any)
	}{
		{
			name:          embeddedDefaultsBranchCleanupTestNameConstant,
			commandUse:    testBranchCleanupCommandNameConstant,
			operationName: testBranchCleanupCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration branches.CommandConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)
				sanitized := configuration.Sanitize()

				assertions := require.New(assertionTarget)
				assertions.Equal(embeddedDefaultRemoteNameConstant, sanitized.RemoteName)
				assertions.Equal(embeddedDefaultPullRequestLimitConstant, sanitized.PullRequestLimit)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, sanitized.RepositoryRoots)
			},
		},
		{
			name:          embeddedDefaultsPackagesTestNameConstant,
			commandUse:    testPackagesCommandNameConstant,
			operationName: testPackagesCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration packages.PurgeConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)
				sanitized := configuration.Sanitize()

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, sanitized.RepositoryRoots)
			},
		},
		{
			name:          embeddedDefaultsReposRemotesTestNameConstant,
			commandUse:    testReposRemotesCommandNameConstant,
			operationName: testReposRemotesCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration repos.RemotesConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, configuration.RepositoryRoots)
			},
		},
		{
			name:          embeddedDefaultsReposProtocolTestNameConstant,
			commandUse:    testReposProtocolCommandNameConstant,
			operationName: testReposProtocolCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration repos.ProtocolConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, configuration.RepositoryRoots)
				assertions.Empty(strings.TrimSpace(configuration.FromProtocol))
				assertions.Empty(strings.TrimSpace(configuration.ToProtocol))
			},
		},
		{
			name:          embeddedDefaultsReposRenameTestNameConstant,
			commandUse:    testReposRenameCommandNameConstant,
			operationName: testReposRenameCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration repos.RenameConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, configuration.RepositoryRoots)
			},
		},
		{
			name:          embeddedDefaultsWorkflowTestNameConstant,
			commandUse:    testWorkflowCommandNameConstant,
			operationName: testWorkflowCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration workflowcmd.CommandConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)
				sanitized := configuration.Sanitize()

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, sanitized.Roots)
			},
		},
		{
			name:          embeddedDefaultsBranchMigrateTestNameConstant,
			commandUse:    testBranchMigrateCommandNameConstant,
			operationName: testBranchMigrateCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration migrate.CommandConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)
				sanitized := configuration.Sanitize()

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, sanitized.RepositoryRoots)
			},
		},
		{
			name:          embeddedDefaultsAuditTestNameConstant,
			commandUse:    testAuditCommandNameConstant,
			operationName: testAuditCommandNameConstant,
			assertion: func(assertionTarget testing.TB, options map[string]any) {
				assertionTarget.Helper()

				var configuration audit.CommandConfiguration
				decodeOperationOptions(assertionTarget, options, &configuration)
				sanitized := configuration.Sanitize()

				assertions := require.New(assertionTarget)
				assertions.Equal([]string{embeddedDefaultRootPathConstant}, sanitized.Roots)
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(t *testing.T) {
			t.Setenv(testConfigurationSearchPathEnvironmentName, t.TempDir())

			application := cli.NewApplication()
			initializationError := application.InitializeForCommand(testCase.commandUse)
			require.NoError(t, initializationError)

			normalizedOperationName := strings.ToLower(strings.TrimSpace(testCase.operationName))
			operationOptions, exists := operationIndex[normalizedOperationName]
			require.True(t, exists)

			testCase.assertion(t, operationOptions)
		})
	}
}

func buildConfigurationContent(operationNames []string) string {
	return buildConfigurationContentWithHeader(testConfigurationHeaderConstant, operationNames)
}

func buildConfigurationContentWithHeader(commonHeader string, operationNames []string) string {
	configurationBuilder := strings.Builder{}
	configurationBuilder.WriteString(commonHeader)

	for _, operationName := range operationNames {
		rootsBlock := fmt.Sprintf(testOperationRootsTemplateConstant, testOperationRootDirectoryConstant)
		operationBlock := fmt.Sprintf(testOperationBlockTemplateConstant, operationName, rootsBlock)
		configurationBuilder.WriteString(operationBlock)
	}

	return configurationBuilder.String()
}

func writeConfigurationFile(t *testing.T, configurationPath string, configurationContent string) {
	t.Helper()

	writeError := os.WriteFile(configurationPath, []byte(configurationContent), 0o600)
	require.NoError(t, writeError)
}

func buildEmbeddedOperationIndex(testingInstance testing.TB) map[string]map[string]any {
	testingInstance.Helper()

	configuration := decodeEmbeddedApplicationConfiguration(testingInstance)
	operationIndex := make(map[string]map[string]any)

	for _, operation := range configuration.Operations {
		normalizedName := strings.ToLower(strings.TrimSpace(operation.Name))
		if len(normalizedName) == 0 {
			continue
		}

		duplicatedOptions := make(map[string]any, len(operation.Options))
		for optionKey, optionValue := range operation.Options {
			duplicatedOptions[optionKey] = optionValue
		}

		operationIndex[normalizedName] = duplicatedOptions
	}

	return operationIndex
}

func decodeEmbeddedApplicationConfiguration(testingInstance testing.TB) cli.ApplicationConfiguration {
	testingInstance.Helper()

	configurationData, configurationType := cli.EmbeddedDefaultConfiguration()
	viperInstance := viper.New()
	viperInstance.SetConfigType(configurationType)

	readError := viperInstance.ReadConfig(bytes.NewReader(configurationData))
	require.NoError(testingInstance, readError)

	var configuration cli.ApplicationConfiguration
	unmarshalError := viperInstance.Unmarshal(&configuration)
	require.NoError(testingInstance, unmarshalError)

	return configuration
}

func decodeOperationOptions(testingInstance testing.TB, options map[string]any, target any) {
	testingInstance.Helper()

	decoder, decoderError := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "mapstructure", Result: target})
	require.NoError(testingInstance, decoderError)

	decodeError := decoder.Decode(options)
	require.NoError(testingInstance, decodeError)
}

type testStderrCapture struct {
	originalDescriptor *os.File
	reader             *os.File
	writer             *os.File
}

func startTestStderrCapture(testingInstance testing.TB) testStderrCapture {
	testingInstance.Helper()

	reader, writer, pipeError := os.Pipe()
	require.NoError(testingInstance, pipeError)

	capture := testStderrCapture{
		originalDescriptor: os.Stderr,
		reader:             reader,
		writer:             writer,
	}

	os.Stderr = writer

	return capture
}

func (capture *testStderrCapture) Stop(testingInstance testing.TB) string {
	testingInstance.Helper()

	os.Stderr = capture.originalDescriptor

	require.NoError(testingInstance, capture.writer.Close())

	capturedBytes, readError := io.ReadAll(capture.reader)
	require.NoError(testingInstance, readError)

	require.NoError(testingInstance, capture.reader.Close())

	return string(capturedBytes)
}
