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
type CanonicalRemoteOperation struct{}

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

		options := remotes.Options{
			RepositoryPath:           repository.Path,
			CurrentOriginURL:         repository.Inspection.OriginURL,
			OriginOwnerRepository:    repository.Inspection.OriginOwnerRepo,
			CanonicalOwnerRepository: repository.Inspection.CanonicalOwnerRepo,
			RemoteProtocol:           shared.RemoteProtocol(repository.Inspection.RemoteProtocol),
			DryRun:                   environment.DryRun,
			AssumeYes:                environment.AssumeYes,
		}

		remotes.Execute(executionContext, dependencies, options)

		if environment.DryRun {
			continue
		}

		if refreshError := repository.Refresh(executionContext, environment.AuditService); refreshError != nil {
			return fmt.Errorf(canonicalRemoteRefreshErrorTemplateConstant, refreshError)
		}
	}

	return nil
}
