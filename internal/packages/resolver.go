package packages

import (
	"github.com/temirov/git_scripts/internal/ghcr"
	"go.uber.org/zap"
)

// DefaultPurgeServiceResolver builds purge services using GHCR APIs and token resolution.
type DefaultPurgeServiceResolver struct {
	HTTPClient        ghcr.HTTPClient
	ServiceBaseURL    string
	PageSize          int
	EnvironmentLookup EnvironmentLookup
	FileReader        FileReader
	TokenResolver     TokenResolver
}

// Resolve creates a purge executor using configured collaborators or sensible defaults.
func (resolver *DefaultPurgeServiceResolver) Resolve(logger *zap.Logger) (PurgeExecutor, error) {
	serviceConfiguration := ghcr.ServiceConfiguration{
		BaseURL:  resolver.ServiceBaseURL,
		PageSize: resolver.PageSize,
	}

	packageService, serviceCreationError := ghcr.NewPackageVersionService(logger, resolver.HTTPClient, serviceConfiguration)
	if serviceCreationError != nil {
		return nil, serviceCreationError
	}

	resolvedTokenResolver := resolver.TokenResolver
	if resolvedTokenResolver == nil {
		resolvedTokenResolver = NewTokenResolver(resolver.EnvironmentLookup, resolver.FileReader)
	}

	purgeService, purgeServiceError := NewPurgeService(logger, packageService, resolvedTokenResolver)
	if purgeServiceError != nil {
		return nil, purgeServiceError
	}

	return purgeService, nil
}
