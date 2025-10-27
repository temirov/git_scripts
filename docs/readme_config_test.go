package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/temirov/gix/internal/workflow"
)

const (
	readmeFileNameConstant             = "README.md"
	yamlFenceStartConstant             = "```yaml"
	yamlFenceEndConstant               = "```"
	configHeaderMarkerConstant         = "# config.yaml"
	readmeSnippetTestNameConstant      = "readme_workflow_configuration"
	readmeSnippetTemporaryPattern      = "readme-config-*.yaml"
	expectedOperationCount             = 8
	parentDirectoryReferenceConstant   = ".."
	missingHeaderMessageConstant       = "README example missing config header marker"
	missingStartFenceMessageConstant   = "README example missing yaml fence start"
	missingEndFenceMessageConstant     = "README example missing yaml fence end"
	unexpectedOperationMessageTemplate = "unexpected operation %s"
	duplicateOperationMessageTemplate  = "duplicate operation %s"
	defaultTempDirectoryRootConstant   = ""
)

var expectedCommandOperations = map[string]struct{}{
	"audit":                 {},
	"repo-packages-purge":   {},
	"repo-prs-purge":        {},
	"repo-remote-update":    {},
	"repo-protocol-convert": {},
	"repo-folders-rename":   {},
	"workflow":              {},
	"branch-default":        {},
}

type readmeApplicationConfiguration struct {
	Operations []readmeOperationConfiguration `yaml:"operations"`
}

type readmeOperationConfiguration struct {
	Operation string         `yaml:"operation"`
	Options   map[string]any `yaml:"with"`
}

func TestReadmeWorkflowConfigurationParses(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)

	readmePath := filepath.Join(workingDirectory, parentDirectoryReferenceConstant, readmeFileNameConstant)
	contentBytes, readError := os.ReadFile(readmePath)
	require.NoError(testInstance, readError)

	contentText := string(contentBytes)
	headerIndex := strings.Index(contentText, configHeaderMarkerConstant)
	require.NotEqual(testInstance, -1, headerIndex, missingHeaderMessageConstant)

	fenceStartIndex := strings.LastIndex(contentText[:headerIndex], yamlFenceStartConstant)
	require.NotEqual(testInstance, -1, fenceStartIndex, missingStartFenceMessageConstant)

	remainingText := contentText[headerIndex:]
	fenceEndRelativeIndex := strings.Index(remainingText, yamlFenceEndConstant)
	require.NotEqual(testInstance, -1, fenceEndRelativeIndex, missingEndFenceMessageConstant)
	fenceEndIndex := headerIndex + fenceEndRelativeIndex

	snippetContent := strings.TrimSpace(contentText[fenceStartIndex+len(yamlFenceStartConstant) : fenceEndIndex])

	testCases := []struct {
		name          string
		configuration string
	}{
		{
			name:          readmeSnippetTestNameConstant,
			configuration: snippetContent,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			tempFile, tempFileError := os.CreateTemp(defaultTempDirectoryRootConstant, readmeSnippetTemporaryPattern)
			require.NoError(subtest, tempFileError)
			subtest.Cleanup(func() {
				require.NoError(subtest, os.Remove(tempFile.Name()))
			})

			_, writeError := tempFile.WriteString(testCase.configuration)
			require.NoError(subtest, writeError)
			require.NoError(subtest, tempFile.Close())

			_, workflowError := workflow.LoadConfiguration(tempFile.Name())
			require.NoError(subtest, workflowError)

			var applicationConfiguration readmeApplicationConfiguration
			unmarshalError := yaml.Unmarshal([]byte(testCase.configuration), &applicationConfiguration)
			require.NoError(subtest, unmarshalError)

			require.Len(subtest, applicationConfiguration.Operations, expectedOperationCount)

			seenOperations := make(map[string]struct{}, len(applicationConfiguration.Operations))
			for _, operationConfig := range applicationConfiguration.Operations {
				normalizedName := strings.TrimSpace(strings.ToLower(operationConfig.Operation))
				_, expected := expectedCommandOperations[normalizedName]
				require.Truef(subtest, expected, unexpectedOperationMessageTemplate, normalizedName)

				_, duplicate := seenOperations[normalizedName]
				require.Falsef(subtest, duplicate, duplicateOperationMessageTemplate, normalizedName)
				seenOperations[normalizedName] = struct{}{}
			}
		})
	}
}
