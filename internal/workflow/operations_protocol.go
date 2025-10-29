package workflow

import (
	"context"
	"fmt"
	"strings"

	conversion "github.com/temirov/gix/internal/repos/protocol"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	protocolRefreshErrorTemplateConstant = "failed to refresh repository after protocol conversion: %w"
)

// ProtocolConversionOperation converts repository remotes between protocols.
type ProtocolConversionOperation struct {
	FromProtocol shared.RemoteProtocol
	ToProtocol   shared.RemoteProtocol
}

// Name identifies the operation type.
func (operation *ProtocolConversionOperation) Name() string {
	return string(OperationTypeProtocolConversion)
}

// Execute applies the protocol conversion to repositories matching the source protocol.
func (operation *ProtocolConversionOperation) Execute(executionContext context.Context, environment *Environment, state *State) error {
	if environment == nil || state == nil {
		return nil
	}

	dependencies := conversion.Dependencies{
		GitManager: environment.RepositoryManager,
		Prompter:   environment.Prompter,
		Output:     environment.Output,
		Errors:     environment.Errors,
	}

	for repositoryIndex := range state.Repositories {
		repository := state.Repositories[repositoryIndex]

		actualProtocol, actualProtocolError := shared.ParseRemoteProtocol(string(repository.Inspection.RemoteProtocol))
		if actualProtocolError != nil {
			return fmt.Errorf("protocol conversion: %w", actualProtocolError)
		}

		if actualProtocol != operation.FromProtocol {
			continue
		}

		assumeYes := false
		if environment.PromptState != nil {
			assumeYes = environment.PromptState.IsAssumeYesEnabled()
		}

		repositoryPath, repositoryPathError := shared.NewRepositoryPath(repository.Path)
		if repositoryPathError != nil {
			return fmt.Errorf("protocol conversion: %w", repositoryPathError)
		}

		var originOwnerRepository *shared.OwnerRepository
		if trimmed := strings.TrimSpace(repository.Inspection.OriginOwnerRepo); len(trimmed) > 0 {
			ownerRepository, ownerRepositoryError := shared.NewOwnerRepository(trimmed)
			if ownerRepositoryError != nil {
				return fmt.Errorf("protocol conversion: %w", ownerRepositoryError)
			}
			originOwnerRepository = &ownerRepository
		}

		var canonicalOwnerRepository *shared.OwnerRepository
		if trimmed := strings.TrimSpace(repository.Inspection.CanonicalOwnerRepo); len(trimmed) > 0 {
			canonicalRepository, canonicalRepositoryError := shared.NewOwnerRepository(trimmed)
			if canonicalRepositoryError != nil {
				return fmt.Errorf("protocol conversion: %w", canonicalRepositoryError)
			}
			canonicalOwnerRepository = &canonicalRepository
		}

		options := conversion.Options{
			RepositoryPath:           repositoryPath,
			OriginOwnerRepository:    originOwnerRepository,
			CanonicalOwnerRepository: canonicalOwnerRepository,
			CurrentProtocol:          operation.FromProtocol,
			TargetProtocol:           operation.ToProtocol,
			DryRun:                   environment.DryRun,
			AssumeYes:                assumeYes,
		}

		conversion.Execute(executionContext, dependencies, options)

		if environment.DryRun {
			continue
		}

		if refreshError := repository.Refresh(executionContext, environment.AuditService); refreshError != nil {
			return fmt.Errorf(protocolRefreshErrorTemplateConstant, refreshError)
		}
	}

	return nil
}
