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
	"github.com/temirov/git_scripts/internal/gitrepo"
	"github.com/temirov/git_scripts/internal/repos/dependencies"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	packagesPurgeCommandUseConstant                     = "repo-packages-purge"
	packagesPurgeCommandShortDescriptionConstant        = "Delete untagged GHCR versions"
	packagesPurgeCommandLongDescriptionConstant         = "repo-packages-purge removes untagged container versions from GitHub Container Registry."
	unexpectedArgumentsErrorMessageConstant             = "repo-packages-purge does not accept positional arguments"
	commandExecutionErrorTemplateConstant               = "repo-packages-purge failed: %w"
	packageFlagNameConstant                             = "package"
	packageFlagDescriptionConstant                      = "Container package name in GHCR"
	dryRunFlagNameConstant                              = "dry-run"
	dryRunFlagDescriptionConstant                       = "Preview deletions without modifying GHCR"
	repositoryRootsFlagNameConstant                     = "roots"
	repositoryRootsFlagDescriptionConstant              = "Directories that contain repositories for package purging"
	tokenSourceParseErrorTemplateConstant               = "invalid token source: %w"
	workingDirectoryResolutionErrorTemplateConstant     = "unable to determine working directory: %w"
	workingDirectoryEmptyErrorMessageConstant           = "working directory not provided"
	gitExecutorResolutionErrorTemplateConstant          = "unable to resolve git executor: %w"
	gitRepositoryManagerResolutionErrorTemplateConstant = "unable to resolve repository manager: %w"
	gitHubResolverResolutionErrorTemplateConstant       = "unable to resolve github metadata resolver: %w"
	originRemoteResolutionErrorTemplateConstant         = "unable to resolve origin remote: %w"
	originRemoteParseErrorTemplateConstant              = "unable to parse origin remote: %w"
	originRemoteOwnerMissingErrorMessageConstant        = "origin remote did not include owner information"
	repositoryMetadataResolutionErrorTemplateConstant   = "unable to resolve repository metadata: %w"
	repositoryMetadataOwnerMissingErrorMessageConstant  = "repository metadata did not include owner"
	repositoryContextResolutionErrorTemplateConstant    = "unable to resolve repository context: %w"
	repositoryDiscoveryErrorTemplateConstant            = "unable to discover repositories: %w"
	repositoryDiscoveryFailedMessageConstant            = "Failed to discover repositories"
	repositoryRootsLogFieldNameConstant                 = "repository_roots"
	repositoryPathLogFieldNameConstant                  = "repository_path"
	repositoryContextFailedMessageConstant              = "Failed to resolve repository context"
	repositoryPurgeFailedMessageConstant                = "repo-packages-purge failed for repository"
	repositoryPathEmptyErrorMessageConstant             = "repository path not provided"
	ownerRepoSeparatorConstant                          = "/"
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
	LoggerProvider           LoggerProvider
	ConfigurationProvider    ConfigurationProvider
	ServiceResolver          PurgeServiceResolver
	HTTPClient               ghcr.HTTPClient
	EnvironmentLookup        EnvironmentLookup
	FileReader               FileReader
	TokenResolver            TokenResolver
	GitExecutor              shared.GitExecutor
	RepositoryManager        shared.GitRepositoryManager
	GitHubResolver           shared.GitHubMetadataResolver
	WorkingDirectoryResolver WorkingDirectoryResolver
	RepositoryDiscoverer     shared.RepositoryDiscoverer
}

// WorkingDirectoryResolver resolves the directory containing the active repository.
type WorkingDirectoryResolver func() (string, error)

type repositoryContext struct {
	Owner     string
	OwnerType ghcr.OwnerType
}

type commandExecutionOptions struct {
	PackageName     string
	DryRun          bool
	TokenSource     TokenSourceConfiguration
	RepositoryRoots []string
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

	repositoryManager, githubResolver, dependenciesError := builder.resolveRepositoryDependencies(logger)
	if dependenciesError != nil {
		return dependenciesError
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

		repositoryContext, contextError := builder.resolveRepositoryContext(command.Context(), repositoryManager, githubResolver, normalizedRepositoryPath)
		if contextError != nil {
			if errors.Is(contextError, context.Canceled) || errors.Is(contextError, context.DeadlineExceeded) {
				return contextError
			}

			logger.Error(
				repositoryContextFailedMessageConstant,
				zap.String(repositoryPathLogFieldNameConstant, normalizedRepositoryPath),
				zap.Error(contextError),
			)

			wrappedContextError := fmt.Errorf(repositoryContextResolutionErrorTemplateConstant, contextError)
			executionErrors = append(executionErrors, wrappedContextError)
			continue
		}

		purgeOptions := PurgeOptions{
			Owner:       repositoryContext.Owner,
			PackageName: executionOptions.PackageName,
			OwnerType:   repositoryContext.OwnerType,
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
	packageValue := selectStringValue(packageFlagValue, configuration.Purge.PackageName)

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
		PackageName:     packageValue,
		DryRun:          dryRunValue,
		TokenSource:     parsedTokenSource,
		RepositoryRoots: repositoryRoots,
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

	return configuration.sanitize()
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

func selectStringValue(flagValue string, configurationValue string) string {
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

		workingDirectory, workingDirectoryError := builder.resolveWorkingDirectory()
		if workingDirectoryError != nil {
			return nil, workingDirectoryError
		}

		return []string{workingDirectory}, nil
	}

	if len(configuration.Purge.RepositoryRoots) > 0 {
		rootsCopy := make([]string, len(configuration.Purge.RepositoryRoots))
		copy(rootsCopy, configuration.Purge.RepositoryRoots)
		return rootsCopy, nil
	}

	workingDirectory, workingDirectoryError := builder.resolveWorkingDirectory()
	if workingDirectoryError != nil {
		return nil, workingDirectoryError
	}

	return []string{workingDirectory}, nil
}

func (builder *CommandBuilder) resolveRepositoryContext(
	executionContext context.Context,
	repositoryManager shared.GitRepositoryManager,
	githubResolver shared.GitHubMetadataResolver,
	repositoryPath string,
) (repositoryContext, error) {
	trimmedRepositoryPath := strings.TrimSpace(repositoryPath)
	if len(trimmedRepositoryPath) == 0 {
		return repositoryContext{}, errors.New(repositoryPathEmptyErrorMessageConstant)
	}

	originURL, originError := repositoryManager.GetRemoteURL(executionContext, trimmedRepositoryPath, shared.OriginRemoteNameConstant)
	if originError != nil {
		return repositoryContext{}, fmt.Errorf(originRemoteResolutionErrorTemplateConstant, originError)
	}

	parsedRemote, parseError := gitrepo.ParseRemoteURL(originURL)
	if parseError != nil {
		return repositoryContext{}, fmt.Errorf(originRemoteParseErrorTemplateConstant, parseError)
	}

	ownerCandidate := strings.TrimSpace(parsedRemote.Owner)
	if len(ownerCandidate) == 0 {
		return repositoryContext{}, fmt.Errorf(originRemoteParseErrorTemplateConstant, errors.New(originRemoteOwnerMissingErrorMessageConstant))
	}

	repositoryIdentifier := fmt.Sprintf("%s/%s", parsedRemote.Owner, parsedRemote.Repository)
	metadata, metadataError := githubResolver.ResolveRepoMetadata(executionContext, repositoryIdentifier)
	if metadataError != nil {
		return repositoryContext{}, fmt.Errorf(repositoryMetadataResolutionErrorTemplateConstant, metadataError)
	}

	resolvedOwner := ownerCandidate
	trimmedNameWithOwner := strings.TrimSpace(metadata.NameWithOwner)
	if len(trimmedNameWithOwner) > 0 {
		ownerFromMetadata, ownerParseError := parseOwnerFromNameWithOwner(trimmedNameWithOwner)
		if ownerParseError != nil {
			return repositoryContext{}, fmt.Errorf(repositoryMetadataResolutionErrorTemplateConstant, ownerParseError)
		}
		resolvedOwner = ownerFromMetadata
	}

	ownerType := ghcr.UserOwnerType
	if metadata.IsInOrganization {
		ownerType = ghcr.OrganizationOwnerType
	}

	return repositoryContext{Owner: resolvedOwner, OwnerType: ownerType}, nil
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

func parseOwnerFromNameWithOwner(nameWithOwner string) (string, error) {
	components := strings.Split(nameWithOwner, ownerRepoSeparatorConstant)
	if len(components) < 2 {
		return "", errors.New(repositoryMetadataOwnerMissingErrorMessageConstant)
	}

	owner := strings.TrimSpace(components[0])
	if len(owner) == 0 {
		return "", errors.New(repositoryMetadataOwnerMissingErrorMessageConstant)
	}

	return owner, nil
}
