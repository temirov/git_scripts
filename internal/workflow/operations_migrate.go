package workflow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	migrate "github.com/temirov/git_scripts/internal/migrate"
)

const (
	defaultMigrationRemoteNameConstant          = "origin"
	defaultMigrationSourceBranchConstant        = "main"
	defaultMigrationTargetBranchConstant        = "master"
	defaultMigrationWorkflowsDirectoryConstant  = ".github/workflows"
	migrationDryRunMessageTemplateConstant      = "WORKFLOW-PLAN: migrate %s (%s → %s)\n"
	migrationSuccessMessageTemplateConstant     = "WORKFLOW-MIGRATE: %s (%s → %s) safe_to_delete=%t\n"
	migrationTargetNotFoundTemplateConstant     = "migrate-branch target did not match any repository (%s)"
	migrationIdentifierMissingMessageConstant   = "repository identifier unavailable for migration target"
	migrationExecutionErrorTemplateConstant     = "branch migration failed: %w"
	migrationRefreshErrorTemplateConstant       = "failed to refresh repository after branch migration: %w"
	migrationDependenciesMissingMessageConstant = "branch migration requires repository manager, git executor, and GitHub client"
)

// BranchMigrationTarget describes repositories eligible for migration.
type BranchMigrationTarget struct {
	RepositoryIdentifier string
	RepositoryPath       string
	RemoteName           string
	SourceBranch         string
	TargetBranch         string
	WorkflowsDirectory   string
	PushUpdates          bool
}

// BranchMigrationOperation performs default-branch migrations for configured targets.
type BranchMigrationOperation struct {
	Targets []BranchMigrationTarget
}

// Name identifies the operation type.
func (operation *BranchMigrationOperation) Name() string {
	return string(OperationTypeBranchMigration)
}

// Execute performs branch migration workflows for configured targets.
func (operation *BranchMigrationOperation) Execute(executionContext context.Context, environment *Environment, state *State) error {
	if environment == nil || state == nil {
		return nil
	}

	if environment.RepositoryManager == nil || environment.GitExecutor == nil || environment.GitHubClient == nil {
		return errors.New(migrationDependenciesMissingMessageConstant)
	}

	serviceDependencies := migrate.ServiceDependencies{
		Logger:            environment.Logger,
		RepositoryManager: environment.RepositoryManager,
		GitHubClient:      environment.GitHubClient,
		GitExecutor:       environment.GitExecutor,
	}

	migrationService, serviceError := migrate.NewService(serviceDependencies)
	if serviceError != nil {
		return fmt.Errorf(migrationExecutionErrorTemplateConstant, serviceError)
	}

	for targetIndex := range operation.Targets {
		target := operation.Targets[targetIndex]
		repositoryState, resolveError := resolveMigrationTarget(state.Repositories, target)
		if resolveError != nil {
			return resolveError
		}

		repositoryIdentifier := strings.TrimSpace(target.RepositoryIdentifier)
		if len(repositoryIdentifier) == 0 {
			repositoryIdentifier = strings.TrimSpace(repositoryState.Inspection.CanonicalOwnerRepo)
		}
		if len(repositoryIdentifier) == 0 {
			repositoryIdentifier = strings.TrimSpace(repositoryState.Inspection.OriginOwnerRepo)
		}
		if len(repositoryIdentifier) == 0 {
			return errors.New(migrationIdentifierMissingMessageConstant)
		}

		options := migrate.MigrationOptions{
			RepositoryPath:       repositoryState.Path,
			RepositoryRemoteName: target.RemoteName,
			RepositoryIdentifier: repositoryIdentifier,
			WorkflowsDirectory:   target.WorkflowsDirectory,
			SourceBranch:         migrate.BranchName(target.SourceBranch),
			TargetBranch:         migrate.BranchName(target.TargetBranch),
			PushUpdates:          target.PushUpdates,
		}

		if environment.DryRun {
			if environment.Output != nil {
				fmt.Fprintf(environment.Output, migrationDryRunMessageTemplateConstant, repositoryState.Path, target.SourceBranch, target.TargetBranch)
			}
			continue
		}

		result, executionError := migrationService.Execute(executionContext, options)
		if executionError != nil {
			return fmt.Errorf(migrationExecutionErrorTemplateConstant, executionError)
		}

		if environment.Output != nil {
			fmt.Fprintf(environment.Output, migrationSuccessMessageTemplateConstant, repositoryState.Path, target.SourceBranch, target.TargetBranch, result.SafetyStatus.SafeToDelete)
		}

		if refreshError := repositoryState.Refresh(executionContext, environment.AuditService); refreshError != nil {
			return fmt.Errorf(migrationRefreshErrorTemplateConstant, refreshError)
		}
	}

	return nil
}

func resolveMigrationTarget(states []*RepositoryState, target BranchMigrationTarget) (*RepositoryState, error) {
	trimmedPath := strings.TrimSpace(target.RepositoryPath)
	if len(trimmedPath) > 0 {
		normalizedTarget := filepath.Clean(trimmedPath)
		for repositoryIndex := range states {
			repository := states[repositoryIndex]
			if filepath.Clean(repository.Path) == normalizedTarget {
				return repository, nil
			}
		}
	}

	trimmedIdentifier := strings.TrimSpace(target.RepositoryIdentifier)
	if len(trimmedIdentifier) > 0 {
		for repositoryIndex := range states {
			repository := states[repositoryIndex]
			if identifiersMatch(repository, trimmedIdentifier) {
				return repository, nil
			}
		}
	}

	return nil, fmt.Errorf(migrationTargetNotFoundTemplateConstant, describeTarget(target))
}

func identifiersMatch(repository *RepositoryState, identifier string) bool {
	if repository == nil {
		return false
	}
	trimmed := strings.TrimSpace(identifier)
	if len(trimmed) == 0 {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(repository.Inspection.CanonicalOwnerRepo), trimmed) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(repository.Inspection.FinalOwnerRepo), trimmed) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(repository.Inspection.OriginOwnerRepo), trimmed) {
		return true
	}
	return false
}

func describeTarget(target BranchMigrationTarget) string {
	if len(strings.TrimSpace(target.RepositoryPath)) > 0 {
		return target.RepositoryPath
	}
	if len(strings.TrimSpace(target.RepositoryIdentifier)) > 0 {
		return target.RepositoryIdentifier
	}
	return "unknown"
}
