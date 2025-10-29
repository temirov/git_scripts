package workflow

import (
	"context"
	"fmt"

	"github.com/temirov/gix/internal/repos/remotes"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	canonicalRemoteRefreshErrorTemplateConstant = "failed to refresh repository after canonical remote update: %w"
)

// CanonicalRemoteOperation updates origin URLs to their canonical GitHub equivalents.
type CanonicalRemoteOperation struct {
	OwnerConstraint string
}

// Name identifies the operation type.
func (operation *CanonicalRemoteOperation) Name() string {
	return string(OperationTypeCanonicalRemote)
}

// Execute applies canonical remote updates using inspection metadata.
func (operation *CanonicalRemoteOperation) Execute(executionContext context.Context, environment *Environment, state *State) error {
	if environment == nil || state == nil {
		return nil
	}

	dependencies := remotes.Dependencies{
		GitManager: environment.RepositoryManager,
		Prompter:   environment.Prompter,
		Reporter:   shared.NewWriterReporter(environment.Output),
	}

	for repositoryIndex := range state.Repositories {
		repository := state.Repositories[repositoryIndex]
		originOwnerRepository, originOwnerError := shared.ParseOwnerRepositoryOptional(repository.Inspection.OriginOwnerRepo)
		if originOwnerError != nil {
			return fmt.Errorf("canonical remote update: %w", originOwnerError)
		}
		canonicalOwnerRepository, canonicalOwnerError := shared.ParseOwnerRepositoryOptional(repository.Inspection.CanonicalOwnerRepo)
		if canonicalOwnerError != nil {
			return fmt.Errorf("canonical remote update: %w", canonicalOwnerError)
		}
		if originOwnerRepository == nil && canonicalOwnerRepository == nil {
			continue
		}
		assumeYes := false
		if environment.PromptState != nil {
			assumeYes = environment.PromptState.IsAssumeYesEnabled()
		}

		repositoryPath, repositoryPathError := shared.NewRepositoryPath(repository.Path)
		if repositoryPathError != nil {
			return fmt.Errorf("canonical remote update: %w", repositoryPathError)
		}

		currentRemoteURL, currentRemoteURLError := shared.ParseRemoteURLOptional(repository.Inspection.OriginURL)
		if currentRemoteURLError != nil {
			return fmt.Errorf("canonical remote update: %w", currentRemoteURLError)
		}

		remoteProtocol, remoteProtocolError := shared.ParseRemoteProtocol(string(repository.Inspection.RemoteProtocol))
		if remoteProtocolError != nil {
			return fmt.Errorf("canonical remote update: %w", remoteProtocolError)
		}

		ownerConstraint, ownerConstraintError := shared.ParseOwnerSlugOptional(operation.OwnerConstraint)
		if ownerConstraintError != nil {
			return fmt.Errorf("canonical remote update: %w", ownerConstraintError)
		}

		options := remotes.Options{
			RepositoryPath:           repositoryPath,
			CurrentOriginURL:         currentRemoteURL,
			OriginOwnerRepository:    originOwnerRepository,
			CanonicalOwnerRepository: canonicalOwnerRepository,
			RemoteProtocol:           remoteProtocol,
			DryRun:                   environment.DryRun,
			ConfirmationPolicy:       shared.ConfirmationPolicyFromBool(assumeYes),
			OwnerConstraint:          ownerConstraint,
		}

		if executionError := remotes.Execute(executionContext, dependencies, options); executionError != nil {
			if logRepositoryOperationError(environment, executionError) {
				continue
			}
			return fmt.Errorf("canonical remote update: %w", executionError)
		}

		if environment.DryRun {
			continue
		}

		if refreshError := repository.Refresh(executionContext, environment.AuditService); refreshError != nil {
			return fmt.Errorf(canonicalRemoteRefreshErrorTemplateConstant, refreshError)
		}
	}

	return nil
}
