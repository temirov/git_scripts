package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/audit"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
	"github.com/temirov/git_scripts/internal/repos/shared"
	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

var workflowExecutorHomeDirectoryExpander = pathutils.NewHomeExpander()

const (
	defaultWorkflowRootConstant            = "."
	workflowExecutionErrorTemplateConstant = "workflow operation %s failed: %w"
	workflowExecutorDependenciesMessage    = "workflow executor requires repository discovery, git, and GitHub dependencies"
	workflowRepositoryLoadErrorTemplate    = "failed to inspect repositories: %w"
)

// Dependencies configures shared collaborators for workflow execution.
type Dependencies struct {
	Logger               *zap.Logger
	RepositoryDiscoverer shared.RepositoryDiscoverer
	GitExecutor          shared.GitExecutor
	RepositoryManager    *gitrepo.RepositoryManager
	GitHubClient         *githubcli.Client
	FileSystem           shared.FileSystem
	Prompter             shared.ConfirmationPrompter
	Output               io.Writer
	Errors               io.Writer
}

// RuntimeOptions captures user-provided execution modifiers.
type RuntimeOptions struct {
	DryRun    bool
	AssumeYes bool
}

// Executor coordinates workflow operation execution.
type Executor struct {
	operations   []Operation
	dependencies Dependencies
}

// NewExecutor constructs an Executor instance.
func NewExecutor(operations []Operation, dependencies Dependencies) *Executor {
	return &Executor{operations: append([]Operation{}, operations...), dependencies: dependencies}
}

// Execute orchestrates workflow operations across discovered repositories.
func (executor *Executor) Execute(executionContext context.Context, roots []string, runtimeOptions RuntimeOptions) error {
	if executor.dependencies.RepositoryDiscoverer == nil || executor.dependencies.GitExecutor == nil || executor.dependencies.RepositoryManager == nil || executor.dependencies.GitHubClient == nil {
		return errors.New(workflowExecutorDependenciesMessage)
	}

	sanitizedRoots := sanitizeRoots(roots)

	auditService := audit.NewService(
		executor.dependencies.RepositoryDiscoverer,
		executor.dependencies.RepositoryManager,
		executor.dependencies.GitExecutor,
		executor.dependencies.GitHubClient,
		executor.dependencies.Output,
		executor.dependencies.Errors,
	)

	inspections, inspectionError := auditService.DiscoverInspections(executionContext, sanitizedRoots, false)
	if inspectionError != nil {
		return fmt.Errorf(workflowRepositoryLoadErrorTemplate, inspectionError)
	}

	repositoryStates := make([]*RepositoryState, 0, len(inspections))
	for inspectionIndex := range inspections {
		repositoryStates = append(repositoryStates, NewRepositoryState(inspections[inspectionIndex]))
	}

	state := &State{Roots: sanitizedRoots, Repositories: repositoryStates}
	environment := &Environment{
		AuditService:      auditService,
		GitExecutor:       executor.dependencies.GitExecutor,
		RepositoryManager: executor.dependencies.RepositoryManager,
		GitHubClient:      executor.dependencies.GitHubClient,
		FileSystem:        executor.dependencies.FileSystem,
		Prompter:          executor.dependencies.Prompter,
		Output:            executor.dependencies.Output,
		Errors:            executor.dependencies.Errors,
		Logger:            executor.dependencies.Logger,
		DryRun:            runtimeOptions.DryRun,
		AssumeYes:         runtimeOptions.AssumeYes,
	}

	for operationIndex := range executor.operations {
		operation := executor.operations[operationIndex]
		if operation == nil {
			continue
		}
		if executeError := operation.Execute(executionContext, environment, state); executeError != nil {
			return fmt.Errorf(workflowExecutionErrorTemplateConstant, operation.Name(), executeError)
		}
	}

	return nil
}

func sanitizeRoots(rawRoots []string) []string {
	if len(rawRoots) == 0 {
		return []string{defaultWorkflowRootConstant}
	}

	sanitized := make([]string, 0, len(rawRoots))
	for rootIndex := range rawRoots {
		trimmed := strings.TrimSpace(rawRoots[rootIndex])
		if len(trimmed) == 0 {
			continue
		}
		expandedRoot := workflowExecutorHomeDirectoryExpander.Expand(trimmed)
		sanitized = append(sanitized, expandedRoot)
	}

	if len(sanitized) == 0 {
		return []string{defaultWorkflowRootConstant}
	}

	return sanitized
}
