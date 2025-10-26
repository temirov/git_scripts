package cd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/execshell"
)

type stubGitExecutor struct {
	recorded []execshell.CommandDetails
	errors   []error
}

func (executor *stubGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.recorded = append(executor.recorded, details)
	if len(executor.errors) == 0 {
		return execshell.ExecutionResult{}, nil
	}
	next := executor.errors[0]
	executor.errors = executor.errors[1:]
	if next != nil {
		return execshell.ExecutionResult{}, next
	}
	return execshell.ExecutionResult{}, nil
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

	require.Equal(t, []string{"fetch", "--all", "--prune"}, executor.recorded[0].Arguments)
	require.Equal(t, []string{"switch", "feature"}, executor.recorded[1].Arguments)
	require.Equal(t, []string{"pull", "--rebase"}, executor.recorded[2].Arguments)
}

func TestChangeCreatesBranchWhenMissing(t *testing.T) {
	executor := &stubGitExecutor{errors: []error{nil, errors.New("missing"), nil, nil}}
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
	executor := &stubGitExecutor{errors: []error{errors.New("fetch failed")}}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	_, changeError := service.Change(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main"})
	require.ErrorContains(t, changeError, "fetch updates")
}
