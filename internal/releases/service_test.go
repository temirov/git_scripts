package releases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/execshell"
)

type recordingGitExecutor struct {
	commands []execshell.CommandDetails
	errors   []error
}

func (executor *recordingGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.commands = append(executor.commands, details)
	if len(executor.errors) == 0 {
		return execshell.ExecutionResult{}, nil
	}
	value := executor.errors[0]
	executor.errors = executor.errors[1:]
	if value != nil {
		return execshell.ExecutionResult{}, value
	}
	return execshell.ExecutionResult{}, nil
}

func (executor *recordingGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func TestReleaseExecutesTagAndPush(t *testing.T) {
	executor := &recordingGitExecutor{}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	result, releaseError := service.Release(context.Background(), Options{RepositoryPath: "/tmp/repo", TagName: "v1.2.3", RemoteName: "origin"})
	require.NoError(t, releaseError)
	require.Equal(t, Result{RepositoryPath: "/tmp/repo", TagName: "v1.2.3"}, result)
	require.Len(t, executor.commands, 2)
}

func TestReleaseDryRunSkipsGitCommands(t *testing.T) {
	executor := &recordingGitExecutor{}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	_, releaseError := service.Release(context.Background(), Options{RepositoryPath: "/tmp/repo", TagName: "v1.0.0", DryRun: true})
	require.NoError(t, releaseError)
	require.Empty(t, executor.commands)
}

func TestReleaseValidatesInputs(t *testing.T) {
	service, err := NewService(ServiceDependencies{GitExecutor: &recordingGitExecutor{}})
	require.NoError(t, err)

	_, releaseError := service.Release(context.Background(), Options{TagName: "v1.0.0"})
	require.Error(t, releaseError)

	_, releaseError = service.Release(context.Background(), Options{RepositoryPath: "/tmp/repo"})
	require.Error(t, releaseError)
}

func TestReleasePropagatesErrors(t *testing.T) {
	executor := &recordingGitExecutor{errors: []error{errors.New("tag failed")}}
	service, err := NewService(ServiceDependencies{GitExecutor: executor})
	require.NoError(t, err)

	_, releaseError := service.Release(context.Background(), Options{RepositoryPath: "/tmp/repo", TagName: "v1.0.0"})
	require.ErrorContains(t, releaseError, "tag failed")
}
