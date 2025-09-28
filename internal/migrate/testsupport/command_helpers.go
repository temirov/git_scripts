package testsupport

import (
	"context"
	"fmt"

	"github.com/temirov/git_scripts/internal/execshell"
	migrate "github.com/temirov/git_scripts/internal/migrate"
)

// RepositoryDiscovererStub implements repository discovery for tests.
type RepositoryDiscovererStub struct {
	Repositories   []string
	DiscoveryError error
	ReceivedRoots  []string
}

// DiscoverRepositories records the requested roots and returns the configured repositories.
func (discoverer *RepositoryDiscovererStub) DiscoverRepositories(roots []string) ([]string, error) {
	discoverer.ReceivedRoots = append([]string{}, roots...)
	if discoverer.DiscoveryError != nil {
		return nil, discoverer.DiscoveryError
	}
	return append([]string{}, discoverer.Repositories...), nil
}

// CommandExecutorStub records git and GitHub CLI invocations for assertions.
type CommandExecutorStub struct {
	RepositoryRemotes      map[string]string
	RepositoryErrors       map[string]error
	ExecutedGitCommands    []execshell.CommandDetails
	ExecutedGitHubCommands []execshell.CommandDetails
}

// ExecuteGit returns the configured remote output or error for the working directory.
func (executor *CommandExecutorStub) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.ExecutedGitCommands = append(executor.ExecutedGitCommands, details)
	if executor.RepositoryErrors != nil {
		if repositoryError, exists := executor.RepositoryErrors[details.WorkingDirectory]; exists {
			return execshell.ExecutionResult{}, repositoryError
		}
	}
	if executor.RepositoryRemotes != nil {
		if remote, exists := executor.RepositoryRemotes[details.WorkingDirectory]; exists {
			return execshell.ExecutionResult{StandardOutput: remote, ExitCode: 0}, nil
		}
	}
	return execshell.ExecutionResult{}, fmt.Errorf("no remote configured for %s", details.WorkingDirectory)
}

// ExecuteGitHubCLI records GitHub CLI commands without mutating state.
func (executor *CommandExecutorStub) ExecuteGitHubCLI(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.ExecutedGitHubCommands = append(executor.ExecutedGitHubCommands, details)
	return execshell.ExecutionResult{ExitCode: 0}, nil
}

// ServiceOutcome configures the result returned by ServiceStub for a repository.
type ServiceOutcome struct {
	Result migrate.MigrationResult
	Error  error
}

// ServiceStub captures migration execution requests for verification.
type ServiceStub struct {
	Outcomes        map[string]ServiceOutcome
	ExecutedOptions []migrate.MigrationOptions
}

// Execute returns the configured outcome for the repository path in the provided options.
func (service *ServiceStub) Execute(_ context.Context, options migrate.MigrationOptions) (migrate.MigrationResult, error) {
	service.ExecutedOptions = append(service.ExecutedOptions, options)
	if service.Outcomes == nil {
		return migrate.MigrationResult{}, nil
	}
	outcome, exists := service.Outcomes[options.RepositoryPath]
	if !exists {
		return migrate.MigrationResult{}, nil
	}
	return outcome.Result, outcome.Error
}
