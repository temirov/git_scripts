package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
)

type fakeGitExecutor struct{}

func (fakeGitExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func (fakeGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
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
