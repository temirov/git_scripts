package cli_test

import (
	"bytes"
	"fmt"
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
)

const (
	testConfigurationFileNameConstant             = "config.yaml"
	testConfigurationHeaderConstant               = "common:\n  log_level: info\n  log_format: structured\noperations:\n"
	testOperationBlockTemplateConstant            = "  - operation: %s\n    with:\n%s"
	testOperationRootsTemplateConstant            = "      roots:\n        - %s\n"
	testOperationRootDirectoryConstant            = "/tmp/config-root"
	testConfigurationSearchPathEnvironmentName    = "GIX_CONFIG_SEARCH_PATH"
	testPackagesCommandNameConstant               = "repo-packages-purge"
	testBranchMigrateCommandNameConstant          = "branch-migrate"
	testBranchCleanupCommandNameConstant          = "repo-prs-purge"
	testReposRemotesCommandNameConstant           = "repo-remote-update"
	testReposProtocolCommandNameConstant          = "repo-protocol-convert"
	testReposRenameCommandNameConstant            = "repo-folders-rename"
	testAuditCommandNameConstant                  = "audit"
	testWorkflowCommandNameConstant               = "workflow"
	embeddedDefaultsBranchCleanupTestNameConstant = "BranchCleanupDefaults"
	embeddedDefaultsPackagesTestNameConstant      = "PackagesDefaults"
	embeddedDefaultsReposRemotesTestNameConstant  = "ReposRemotesDefaults"
	embeddedDefaultsReposProtocolTestNameConstant = "ReposProtocolDefaults"
	embeddedDefaultsReposRenameTestNameConstant   = "ReposRenameDefaults"
	embeddedDefaultsWorkflowTestNameConstant      = "WorkflowDefaults"
	embeddedDefaultsBranchMigrateTestNameConstant = "BranchMigrateDefaults"
	embeddedDefaultsAuditTestNameConstant         = "AuditDefaults"
	embeddedDefaultRootPathConstant               = "."
	embeddedDefaultRemoteNameConstant             = "origin"
	embeddedDefaultPullRequestLimitConstant       = 100
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
	configurationBuilder := strings.Builder{}
	configurationBuilder.WriteString(testConfigurationHeaderConstant)

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
