package cd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/execshell"
)

type stubGitExecutor struct {
	recorded  []execshell.CommandDetails
	responses []stubGitResponse
}

type stubGitResponse struct {
	result execshell.ExecutionResult
	err    error
}

func (executor *stubGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.recorded = append(executor.recorded, details)
	if len(executor.responses) == 0 {
		return execshell.ExecutionResult{}, nil
	}

	next := executor.responses[0]
	executor.responses = executor.responses[1:]
	if next.err != nil {
		return execshell.ExecutionResult{}, next.err
	}
	return next.result, nil
}

func (executor *stubGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func TestChangeExecutesExpectedCommands(t *testing.T) {
	executor := &stubGitExecutor{}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	result, changeError := service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "feature", RemoteName: "origin"})
	require.NoError(t, changeError)
	require.Equal(t, Result{RepositoryPath: "/tmp/repo", BranchName: "feature", BranchCreated: false}, result)
	require.Len(t, executor.recorded, 3)

	require.Equal(t, []string{"fetch", "--prune", "origin"}, executor.recorded[0].Arguments)
	require.Equal(t, []string{"switch", "feature"}, executor.recorded[1].Arguments)
	require.Equal(t, []string{"pull", "--rebase"}, executor.recorded[2].Arguments)
}

func TestChangeCreatesBranchWhenMissing(t *testing.T) {
	executor := &stubGitExecutor{responses: []stubGitResponse{{}, {err: errors.New("missing")}}}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	result, changeError := service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "feature", RemoteName: "upstream", CreateIfMissing: true})
	require.NoError(t, changeError)
	require.True(t, result.BranchCreated)

	require.Len(t, executor.recorded, 4)
	require.Equal(t, []string{"switch", "-c", "feature", "--track", "upstream/feature"}, executor.recorded[2].Arguments)
}

func TestChangeValidatesInputs(t *testing.T) {
	service, err := NewService(ServiceDependencies{GitExecutor: &stubGitExecutor{}})
	require.NoError(t, err)

	_, changeError := service.Change(context.Background(), Options{BranchName: "main"})
	require.Error(t, changeError)

	_, changeError = service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo"})
	require.Error(t, changeError)
}

func TestChangeSurfaceGitErrors(t *testing.T) {
	executor := &stubGitExecutor{responses: []stubGitResponse{
		{result: execshell.ExecutionResult{StandardOutput: "origin\n"}},
		{err: errors.New("fetch failed")},
	}}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	_, changeError := service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main"})
	require.ErrorContains(t, changeError, "fetch updates")
}

func TestChangeFetchesAllWhenDefaultRemoteMissing(t *testing.T) {
	executor := &stubGitExecutor{responses: []stubGitResponse{
		{result: execshell.ExecutionResult{StandardOutput: "upstream\n"}},
	}}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	result, changeError := service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "feature"})
	require.NoError(t, changeError)
	require.False(t, result.BranchCreated)

	require.Len(t, executor.recorded, 4)
	require.Equal(t, []string{"remote"}, executor.recorded[0].Arguments)
	require.Equal(t, []string{"fetch", "--all", "--prune"}, executor.recorded[1].Arguments)
}

func TestChangeFailsWhenRemoteEnumerationFails(t *testing.T) {
	executor := &stubGitExecutor{responses: []stubGitResponse{{err: errors.New("remote list failed")}}}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	_, changeError := service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main"})
	require.ErrorContains(t, changeError, "fetch updates")
}
