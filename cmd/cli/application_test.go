package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/cmd/cli"
)

const (
	testConfigurationFileNameConstant          = "config.yaml"
	testConfigurationHeaderConstant            = "common:\n  log_level: info\n  log_format: structured\noperations:\n"
	testOperationBlockTemplateConstant         = "  - operation: %s\n    with:\n%s"
	testOperationRootsTemplateConstant         = "      roots:\n        - %s\n"
	testOperationRootDirectoryConstant         = "/tmp/config-root"
	testConfigurationSearchPathEnvironmentName = "GITSCRIPTS_CONFIG_SEARCH_PATH"
	testPackagesCommandNameConstant            = "repo-packages-purge"
	testBranchMigrateCommandNameConstant       = "branch-migrate"
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
			name: "MissingOperationConfiguration",
			operationNames: []string{
				"audit",
				"repo-packages-purge",
				"repo-prs-purge",
				"repo-folders-rename",
				"repo-remote-update",
				"repo-protocol-convert",
				"workflow",
			},
			expectedErrorSample:   &cli.MissingOperationConfigurationError{},
			expectedOperationName: "branch-migrate",
			commandUse:            testBranchMigrateCommandNameConstant,
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
