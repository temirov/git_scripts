package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/githubauth"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
	migrate "github.com/temirov/gix/internal/migrate"
)

type fakeGitExecutor struct{}

func (fakeGitExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func (fakeGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type defaultBranchFailureExecutor struct {
	failure execshell.CommandFailedError
}

func newDefaultBranchFailureExecutor(message string) defaultBranchFailureExecutor {
	return defaultBranchFailureExecutor{
		failure: execshell.CommandFailedError{
			Command: execshell.ShellCommand{Name: execshell.CommandGitHub},
			Result: execshell.ExecutionResult{
				ExitCode:      1,
				StandardError: message,
			},
		},
	}
}

func (executor defaultBranchFailureExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func (executor defaultBranchFailureExecutor) ExecuteGitHubCLI(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	if len(details.Arguments) >= 2 && details.Arguments[0] == "api" {
		for _, argument := range details.Arguments {
			if strings.Contains(argument, "default_branch=") {
				return execshell.ExecutionResult{}, executor.failure
			}
		}
	}
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}

func TestBranchMigrationOperationRequiresSingleTarget(testInstance *testing.T) {
	executor := fakeGitExecutor{}

	repositoryManager, managerError := gitrepo.NewRepositoryManager(executor)
	require.NoError(testInstance, managerError)

	githubClient, clientError := githubcli.NewClient(executor)
	require.NoError(testInstance, clientError)

	operation := &BranchMigrationOperation{Targets: []BranchMigrationTarget{
		{RemoteName: "origin", SourceBranch: "main", TargetBranch: "master"},
		{RemoteName: "upstream", SourceBranch: "develop", TargetBranch: "main"},
	}}

	environment := &Environment{RepositoryManager: repositoryManager, GitExecutor: executor, GitHubClient: githubClient}

	executionError := operation.Execute(context.Background(), environment, &State{})

	require.Error(testInstance, executionError)
	require.EqualError(testInstance, executionError, migrationMultipleTargetsUnsupportedMessageConstant)
}

func TestBranchMigrationOperationReturnsActionableDefaultBranchError(testInstance *testing.T) {
	testInstance.Setenv(githubauth.EnvGitHubCLIToken, "test-token")
	testInstance.Setenv(githubauth.EnvGitHubToken, "test-token")
	executor := newDefaultBranchFailureExecutor("GraphQL: branch not found")

	repositoryManager, managerError := gitrepo.NewRepositoryManager(executor)
	require.NoError(testInstance, managerError)

	githubClient, clientError := githubcli.NewClient(executor)
	require.NoError(testInstance, clientError)

	operation := &BranchMigrationOperation{Targets: []BranchMigrationTarget{
		{RemoteName: "origin", SourceBranch: "main", TargetBranch: "master"},
	}}

	repositoryPath := testInstance.TempDir()

	state := &State{
		Repositories: []*RepositoryState{
			{
				Path: repositoryPath,
				Inspection: audit.RepositoryInspection{
					CanonicalOwnerRepo: "owner/example",
				},
			},
		},
	}

	environment := &Environment{
		RepositoryManager: repositoryManager,
		GitExecutor:       executor,
		GitHubClient:      githubClient,
	}

	executionError := operation.Execute(context.Background(), environment, state)

	require.Error(testInstance, executionError)

	var updateError migrate.DefaultBranchUpdateError
	require.ErrorAs(testInstance, executionError, &updateError)

	errorMessage := executionError.Error()
	require.Contains(testInstance, errorMessage, repositoryPath)
	require.Contains(testInstance, errorMessage, "owner/example")
	require.Contains(testInstance, errorMessage, "source=main")
	require.Contains(testInstance, errorMessage, "target=master")
	require.Contains(testInstance, errorMessage, "GraphQL: branch not found")
	require.NotContains(testInstance, errorMessage, "default branch update failed")
}
