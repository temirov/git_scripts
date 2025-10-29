package migrate

import (
	"context"
	"errors"
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
	pagesError       error
	defaultBranchSet bool
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
	return nil, nil
}

func (operations *recordingGitHubOperations) UpdatePullRequestBase(context.Context, string, int, string) error {
	return nil
}

func (operations *recordingGitHubOperations) SetDefaultBranch(context.Context, string, string) error {
	operations.defaultBranchSet = true
	return nil
}

func (operations *recordingGitHubOperations) CheckBranchProtection(context.Context, string, string) (bool, error) {
	return false, nil
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
}
