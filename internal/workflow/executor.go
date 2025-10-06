package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.uber.org/zap"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
	"github.com/temirov/gix/internal/repos/shared"
	pathutils "github.com/temirov/gix/internal/utils/path"
)

var workflowExecutorRepositoryPathSanitizer = pathutils.NewRepositoryPathSanitizerWithConfiguration(nil, pathutils.RepositoryPathSanitizerConfiguration{PruneNestedPaths: true})

const (
	workflowExecutionErrorTemplateConstant = "workflow operation %s failed: %w"
	workflowExecutorDependenciesMessage    = "workflow executor requires repository discovery, git, and GitHub dependencies"
	workflowExecutorMissingRootsMessage    = "workflow executor requires at least one repository root"
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

	sanitizedRoots := workflowExecutorRepositoryPathSanitizer.Sanitize(roots)
	if len(sanitizedRoots) == 0 {
		return errors.New(workflowExecutorMissingRootsMessage)
	}

	auditService := audit.NewService(
		executor.dependencies.RepositoryDiscoverer,
		executor.dependencies.RepositoryManager,
		executor.dependencies.GitExecutor,
		executor.dependencies.GitHubClient,
		executor.dependencies.Output,
		executor.dependencies.Errors,
	)

	inspections, inspectionError := auditService.DiscoverInspections(executionContext, sanitizedRoots, false, audit.InspectionDepthFull)
	if inspectionError != nil {
		return fmt.Errorf(workflowRepositoryLoadErrorTemplate, inspectionError)
	}

	repositoryStates := make([]*RepositoryState, 0, len(inspections))
	for inspectionIndex := range inspections {
		repositoryStates = append(repositoryStates, NewRepositoryState(inspections[inspectionIndex]))
	}

	promptState := NewPromptState(runtimeOptions.AssumeYes)
	dispatchingPrompter := newPromptDispatcher(executor.dependencies.Prompter, promptState)

	state := &State{Roots: sanitizedRoots, Repositories: repositoryStates}
	environment := &Environment{
		AuditService:      auditService,
		GitExecutor:       executor.dependencies.GitExecutor,
		RepositoryManager: executor.dependencies.RepositoryManager,
		GitHubClient:      executor.dependencies.GitHubClient,
		FileSystem:        executor.dependencies.FileSystem,
		Prompter:          dispatchingPrompter,
		PromptState:       promptState,
		Output:            executor.dependencies.Output,
		Errors:            executor.dependencies.Errors,
		Logger:            executor.dependencies.Logger,
		DryRun:            runtimeOptions.DryRun,
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
