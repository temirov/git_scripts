package githubcli_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
)

const (
	testRepositoryIdentifierConstant                = "owner/example"
	testBaseBranchConstant                          = "main"
	testPullRequestTitleConstant                    = "Example"
	testPullRequestHeadConstant                     = "feature/example"
	testPagesSourceBranchConstant                   = "gh-pages"
	testPagesSourcePathConstant                     = "/docs"
	testResolveSuccessCaseNameConstant              = "resolve_success"
	testResolveDecodeFailureCaseNameConstant        = "resolve_decode_failure"
	testResolveCommandFailureCaseNameConstant       = "resolve_command_failure"
	testResolveInputFailureCaseNameConstant         = "resolve_input_failure"
	testListSuccessCaseNameConstant                 = "list_success"
	testListDecodeFailureCaseNameConstant           = "list_decode_failure"
	testListCommandFailureCaseNameConstant          = "list_command_failure"
	testListRepositoryValidationCaseNameConstant    = "list_repository_validation"
	testListBaseValidationCaseNameConstant          = "list_base_validation"
	testListStateValidationCaseNameConstant         = "list_state_validation"
	testPagesSuccessCaseNameConstant                = "pages_success"
	testPagesCommandFailureCaseNameConstant         = "pages_command_failure"
	testPagesRepositoryValidationCaseNameConstant   = "pages_repository_validation"
	testPagesSourceBranchValidationCaseNameConstant = "pages_source_branch_validation"
)

type stubGitHubExecutor struct {
	executeFunc     func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error)
	recordedDetails []execshell.CommandDetails
}

func (executor *stubGitHubExecutor) ExecuteGitHubCLI(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.recordedDetails = append(executor.recordedDetails, details)
	if executor.executeFunc != nil {
		return executor.executeFunc(executionContext, details)
	}
	return execshell.ExecutionResult{}, nil
}

func TestNewClientValidation(testInstance *testing.T) {
	testInstance.Run("nil_executor", func(testInstance *testing.T) {
		client, creationError := githubcli.NewClient(nil)
		require.Error(testInstance, creationError)
		require.ErrorIs(testInstance, creationError, githubcli.ErrExecutorNotConfigured)
		require.Nil(testInstance, client)
	})
}

func TestResolveRepoMetadata(testInstance *testing.T) {
	testCases := []struct {
		name        string
		repository  string
		executor    *stubGitHubExecutor
		expectError bool
		errorType   any
		verify      func(testInstance *testing.T, metadata githubcli.RepositoryMetadata, executor *stubGitHubExecutor)
	}{
		{
			name:       testResolveSuccessCaseNameConstant,
			repository: testRepositoryIdentifierConstant,
			executor: &stubGitHubExecutor{
				executeFunc: func(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
					return execshell.ExecutionResult{StandardOutput: `{"nameWithOwner":"owner/example","description":"Example repo","defaultBranchRef":{"name":"main"}}`}, nil
				},
			},
			verify: func(testInstance *testing.T, metadata githubcli.RepositoryMetadata, executor *stubGitHubExecutor) {
				require.Equal(testInstance, "owner/example", metadata.NameWithOwner)
				require.Equal(testInstance, "Example repo", metadata.Description)
				require.Equal(testInstance, "main", metadata.DefaultBranch)
				require.Len(testInstance, executor.recordedDetails, 1)
				require.Contains(testInstance, executor.recordedDetails[0].Arguments, testRepositoryIdentifierConstant)
			},
		},
		{
			name:       testResolveDecodeFailureCaseNameConstant,
			repository: testRepositoryIdentifierConstant,
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{StandardOutput: "not-json"}, nil
			}},
			expectError: true,
			errorType:   githubcli.ResponseDecodingError{},
		},
		{
			name:       testResolveCommandFailureCaseNameConstant,
			repository: testRepositoryIdentifierConstant,
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{}, execshell.CommandFailedError{Command: execshell.ShellCommand{Name: execshell.CommandGitHub}, Result: execshell.ExecutionResult{ExitCode: 1}}
			}},
			expectError: true,
			errorType:   githubcli.OperationError{},
		},
		{
			name:        testResolveInputFailureCaseNameConstant,
			repository:  "  ",
			executor:    &stubGitHubExecutor{},
			expectError: true,
			errorType:   githubcli.InvalidInputError{},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			client, creationError := githubcli.NewClient(testCase.executor)
			require.NoError(testInstance, creationError)

			metadata, resolutionError := client.ResolveRepoMetadata(context.Background(), testCase.repository)
			if testCase.expectError {
				require.Error(testInstance, resolutionError)
				require.IsType(testInstance, testCase.errorType, resolutionError)
			} else {
				require.NoError(testInstance, resolutionError)
				require.NotNil(testInstance, testCase.verify)
				testCase.verify(testInstance, metadata, testCase.executor)
			}
		})
	}
}

func TestListPullRequests(testInstance *testing.T) {
	testCases := []struct {
		name        string
		repository  string
		options     githubcli.PullRequestListOptions
		executor    *stubGitHubExecutor
		expectError bool
		errorType   any
		verify      func(testInstance *testing.T, pullRequests []githubcli.PullRequest, executor *stubGitHubExecutor)
	}{
		{
			name:       testListSuccessCaseNameConstant,
			repository: testRepositoryIdentifierConstant,
			options: githubcli.PullRequestListOptions{
				State:       githubcli.PullRequestStateOpen,
				BaseBranch:  testBaseBranchConstant,
				ResultLimit: 50,
			},
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{StandardOutput: `[{"number":42,"title":"Example","headRefName":"feature/example"}]`}, nil
			}},
			verify: func(testInstance *testing.T, pullRequests []githubcli.PullRequest, executor *stubGitHubExecutor) {
				require.Len(testInstance, pullRequests, 1)
				require.Equal(testInstance, 42, pullRequests[0].Number)
				require.Equal(testInstance, testPullRequestTitleConstant, pullRequests[0].Title)
				require.Equal(testInstance, testPullRequestHeadConstant, pullRequests[0].HeadRefName)
				require.Len(testInstance, executor.recordedDetails, 1)
				require.Contains(testInstance, executor.recordedDetails[0].Arguments, testRepositoryIdentifierConstant)
			},
		},
		{
			name:       testListDecodeFailureCaseNameConstant,
			repository: testRepositoryIdentifierConstant,
			options:    githubcli.PullRequestListOptions{State: githubcli.PullRequestStateOpen, BaseBranch: testBaseBranchConstant},
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{StandardOutput: "not-json"}, nil
			}},
			expectError: true,
			errorType:   githubcli.ResponseDecodingError{},
		},
		{
			name:       testListCommandFailureCaseNameConstant,
			repository: testRepositoryIdentifierConstant,
			options:    githubcli.PullRequestListOptions{State: githubcli.PullRequestStateClosed, BaseBranch: testBaseBranchConstant},
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{}, execshell.CommandExecutionError{Command: execshell.ShellCommand{Name: execshell.CommandGitHub}, Cause: errors.New("failed")}
			}},
			expectError: true,
			errorType:   githubcli.OperationError{},
		},
		{
			name:        testListRepositoryValidationCaseNameConstant,
			repository:  "",
			options:     githubcli.PullRequestListOptions{State: githubcli.PullRequestStateOpen, BaseBranch: testBaseBranchConstant},
			executor:    &stubGitHubExecutor{},
			expectError: true,
			errorType:   githubcli.InvalidInputError{},
		},
		{
			name:        testListBaseValidationCaseNameConstant,
			repository:  testRepositoryIdentifierConstant,
			options:     githubcli.PullRequestListOptions{State: githubcli.PullRequestStateOpen, BaseBranch: " "},
			executor:    &stubGitHubExecutor{},
			expectError: true,
			errorType:   githubcli.InvalidInputError{},
		},
		{
			name:        testListStateValidationCaseNameConstant,
			repository:  testRepositoryIdentifierConstant,
			options:     githubcli.PullRequestListOptions{BaseBranch: testBaseBranchConstant},
			executor:    &stubGitHubExecutor{},
			expectError: true,
			errorType:   githubcli.InvalidInputError{},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			client, creationError := githubcli.NewClient(testCase.executor)
			require.NoError(testInstance, creationError)

			pullRequests, listError := client.ListPullRequests(context.Background(), testCase.repository, testCase.options)
			if testCase.expectError {
				require.Error(testInstance, listError)
				require.IsType(testInstance, testCase.errorType, listError)
			} else {
				require.NoError(testInstance, listError)
				require.NotNil(testInstance, testCase.verify)
				testCase.verify(testInstance, pullRequests, testCase.executor)
			}
		})
	}
}

func TestUpdatePagesConfig(testInstance *testing.T) {
	testCases := []struct {
		name          string
		repository    string
		configuration githubcli.PagesConfiguration
		executor      *stubGitHubExecutor
		expectError   bool
		errorType     any
		verify        func(testInstance *testing.T, executor *stubGitHubExecutor)
	}{
		{
			name:          testPagesSuccessCaseNameConstant,
			repository:    testRepositoryIdentifierConstant,
			configuration: githubcli.PagesConfiguration{SourceBranch: testPagesSourceBranchConstant, SourcePath: testPagesSourcePathConstant},
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{}, nil
			}},
			verify: func(testInstance *testing.T, executor *stubGitHubExecutor) {
				require.Len(testInstance, executor.recordedDetails, 1)
				require.Equal(testInstance, fmt.Sprintf("repos/%s/pages", testRepositoryIdentifierConstant), executor.recordedDetails[0].Arguments[1])
				require.NotEmpty(testInstance, executor.recordedDetails[0].StandardInput)
			},
		},
		{
			name:          testPagesCommandFailureCaseNameConstant,
			repository:    testRepositoryIdentifierConstant,
			configuration: githubcli.PagesConfiguration{SourceBranch: testPagesSourceBranchConstant},
			executor: &stubGitHubExecutor{executeFunc: func(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
				return execshell.ExecutionResult{}, execshell.CommandExecutionError{Command: execshell.ShellCommand{Name: execshell.CommandGitHub}, Cause: errors.New("failed")}
			}},
			expectError: true,
			errorType:   githubcli.OperationError{},
		},
		{
			name:          testPagesRepositoryValidationCaseNameConstant,
			repository:    " ",
			configuration: githubcli.PagesConfiguration{SourceBranch: testPagesSourceBranchConstant},
			executor:      &stubGitHubExecutor{},
			expectError:   true,
			errorType:     githubcli.InvalidInputError{},
		},
		{
			name:          testPagesSourceBranchValidationCaseNameConstant,
			repository:    testRepositoryIdentifierConstant,
			configuration: githubcli.PagesConfiguration{SourceBranch: " "},
			executor:      &stubGitHubExecutor{},
			expectError:   true,
			errorType:     githubcli.InvalidInputError{},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			client, creationError := githubcli.NewClient(testCase.executor)
			require.NoError(testInstance, creationError)

			executionError := client.UpdatePagesConfig(context.Background(), testCase.repository, testCase.configuration)
			if testCase.expectError {
				require.Error(testInstance, executionError)
				require.IsType(testInstance, testCase.errorType, executionError)
			} else {
				require.NoError(testInstance, executionError)
				require.NotNil(testInstance, testCase.verify)
				testCase.verify(testInstance, testCase.executor)
			}
		})
	}
}
