package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/ghcr"
	"github.com/temirov/git_scripts/internal/repos/dependencies"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	packagesPurgeCommandUseConstant                           = "repo-packages-purge"
	packagesPurgeCommandShortDescriptionConstant              = "Delete untagged GHCR versions"
	packagesPurgeCommandLongDescriptionConstant               = "repo-packages-purge removes untagged container versions from GitHub Container Registry."
	unexpectedArgumentsErrorMessageConstant                   = "repo-packages-purge does not accept positional arguments"
	commandExecutionErrorTemplateConstant                     = "repo-packages-purge failed: %w"
	packageFlagNameConstant                                   = "package"
	packageFlagDescriptionConstant                            = "Container package name in GHCR"
	dryRunFlagNameConstant                                    = "dry-run"
	dryRunFlagDescriptionConstant                             = "Preview deletions without modifying GHCR"
	repositoryRootsFlagNameConstant                           = "roots"
	repositoryRootsFlagDescriptionConstant                    = "Directories that contain repositories for package purging"
	missingRepositoryRootsErrorMessageConstant                = "no repository roots provided; specify --root or configure defaults"
	tokenSourceParseErrorTemplateConstant                     = "invalid token source: %w"
	workingDirectoryResolutionErrorTemplateConstant           = "unable to determine working directory: %w"
	workingDirectoryEmptyErrorMessageConstant                 = "working directory not provided"
	gitExecutorResolutionErrorTemplateConstant                = "unable to resolve git executor: %w"
	gitRepositoryManagerResolutionErrorTemplateConstant       = "unable to resolve repository manager: %w"
	gitHubResolverResolutionErrorTemplateConstant             = "unable to resolve github metadata resolver: %w"
	repositoryMetadataResolverResolutionErrorTemplateConstant = "unable to resolve repository metadata resolver: %w"
	repositoryDiscoveryErrorTemplateConstant                  = "unable to discover repositories: %w"
	repositoryDiscoveryFailedMessageConstant                  = "Failed to discover repositories"
	repositoryRootsLogFieldNameConstant                       = "repository_roots"
	repositoryPathLogFieldNameConstant                        = "repository_path"
	repositoryMetadataFailedMessageConstant                   = "Failed to resolve repository metadata"
	repositoryPurgeFailedMessageConstant                      = "repo-packages-purge failed for repository"
	ownerRepoSeparatorConstant                                = "/"
)

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// ConfigurationProvider returns the current packages configuration.
type ConfigurationProvider func() Configuration

// PurgeServiceResolver creates purge executors for the command.
type PurgeServiceResolver interface {
	Resolve(logger *zap.Logger) (PurgeExecutor, error)
}

// CommandBuilder assembles the repo-packages-purge command.
type CommandBuilder struct {
	LoggerProvider             LoggerProvider
	ConfigurationProvider      ConfigurationProvider
	ServiceResolver            PurgeServiceResolver
	HTTPClient                 ghcr.HTTPClient
	EnvironmentLookup          EnvironmentLookup
	FileReader                 FileReader
	TokenResolver              TokenResolver
	GitExecutor                shared.GitExecutor
	RepositoryManager          shared.GitRepositoryManager
	GitHubResolver             shared.GitHubMetadataResolver
	RepositoryMetadataResolver RepositoryMetadataResolver
	WorkingDirectoryResolver   WorkingDirectoryResolver
	RepositoryDiscoverer       shared.RepositoryDiscoverer
}

// WorkingDirectoryResolver resolves the directory containing the active repository.
type WorkingDirectoryResolver func() (string, error)

type commandExecutionOptions struct {
	PackageNameOverride string
	DryRun              bool
	TokenSource         TokenSourceConfiguration
	RepositoryRoots     []string
}

// Build constructs the repo-packages-purge command with purge functionality.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	purgeCommand := &cobra.Command{
		Use:   packagesPurgeCommandUseConstant,
		Short: packagesPurgeCommandShortDescriptionConstant,
		Long:  packagesPurgeCommandLongDescriptionConstant,
		RunE:  builder.runPurge,
	}

	purgeCommand.Flags().String(packageFlagNameConstant, "", packageFlagDescriptionConstant)
	purgeCommand.Flags().Bool(dryRunFlagNameConstant, false, dryRunFlagDescriptionConstant)
	purgeCommand.Flags().StringSlice(repositoryRootsFlagNameConstant, nil, repositoryRootsFlagDescriptionConstant)

	return purgeCommand, nil
}

func (builder *CommandBuilder) runPurge(command *cobra.Command, arguments []string) error {
	if len(arguments) > 0 {
		return errors.New(unexpectedArgumentsErrorMessageConstant)
	}

	logger := builder.resolveLogger()

	executionOptions, optionsError := builder.parseCommandOptions(command)
	if optionsError != nil {
		return optionsError
	}

	purgeService, serviceError := builder.resolvePurgeService(logger)
	if serviceError != nil {
		return serviceError
	}

	repositoryMetadataResolver, metadataResolverError := builder.resolveRepositoryMetadataResolver(logger)
	if metadataResolverError != nil {
		return metadataResolverError
	}

	repositoryDiscoverer := dependencies.ResolveRepositoryDiscoverer(builder.RepositoryDiscoverer)
	repositoryPaths, discoveryError := repositoryDiscoverer.DiscoverRepositories(executionOptions.RepositoryRoots)
	if discoveryError != nil {
		logger.Error(
			repositoryDiscoveryFailedMessageConstant,
			zap.Strings(repositoryRootsLogFieldNameConstant, executionOptions.RepositoryRoots),
			zap.Error(discoveryError),
		)
		return fmt.Errorf(repositoryDiscoveryErrorTemplateConstant, discoveryError)
	}

	var executionErrors []error

	for _, repositoryPath := range repositoryPaths {
		normalizedRepositoryPath := filepath.Clean(repositoryPath)

		repositoryMetadata, metadataError := repositoryMetadataResolver.ResolveMetadata(command.Context(), normalizedRepositoryPath)
		if metadataError != nil {
			if errors.Is(metadataError, context.Canceled) || errors.Is(metadataError, context.DeadlineExceeded) {
				return metadataError
			}

			logger.Error(
				repositoryMetadataFailedMessageConstant,
				zap.String(repositoryPathLogFieldNameConstant, normalizedRepositoryPath),
				zap.Error(metadataError),
			)

			wrappedMetadataError := fmt.Errorf(repositoryMetadataResolutionErrorTemplateConstant, metadataError)
			executionErrors = append(executionErrors, wrappedMetadataError)
			continue
		}

		packageName := strings.TrimSpace(executionOptions.PackageNameOverride)
		if len(packageName) == 0 {
			packageName = repositoryMetadata.DefaultPackageName
		}

		purgeOptions := PurgeOptions{
			Owner:       repositoryMetadata.Owner,
			PackageName: packageName,
			OwnerType:   repositoryMetadata.OwnerType,
			TokenSource: executionOptions.TokenSource,
			DryRun:      executionOptions.DryRun,
		}

		_, executionError := purgeService.Execute(command.Context(), purgeOptions)
		if executionError != nil {
			if errors.Is(executionError, context.Canceled) || errors.Is(executionError, context.DeadlineExceeded) {
				return executionError
			}

			logger.Error(
				repositoryPurgeFailedMessageConstant,
				zap.String(repositoryPathLogFieldNameConstant, normalizedRepositoryPath),
				zap.Error(executionError),
			)

			wrappedExecutionError := fmt.Errorf(commandExecutionErrorTemplateConstant, executionError)
			executionErrors = append(executionErrors, wrappedExecutionError)
			continue
		}
	}

	if len(executionErrors) > 0 {
		return errors.Join(executionErrors...)
	}

	return nil
}

func (builder *CommandBuilder) parseCommandOptions(command *cobra.Command) (commandExecutionOptions, error) {
	configuration := builder.resolveConfiguration()

	packageFlagValue, packageFlagError := command.Flags().GetString(packageFlagNameConstant)
	if packageFlagError != nil {
		return commandExecutionOptions{}, packageFlagError
	}
	packageValue := selectOptionalStringValue(packageFlagValue, configuration.Purge.PackageName)

	parsedTokenSource, tokenParseError := ParseTokenSource(defaultTokenSourceValueConstant)
	if tokenParseError != nil {
		return commandExecutionOptions{}, fmt.Errorf(tokenSourceParseErrorTemplateConstant, tokenParseError)
	}

	dryRunValue := configuration.Purge.DryRun
	if command.Flags().Changed(dryRunFlagNameConstant) {
		flagDryRunValue, dryRunFlagError := command.Flags().GetBool(dryRunFlagNameConstant)
		if dryRunFlagError != nil {
			return commandExecutionOptions{}, dryRunFlagError
		}
		dryRunValue = flagDryRunValue
	}

	repositoryRoots, rootsError := builder.determineRepositoryRoots(command, configuration)
	if rootsError != nil {
		return commandExecutionOptions{}, rootsError
	}

	executionOptions := commandExecutionOptions{
		PackageNameOverride: packageValue,
		DryRun:              dryRunValue,
		TokenSource:         parsedTokenSource,
		RepositoryRoots:     repositoryRoots,
	}

	return executionOptions, nil
}

func (builder *CommandBuilder) resolveLogger() *zap.Logger {
	if builder.LoggerProvider == nil {
		return zap.NewNop()
	}

	logger := builder.LoggerProvider()
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

func (builder *CommandBuilder) resolveConfiguration() Configuration {
	configuration := DefaultConfiguration()
	if builder.ConfigurationProvider != nil {
		configuration = builder.ConfigurationProvider()
	}

	return configuration.Sanitize()
}

func (builder *CommandBuilder) resolvePurgeService(logger *zap.Logger) (PurgeExecutor, error) {
	if builder.ServiceResolver != nil {
		return builder.ServiceResolver.Resolve(logger)
	}

	defaultResolver := &DefaultPurgeServiceResolver{
		HTTPClient:        builder.HTTPClient,
		EnvironmentLookup: builder.EnvironmentLookup,
		FileReader:        builder.FileReader,
		TokenResolver:     builder.TokenResolver,
	}

	return defaultResolver.Resolve(logger)
}

func selectOptionalStringValue(flagValue string, configurationValue string) string {
	trimmedFlagValue := strings.TrimSpace(flagValue)
	if len(trimmedFlagValue) > 0 {
		return trimmedFlagValue
	}

	return strings.TrimSpace(configurationValue)
}

func (builder *CommandBuilder) determineRepositoryRoots(command *cobra.Command, configuration Configuration) ([]string, error) {
	if command != nil && command.Flags().Changed(repositoryRootsFlagNameConstant) {
		flagRoots, flagError := command.Flags().GetStringSlice(repositoryRootsFlagNameConstant)
		if flagError != nil {
			return nil, flagError
		}

		sanitizedFlagRoots := sanitizeRoots(flagRoots)
		if len(sanitizedFlagRoots) > 0 {
			return sanitizedFlagRoots, nil
		}
		if command != nil {
			_ = command.Help()
		}
		return nil, errors.New(missingRepositoryRootsErrorMessageConstant)
	}

	if len(configuration.Purge.RepositoryRoots) > 0 {
		rootsCopy := make([]string, len(configuration.Purge.RepositoryRoots))
		copy(rootsCopy, configuration.Purge.RepositoryRoots)
		return rootsCopy, nil
	}

	if command != nil {
		_ = command.Help()
	}

	return nil, errors.New(missingRepositoryRootsErrorMessageConstant)
}

func (builder *CommandBuilder) resolveRepositoryMetadataResolver(logger *zap.Logger) (RepositoryMetadataResolver, error) {
	if builder.RepositoryMetadataResolver != nil {
		return builder.RepositoryMetadataResolver, nil
	}

	repositoryManager, githubResolver, dependenciesError := builder.resolveRepositoryDependencies(logger)
	if dependenciesError != nil {
		return nil, fmt.Errorf(repositoryMetadataResolverResolutionErrorTemplateConstant, dependenciesError)
	}

	return &DefaultRepositoryMetadataResolver{
		RepositoryManager: repositoryManager,
		GitHubResolver:    githubResolver,
	}, nil
}

func (builder *CommandBuilder) resolveWorkingDirectory() (string, error) {
	if builder.WorkingDirectoryResolver != nil {
		directory, resolutionError := builder.WorkingDirectoryResolver()
		if resolutionError != nil {
			return "", fmt.Errorf(workingDirectoryResolutionErrorTemplateConstant, resolutionError)
		}
		trimmedDirectory := strings.TrimSpace(directory)
		if len(trimmedDirectory) == 0 {
			return "", errors.New(workingDirectoryEmptyErrorMessageConstant)
		}
		return trimmedDirectory, nil
	}

	directory, resolutionError := os.Getwd()
	if resolutionError != nil {
		return "", fmt.Errorf(workingDirectoryResolutionErrorTemplateConstant, resolutionError)
	}
	trimmedDirectory := strings.TrimSpace(directory)
	if len(trimmedDirectory) == 0 {
		return "", errors.New(workingDirectoryEmptyErrorMessageConstant)
	}
	return trimmedDirectory, nil
}

func (builder *CommandBuilder) resolveRepositoryDependencies(logger *zap.Logger) (shared.GitRepositoryManager, shared.GitHubMetadataResolver, error) {
	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, false)
	if executorError != nil {
		return nil, nil, fmt.Errorf(gitExecutorResolutionErrorTemplateConstant, executorError)
	}

	repositoryManager, managerError := dependencies.ResolveGitRepositoryManager(builder.RepositoryManager, gitExecutor)
	if managerError != nil {
		return nil, nil, fmt.Errorf(gitRepositoryManagerResolutionErrorTemplateConstant, managerError)
	}

	githubResolver, resolverError := dependencies.ResolveGitHubResolver(builder.GitHubResolver, gitExecutor)
	if resolverError != nil {
		return nil, nil, fmt.Errorf(gitHubResolverResolutionErrorTemplateConstant, resolverError)
	}

	return repositoryManager, githubResolver, nil
}
