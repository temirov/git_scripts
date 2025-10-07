package refresh

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/execshell"
)

type stubGitExecutor struct {
	invocationErrors []error
	recordedCommands []execshell.CommandDetails
}

func (executor *stubGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.recordedCommands = append(executor.recordedCommands, details)
	if len(executor.invocationErrors) == 0 {
		return execshell.ExecutionResult{}, nil
	}
	err := executor.invocationErrors[0]
	executor.invocationErrors = executor.invocationErrors[1:]
	if err != nil {
		return execshell.ExecutionResult{}, err
	}
	return execshell.ExecutionResult{}, nil
}

func (executor *stubGitExecutor) ExecuteGitHubCLI(_ context.Context, _ execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type stubRepositoryManager struct {
	cleanState     bool
	executionError error
}

func (manager stubRepositoryManager) CheckCleanWorktree(context.Context, string) (bool, error) {
	return manager.cleanState, manager.executionError
}

func (stubRepositoryManager) GetCurrentBranch(context.Context, string) (string, error) {
	return "", nil
}

func (stubRepositoryManager) GetRemoteURL(context.Context, string, string) (string, error) {
	return "", nil
}

func (stubRepositoryManager) SetRemoteURL(context.Context, string, string, string) error {
	return nil
}

func TestNewServiceValidatesDependencies(t *testing.T) {
	testCases := []struct {
		name         string
		dependencies Dependencies
		expectedErr  error
	}{
		{
			name:         "MissingGitExecutor",
			dependencies: Dependencies{RepositoryManager: stubRepositoryManager{cleanState: true}},
			expectedErr:  ErrGitExecutorNotConfigured,
		},
		{
			name:         "MissingRepositoryManager",
			dependencies: Dependencies{GitExecutor: &stubGitExecutor{}},
			expectedErr:  ErrRepositoryManagerNotConfigured,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			service, creationError := NewService(testCase.dependencies)
			require.ErrorIs(t, creationError, testCase.expectedErr)
			require.Nil(t, service)
		})
	}

	service, creationError := NewService(Dependencies{GitExecutor: &stubGitExecutor{}, RepositoryManager: stubRepositoryManager{cleanState: true}})
	require.NoError(t, creationError)
	require.NotNil(t, service)
}

func TestRefreshValidatesInputs(t *testing.T) {
	executor := &stubGitExecutor{}
	service, creationError := NewService(Dependencies{GitExecutor: executor, RepositoryManager: stubRepositoryManager{cleanState: true}})
	require.NoError(t, creationError)

	_, err := service.Refresh(context.Background(), Options{BranchName: "main", RequireClean: true})
	require.ErrorIs(t, err, ErrRepositoryPathRequired)

	_, err = service.Refresh(context.Background(), Options{RepositoryPath: "/tmp/repo", RequireClean: true})
	require.ErrorIs(t, err, ErrBranchNameRequired)
}

func TestRefreshPropagatesCleanCheckError(t *testing.T) {
	executor := &stubGitExecutor{}
	repositoryManager := stubRepositoryManager{cleanState: false, executionError: errors.New("status failed")}
	service, creationError := NewService(Dependencies{GitExecutor: executor, RepositoryManager: repositoryManager})
	require.NoError(t, creationError)

	_, err := service.Refresh(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main", RequireClean: true})
	require.ErrorContains(t, err, "failed to verify clean worktree")
}

func TestRefreshFailsWhenWorktreeDirty(t *testing.T) {
	executor := &stubGitExecutor{}
	repositoryManager := stubRepositoryManager{cleanState: false}
	service, creationError := NewService(Dependencies{GitExecutor: executor, RepositoryManager: repositoryManager})
	require.NoError(t, creationError)

	_, err := service.Refresh(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main", RequireClean: true})
	require.ErrorIs(t, err, ErrWorktreeNotClean)
}

func TestRefreshExecutesGitCommandsInOrder(t *testing.T) {
	executor := &stubGitExecutor{}
	repositoryManager := stubRepositoryManager{cleanState: true}
	service, creationError := NewService(Dependencies{GitExecutor: executor, RepositoryManager: repositoryManager})
	require.NoError(t, creationError)

	result, err := service.Refresh(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main", RequireClean: true})
	require.NoError(t, err)
	require.Equal(t, Result{RepositoryPath: "/tmp/repo", BranchName: "main"}, result)
	require.Len(t, executor.recordedCommands, 3)
	require.Equal(t, []string{gitFetchSubcommandConstant, gitFetchPruneFlagConstant}, executor.recordedCommands[0].Arguments)
	require.Equal(t, []string{gitCheckoutSubcommandConstant, "main"}, executor.recordedCommands[1].Arguments)
	require.Equal(t, []string{gitPullSubcommandConstant, gitPullFastForwardFlagConstant}, executor.recordedCommands[2].Arguments)

	for _, commandDetails := range executor.recordedCommands {
		require.Equal(t, gitTerminalPromptEnvironmentDisableConstant, commandDetails.EnvironmentVariables[gitTerminalPromptEnvironmentNameConstant])
	}
}

func TestRefreshSurfacesGitFailures(t *testing.T) {
	testError := errors.New("execution failed")
	testCases := []struct {
		name             string
		errors           []error
		expectedFragment string
	}{
		{
			name:             "FetchFailure",
			errors:           []error{testError},
			expectedFragment: "failed to fetch updates",
		},
		{
			name:             "CheckoutFailure",
			errors:           []error{nil, testError},
			expectedFragment: "failed to checkout branch",
		},
		{
			name:             "PullFailure",
			errors:           []error{nil, nil, testError},
			expectedFragment: "failed to pull latest changes",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			executor := &stubGitExecutor{invocationErrors: append([]error{}, testCase.errors...)}
			repositoryManager := stubRepositoryManager{cleanState: true}
			service, creationError := NewService(Dependencies{GitExecutor: executor, RepositoryManager: repositoryManager})
			require.NoError(t, creationError)

			_, err := service.Refresh(context.Background(), Options{RepositoryPath: "/tmp/repo", BranchName: "main", RequireClean: true})
			require.ErrorContains(t, err, testCase.expectedFragment)
			require.Contains(t, err.Error(), testError.Error())
		})
	}
}
