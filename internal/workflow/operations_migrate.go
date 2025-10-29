package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	migrate "github.com/temirov/gix/internal/migrate"
)

const (
	defaultMigrationRemoteNameConstant                 = "origin"
	defaultMigrationTargetBranchConstant               = "master"
	defaultMigrationWorkflowsDirectoryConstant         = ".github/workflows"
	migrationDryRunMessageTemplateConstant             = "WORKFLOW-PLAN: default %s (%s → %s)\n"
	migrationSuccessMessageTemplateConstant            = "WORKFLOW-DEFAULT: %s (%s → %s) safe_to_delete=%t\n"
	migrationIdentifierMissingMessageConstant          = "repository identifier unavailable for default-branch target"
	migrationExecutionErrorTemplateConstant            = "default branch update failed: %w"
	migrationRefreshErrorTemplateConstant              = "failed to refresh repository after default branch update: %w"
	migrationDependenciesMissingMessageConstant        = "default branch update requires repository manager, git executor, and GitHub client"
	migrationMultipleTargetsUnsupportedMessageConstant = "default branch update requires exactly one target configuration"
	migrationMetadataResolutionErrorTemplateConstant   = "default branch metadata resolution failed: %w"
	migrationMetadataMissingMessageConstant            = "repository metadata missing default branch for update"
	migrationSkipMessageTemplateConstant               = "WORKFLOW-DEFAULT-SKIP: %s already defaults to %s\n"
)

// BranchMigrationTarget describes branch migration behavior for discovered repositories.
type BranchMigrationTarget struct {
	RemoteName         string
	SourceBranch       string
	TargetBranch       string
	PushToRemote       bool
	DeleteSourceBranch bool
}

// BranchMigrationOperation performs default-branch migrations for configured targets.
type BranchMigrationOperation struct {
	Targets []BranchMigrationTarget
}

// Name identifies the operation type.
func (operation *BranchMigrationOperation) Name() string {
	return string(OperationTypeBranchDefault)
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

	if len(operation.Targets) == 0 {
		return nil
	}
	if len(operation.Targets) > 1 {
		return errors.New(migrationMultipleTargetsUnsupportedMessageConstant)
	}

	target := operation.Targets[0]
	repositories := state.CloneRepositories()

	for repositoryIndex := range repositories {
		repositoryState := repositories[repositoryIndex]
		if repositoryState == nil {
			continue
		}

		repositoryIdentifier, identifierError := resolveRepositoryIdentifier(repositoryState)
		if identifierError != nil {
			return identifierError
		}

		targetBranchValue := strings.TrimSpace(target.TargetBranch)
		if len(targetBranchValue) == 0 {
			targetBranchValue = defaultMigrationTargetBranchConstant
		}
		targetBranch := migrate.BranchName(targetBranchValue)

		sourceBranchValue := strings.TrimSpace(target.SourceBranch)
		if len(sourceBranchValue) == 0 {
			metadata, metadataError := environment.GitHubClient.ResolveRepoMetadata(executionContext, repositoryIdentifier)
			if metadataError != nil {
				return fmt.Errorf(migrationMetadataResolutionErrorTemplateConstant, metadataError)
			}
			sourceBranchValue = strings.TrimSpace(metadata.DefaultBranch)
			if len(sourceBranchValue) == 0 {
				return errors.New(migrationMetadataMissingMessageConstant)
			}
		}
		sourceBranch := migrate.BranchName(sourceBranchValue)

		if sourceBranch == targetBranch {
			if environment.Output != nil {
				fmt.Fprintf(environment.Output, migrationSkipMessageTemplateConstant, repositoryState.Path, sourceBranchValue)
			}
			continue
		}

		options := migrate.MigrationOptions{
			RepositoryPath:       repositoryState.Path,
			RepositoryRemoteName: target.RemoteName,
			RepositoryIdentifier: repositoryIdentifier,
			WorkflowsDirectory:   defaultMigrationWorkflowsDirectoryConstant,
			SourceBranch:         sourceBranch,
			TargetBranch:         targetBranch,
			PushUpdates:          target.PushToRemote,
			DeleteSourceBranch:   target.DeleteSourceBranch,
		}

		if environment.DryRun {
			if environment.Output != nil {
				fmt.Fprintf(environment.Output, migrationDryRunMessageTemplateConstant, repositoryState.Path, sourceBranchValue, targetBranchValue)
			}
			continue
		}

		result, executionError := migrationService.Execute(executionContext, options)
		if executionError != nil {
			var updateError migrate.DefaultBranchUpdateError
			if errors.As(executionError, &updateError) {
				return executionError
			}
			return fmt.Errorf(migrationExecutionErrorTemplateConstant, executionError)
		}

		if environment.Output != nil {
			fmt.Fprintf(environment.Output, migrationSuccessMessageTemplateConstant, repositoryState.Path, sourceBranchValue, targetBranchValue, result.SafetyStatus.SafeToDelete)
			for _, warning := range result.Warnings {
				fmt.Fprintln(environment.Output, warning)
			}
		}

		if refreshError := repositoryState.Refresh(executionContext, environment.AuditService); refreshError != nil {
			return fmt.Errorf(migrationRefreshErrorTemplateConstant, refreshError)
		}
	}

	return nil
}

func resolveRepositoryIdentifier(repositoryState *RepositoryState) (string, error) {
	if repositoryState == nil {
		return "", errors.New(migrationIdentifierMissingMessageConstant)
	}

	identifierCandidates := []string{
		repositoryState.Inspection.CanonicalOwnerRepo,
		repositoryState.Inspection.FinalOwnerRepo,
		repositoryState.Inspection.OriginOwnerRepo,
	}

	for _, candidate := range identifierCandidates {
		trimmed := strings.TrimSpace(candidate)
		if len(trimmed) > 0 {
			return trimmed, nil
		}
	}

	return "", errors.New(migrationIdentifierMissingMessageConstant)
}
