package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
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
}

// WorkingDirectoryResolver resolves the directory containing the active repository.
type WorkingDirectoryResolver func() (string, error)

type repositoryContext struct {
	Owner     string
	OwnerType ghcr.OwnerType
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

	return purgeCommand, nil
}

func (builder *CommandBuilder) runPurge(command *cobra.Command, arguments []string) error {
	if len(arguments) > 0 {
		return errors.New(unexpectedArgumentsErrorMessageConstant)
	}

	logger := builder.resolveLogger()

	purgeOptions, optionsError := builder.parsePurgeOptions(command, logger)
	if optionsError != nil {
		return optionsError
	}

	purgeService, serviceError := builder.resolvePurgeService(logger)
	if serviceError != nil {
		return serviceError
	}

	_, executionError := purgeService.Execute(command.Context(), purgeOptions)
	if executionError != nil {
		return fmt.Errorf(commandExecutionErrorTemplateConstant, executionError)
	}

	return nil
}

func (builder *CommandBuilder) parsePurgeOptions(command *cobra.Command, logger *zap.Logger) (PurgeOptions, error) {
	configuration := builder.resolveConfiguration()

	packageFlagValue, packageFlagError := command.Flags().GetString(packageFlagNameConstant)
	if packageFlagError != nil {
		return PurgeOptions{}, packageFlagError
	}
	packageValue := selectStringValue(packageFlagValue, configuration.Purge.PackageName)

	repositoryContext, contextError := builder.resolveRepositoryContext(command.Context(), logger)
	if contextError != nil {
		return PurgeOptions{}, fmt.Errorf(repositoryContextResolutionErrorTemplateConstant, contextError)
	}

	parsedTokenSource, tokenParseError := ParseTokenSource(defaultTokenSourceValueConstant)
	if tokenParseError != nil {
		return PurgeOptions{}, fmt.Errorf(tokenSourceParseErrorTemplateConstant, tokenParseError)
	}

	dryRunValue := configuration.Purge.DryRun
	if command.Flags().Changed(dryRunFlagNameConstant) {
		flagDryRunValue, dryRunFlagError := command.Flags().GetBool(dryRunFlagNameConstant)
		if dryRunFlagError != nil {
			return PurgeOptions{}, dryRunFlagError
		}
		dryRunValue = flagDryRunValue
	}

	purgeOptions := PurgeOptions{
		Owner:       repositoryContext.Owner,
		PackageName: packageValue,
		OwnerType:   repositoryContext.OwnerType,
		TokenSource: parsedTokenSource,
		DryRun:      dryRunValue,
	}

	return purgeOptions, nil
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

	return configuration
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

func (builder *CommandBuilder) resolveRepositoryContext(executionContext context.Context, logger *zap.Logger) (repositoryContext, error) {
	workingDirectory, workingDirectoryError := builder.resolveWorkingDirectory()
	if workingDirectoryError != nil {
		return repositoryContext{}, workingDirectoryError
	}

	repositoryManager, githubResolver, resolutionError := builder.resolveRepositoryDependencies(logger)
	if resolutionError != nil {
		return repositoryContext{}, resolutionError
	}

	originURL, originError := repositoryManager.GetRemoteURL(executionContext, workingDirectory, shared.OriginRemoteNameConstant)
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
