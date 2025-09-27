package workflow_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/internal/repos/shared"
	"github.com/temirov/git_scripts/internal/workflow"
)

const (
	testToolReferenceKeyConstant                 = "tool_ref"
	testToolNameConstant                         = "shared-protocol"
	testOptionFromKeyConstant                    = "from"
	testOptionToKeyConstant                      = "to"
	testMissingToolNameConstant                  = "missing-tool"
	testMismatchedOperationErrorTemplateConstant = "workflow step references tool %s expecting operation %s but step configured %s"
	testMissingOperationMessageConstant          = "workflow step missing operation name"
	testWorkflowConfigFileNameConstant           = "workflow.yaml"
	testSequenceWithToolReferenceCaseName        = "sequence with reusable tool reference"
	testSequenceWithInlineOperationCaseName      = "sequence with inline operation"
	emptyOperationTypeConstant                   = workflow.OperationType("")
	testWorkflowSequenceWithToolReferenceYAML    = `workflow_tools:
  - name: shared-protocol
    operation: convert-protocol
    with:
      from: https
      to: ssh
workflow:
  - with:
      tool_ref: shared-protocol
`
	testWorkflowSequenceWithoutToolReferenceYAML = `workflow:
  - operation: update-canonical-remote
`
)

func TestBuildOperationsToolReferenceResolution(testInstance *testing.T) {
	testCases := []struct {
		name                 string
		configuration        workflow.Configuration
		expectedFromProtocol shared.RemoteProtocol
		expectedToProtocol   shared.RemoteProtocol
	}{
		{
			name: "uses tool defaults when only reference provided",
			configuration: workflow.Configuration{
				Tools: []workflow.NamedToolConfiguration{
					{
						Name: testToolNameConstant,
						ToolConfiguration: workflow.ToolConfiguration{
							Operation: workflow.OperationTypeProtocolConversion,
							Options: map[string]any{
								testOptionFromKeyConstant: string(shared.RemoteProtocolHTTPS),
								testOptionToKeyConstant:   string(shared.RemoteProtocolSSH),
							},
						},
					},
				},
				Steps: []workflow.StepConfiguration{
					{
						Options: map[string]any{
							testToolReferenceKeyConstant: testToolNameConstant,
						},
					},
				},
			},
			expectedFromProtocol: shared.RemoteProtocolHTTPS,
			expectedToProtocol:   shared.RemoteProtocolSSH,
		},
		{
			name: "inline overrides replace tool defaults",
			configuration: workflow.Configuration{
				Tools: []workflow.NamedToolConfiguration{
					{
						Name: testToolNameConstant,
						ToolConfiguration: workflow.ToolConfiguration{
							Operation: workflow.OperationTypeProtocolConversion,
							Options: map[string]any{
								testOptionFromKeyConstant: string(shared.RemoteProtocolHTTPS),
								testOptionToKeyConstant:   string(shared.RemoteProtocolSSH),
							},
						},
					},
				},
				Steps: []workflow.StepConfiguration{
					{
						Options: map[string]any{
							testToolReferenceKeyConstant: testToolNameConstant,
							testOptionToKeyConstant:      string(shared.RemoteProtocolGit),
						},
					},
				},
			},
			expectedFromProtocol: shared.RemoteProtocolHTTPS,
			expectedToProtocol:   shared.RemoteProtocolGit,
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testingInstance *testing.T) {
			operations, buildError := workflow.BuildOperations(testCase.configuration)
			require.NoError(testingInstance, buildError)
			require.Len(testingInstance, operations, 1)

			protocolOperation, typeAssertionSucceeded := operations[0].(*workflow.ProtocolConversionOperation)
			require.True(testingInstance, typeAssertionSucceeded)
			require.Equal(testingInstance, testCase.expectedFromProtocol, protocolOperation.FromProtocol)
			require.Equal(testingInstance, testCase.expectedToProtocol, protocolOperation.ToProtocol)
		})
	}
}

func TestBuildOperationsMissingToolReference(testInstance *testing.T) {
	configuration := workflow.Configuration{
		Tools: []workflow.NamedToolConfiguration{},
		Steps: []workflow.StepConfiguration{
			{
				Options: map[string]any{
					testToolReferenceKeyConstant: testMissingToolNameConstant,
				},
			},
		},
	}

	_, buildError := workflow.BuildOperations(configuration)
	require.Error(testInstance, buildError)

	var notFoundError workflow.ToolReferenceNotFoundError
	require.ErrorAs(testInstance, buildError, &notFoundError)
	require.Equal(testInstance, testMissingToolNameConstant, notFoundError.ToolName)
}

func TestBuildOperationsToolReferenceOperationValidation(testInstance *testing.T) {
	testCases := []struct {
		name          string
		configuration workflow.Configuration
		expectedError string
	}{
		{
			name: "missing operation without tool reference",
			configuration: workflow.Configuration{
				Steps: []workflow.StepConfiguration{{}},
			},
			expectedError: testMissingOperationMessageConstant,
		},
		{
			name: "tool reference with mismatched operation",
			configuration: workflow.Configuration{
				Tools: []workflow.NamedToolConfiguration{
					{
						Name: testToolNameConstant,
						ToolConfiguration: workflow.ToolConfiguration{
							Operation: workflow.OperationTypeProtocolConversion,
						},
					},
				},
				Steps: []workflow.StepConfiguration{
					{
						Operation: workflow.OperationTypeRenameDirectories,
						Options: map[string]any{
							testToolReferenceKeyConstant: testToolNameConstant,
						},
					},
				},
			},
			expectedError: fmt.Sprintf(
				testMismatchedOperationErrorTemplateConstant,
				testToolNameConstant,
				workflow.OperationTypeProtocolConversion,
				workflow.OperationTypeRenameDirectories,
			),
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			_, buildError := workflow.BuildOperations(testCase.configuration)
			require.Error(subtest, buildError)
			require.ErrorContains(subtest, buildError, testCase.expectedError)
		})
	}
}

func TestLoadConfigurationWorkflowSequence(testInstance *testing.T) {
	testCases := []struct {
		name                 string
		workflowContents     string
		expectedOperations   []workflow.OperationType
		expectedOptionsSlice []map[string]any
		expectedToolCount    int
	}{
		{
			name:             testSequenceWithToolReferenceCaseName,
			workflowContents: testWorkflowSequenceWithToolReferenceYAML,
			expectedOperations: []workflow.OperationType{
				emptyOperationTypeConstant,
			},
			expectedOptionsSlice: []map[string]any{
				{
					testToolReferenceKeyConstant: testToolNameConstant,
				},
			},
			expectedToolCount: 1,
		},
		{
			name:             testSequenceWithInlineOperationCaseName,
			workflowContents: testWorkflowSequenceWithoutToolReferenceYAML,
			expectedOperations: []workflow.OperationType{
				workflow.OperationTypeCanonicalRemote,
			},
			expectedOptionsSlice: []map[string]any{nil},
			expectedToolCount:    0,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(testingInstance *testing.T) {
			tempDirectory := testingInstance.TempDir()
			configurationPath := filepath.Join(tempDirectory, testWorkflowConfigFileNameConstant)
			require.NoError(testingInstance, os.WriteFile(configurationPath, []byte(testCase.workflowContents), 0o644))

			configuration, loadError := workflow.LoadConfiguration(configurationPath)
			require.NoError(testingInstance, loadError)

			require.Len(testingInstance, configuration.Steps, len(testCase.expectedOperations))
			for stepIndex := range configuration.Steps {
				if testCase.expectedOperations[stepIndex] == emptyOperationTypeConstant {
					require.Empty(testingInstance, configuration.Steps[stepIndex].Operation)
				} else {
					require.Equal(testingInstance, testCase.expectedOperations[stepIndex], configuration.Steps[stepIndex].Operation)
				}
				require.Equal(testingInstance, testCase.expectedOptionsSlice[stepIndex], configuration.Steps[stepIndex].Options)
			}

			require.Len(testingInstance, configuration.Tools, testCase.expectedToolCount)
			if testCase.expectedToolCount > 0 {
				require.Equal(testingInstance, testToolNameConstant, configuration.Tools[0].Name)
				require.Equal(testingInstance, workflow.OperationTypeProtocolConversion, configuration.Tools[0].Operation)
				require.Equal(testingInstance, map[string]any{
					testOptionFromKeyConstant: string(shared.RemoteProtocolHTTPS),
					testOptionToKeyConstant:   string(shared.RemoteProtocolSSH),
				}, configuration.Tools[0].Options)
			}
		})
	}
}
