package workflow

import (
	"context"
	"fmt"
	"strings"

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
		Output:     environment.Output,
	}

	for repositoryIndex := range state.Repositories {
		repository := state.Repositories[repositoryIndex]
		originOwner := strings.TrimSpace(repository.Inspection.OriginOwnerRepo)
		canonicalOwner := strings.TrimSpace(repository.Inspection.CanonicalOwnerRepo)
		if len(originOwner) == 0 && len(canonicalOwner) == 0 {
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

		var currentRemoteURL *shared.RemoteURL
		if trimmedURL := strings.TrimSpace(repository.Inspection.OriginURL); len(trimmedURL) > 0 {
			remoteURL, remoteURLError := shared.NewRemoteURL(trimmedURL)
			if remoteURLError != nil {
				return fmt.Errorf("canonical remote update: %w", remoteURLError)
			}
			currentRemoteURL = &remoteURL
		}

		var originOwnerRepository *shared.OwnerRepository
		if len(originOwner) > 0 {
			ownerRepository, ownerRepositoryError := shared.NewOwnerRepository(originOwner)
			if ownerRepositoryError != nil {
				return fmt.Errorf("canonical remote update: %w", ownerRepositoryError)
			}
			originOwnerRepository = &ownerRepository
		}

		var canonicalOwnerRepository *shared.OwnerRepository
		if len(canonicalOwner) > 0 {
			canonicalRepository, canonicalRepositoryError := shared.NewOwnerRepository(canonicalOwner)
			if canonicalRepositoryError != nil {
				return fmt.Errorf("canonical remote update: %w", canonicalRepositoryError)
			}
			canonicalOwnerRepository = &canonicalRepository
		}

		remoteProtocol, remoteProtocolError := shared.ParseRemoteProtocol(string(repository.Inspection.RemoteProtocol))
		if remoteProtocolError != nil {
			return fmt.Errorf("canonical remote update: %w", remoteProtocolError)
		}

		var ownerConstraint *shared.OwnerSlug
		if trimmedConstraint := strings.TrimSpace(operation.OwnerConstraint); len(trimmedConstraint) > 0 {
			constraint, constraintError := shared.NewOwnerSlug(trimmedConstraint)
			if constraintError != nil {
				return fmt.Errorf("canonical remote update: %w", constraintError)
			}
			ownerConstraint = &constraint
		}

		options := remotes.Options{
			RepositoryPath:           repositoryPath,
			CurrentOriginURL:         currentRemoteURL,
			OriginOwnerRepository:    originOwnerRepository,
			CanonicalOwnerRepository: canonicalOwnerRepository,
			RemoteProtocol:           remoteProtocol,
			DryRun:                   environment.DryRun,
			AssumeYes:                assumeYes,
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
