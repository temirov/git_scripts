package migrate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
)

type stubCommandExecutor struct{}

func (stubCommandExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func (stubCommandExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type stubGitCommandExecutor struct{}

func (stubGitCommandExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type recordingGitHubOperations struct {
	pagesError        error
	listError         error
	retargetErrors    map[int]error
	protectionError   error
	defaultBranchSet  bool
	pullRequests      []githubcli.PullRequest
	retargetedNumbers []int
}

func (operations *recordingGitHubOperations) ResolveRepoMetadata(context.Context, string) (githubcli.RepositoryMetadata, error) {
	return githubcli.RepositoryMetadata{}, nil
}

func (operations *recordingGitHubOperations) GetPagesConfig(context.Context, string) (githubcli.PagesStatus, error) {
	if operations.pagesError != nil {
		return githubcli.PagesStatus{}, operations.pagesError
	}
	return githubcli.PagesStatus{}, nil
}

func (operations *recordingGitHubOperations) UpdatePagesConfig(context.Context, string, githubcli.PagesConfiguration) error {
	return nil
}

func (operations *recordingGitHubOperations) ListPullRequests(context.Context, string, githubcli.PullRequestListOptions) ([]githubcli.PullRequest, error) {
	if operations.listError != nil {
		return nil, operations.listError
	}
	return append([]githubcli.PullRequest(nil), operations.pullRequests...), nil
}

func (operations *recordingGitHubOperations) UpdatePullRequestBase(_ context.Context, _ string, pullRequestNumber int, _ string) error {
	operations.retargetedNumbers = append(operations.retargetedNumbers, pullRequestNumber)
	if operations.retargetErrors != nil {
		if err, exists := operations.retargetErrors[pullRequestNumber]; exists {
			return err
		}
	}
	return nil
}

func (operations *recordingGitHubOperations) SetDefaultBranch(context.Context, string, string) error {
	operations.defaultBranchSet = true
	return nil
}

func (operations *recordingGitHubOperations) CheckBranchProtection(context.Context, string, string) (bool, error) {
	if operations.protectionError != nil {
		return false, operations.protectionError
	}
	return false, nil
}

func makeCommandFailedError(message string) error {
	return execshell.CommandFailedError{
		Command: execshell.ShellCommand{Name: execshell.CommandGit},
		Result: execshell.ExecutionResult{
			ExitCode:      128,
			StandardError: message,
		},
	}
}

func TestServiceExecuteContinuesWhenPagesLookupFails(testInstance *testing.T) {
	testInstance.Parallel()

	repositoryExecutor := stubGitCommandExecutor{}
	repositoryManager, managerError := gitrepo.NewRepositoryManager(repositoryExecutor)
	require.NoError(testInstance, managerError)

	pagesLookupError := githubcli.OperationError{
		Operation: githubcli.OperationName("GetPagesConfig"),
		Cause:     errors.New("gh command exited with code 1"),
	}

	githubOperations := &recordingGitHubOperations{pagesError: pagesLookupError}

	service, serviceError := NewService(ServiceDependencies{
		Logger:            zap.NewNop(),
		RepositoryManager: repositoryManager,
		GitHubClient:      githubOperations,
		GitExecutor:       stubCommandExecutor{},
	})
	require.NoError(testInstance, serviceError)

	options := MigrationOptions{
		RepositoryPath:       testInstance.TempDir(),
		RepositoryRemoteName: "origin",
		RepositoryIdentifier: "owner/example",
		WorkflowsDirectory:   ".github/workflows",
		SourceBranch:         BranchMain,
		TargetBranch:         BranchMaster,
		PushUpdates:          false,
		DeleteSourceBranch:   false,
	}

	result, executionError := service.Execute(context.Background(), options)
	require.NoError(testInstance, executionError)
	require.False(testInstance, result.PagesConfigurationUpdated)
	require.True(testInstance, result.DefaultBranchUpdated)
	require.True(testInstance, githubOperations.defaultBranchSet)
	require.Len(testInstance, result.Warnings, 1)
	require.Contains(testInstance, result.Warnings[0], "PAGES-SKIP")
}

func TestServiceExecuteWarnsWhenRetargetFails(testInstance *testing.T) {
	testInstance.Parallel()

	repositoryExecutor := stubGitCommandExecutor{}
	repositoryManager, managerError := gitrepo.NewRepositoryManager(repositoryExecutor)
	require.NoError(testInstance, managerError)

	retargetError := makeCommandFailedError("fatal: cannot update PR")

	githubOperations := &recordingGitHubOperations{
		pullRequests:   []githubcli.PullRequest{{Number: 42}},
		retargetErrors: map[int]error{42: retargetError},
	}

	service, serviceError := NewService(ServiceDependencies{
		Logger:            zap.NewNop(),
		RepositoryManager: repositoryManager,
		GitHubClient:      githubOperations,
		GitExecutor:       stubCommandExecutor{},
	})
	require.NoError(testInstance, serviceError)

	options := MigrationOptions{
		RepositoryPath:       testInstance.TempDir(),
		RepositoryRemoteName: "origin",
		RepositoryIdentifier: "owner/example",
		WorkflowsDirectory:   ".github/workflows",
		SourceBranch:         BranchMain,
		TargetBranch:         BranchMaster,
		PushUpdates:          false,
		DeleteSourceBranch:   false,
	}

	result, executionError := service.Execute(context.Background(), options)
	require.NoError(testInstance, executionError)
	require.Contains(testInstance, strings.Join(result.Warnings, " "), "PR-RETARGET-SKIP")
}

func TestServiceExecuteWarnsWhenBranchProtectionFails(testInstance *testing.T) {
	testInstance.Parallel()

	repositoryExecutor := stubGitCommandExecutor{}
	repositoryManager, managerError := gitrepo.NewRepositoryManager(repositoryExecutor)
	require.NoError(testInstance, managerError)

	githubOperations := &recordingGitHubOperations{
		protectionError: makeCommandFailedError("fatal: protection read failed"),
	}

	service, serviceError := NewService(ServiceDependencies{
		Logger:            zap.NewNop(),
		RepositoryManager: repositoryManager,
		GitHubClient:      githubOperations,
		GitExecutor:       stubCommandExecutor{},
	})
	require.NoError(testInstance, serviceError)

	options := MigrationOptions{
		RepositoryPath:       testInstance.TempDir(),
		RepositoryRemoteName: "origin",
		RepositoryIdentifier: "owner/example",
		WorkflowsDirectory:   ".github/workflows",
		SourceBranch:         BranchMain,
		TargetBranch:         BranchMaster,
		PushUpdates:          false,
		DeleteSourceBranch:   false,
	}

	result, executionError := service.Execute(context.Background(), options)
	require.NoError(testInstance, executionError)
	require.Contains(testInstance, strings.Join(result.Warnings, " "), "PROTECTION-SKIP")
	require.False(testInstance, result.SafetyStatus.SafeToDelete)
}
