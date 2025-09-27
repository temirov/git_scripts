package workflow_test

import (
	"fmt"
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
