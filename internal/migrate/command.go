package migrate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
	"github.com/temirov/gix/internal/repos/discovery"
	"github.com/temirov/gix/internal/utils"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	commandUseConstant                                = "branch-default"
	commandShortDescriptionConstant                   = "Set the repository default branch"
	commandLongDescriptionConstant                    = "branch-default retargets workflows, updates GitHub configuration, and evaluates safety gates before promoting the requested branch, automatically detecting the current default branch."
	defaultCommandExecutionErrorTemplateConstant      = "default branch update failed: %w"
	defaultRemoteNameConstant                         = "origin"
	targetBranchFlagNameConstant                      = "to"
	targetBranchFlagUsageConstant                     = "Target branch to promote to default"
	workflowsDirectoryConstant                        = ".github/workflows"
	repositoryDiscoveryErrorTemplateConstant          = "repository discovery failed: %w"
	repositoryResolutionErrorTemplateConstant         = "unable to resolve repository identifier: %w"
	repositoryMetadataResolutionErrorTemplateConstant = "unable to resolve repository metadata: %w"
	repositoryMetadataDefaultBranchMissingMessage     = "repository metadata missing default branch"
	repositoryOwnerRepositoryFormatConstant           = "%s/%s"
	repositoryManagerCreationErrorTemplate            = "unable to construct repository manager: %w"
	githubClientCreationErrorTemplate                 = "unable to construct GitHub client: %w"
	defaultBranchUpdatedMessageConstant               = "Default branch update completed"
	migratedWorkflowFilesFieldConstant                = "migrated_workflows"
	defaultBranchUpdatedFieldConstant                 = "default_branch_updated"
	pagesConfigurationUpdatedFieldConstant            = "pages_configuration_updated"
	retargetedPullRequestsFieldConstant               = "retargeted_pull_requests"
	safeToDeleteFieldConstant                         = "safe_to_delete"
	safetyGatesBlockingMessageConstant                = "Branch deletion blocked by safety gates"
	safetyGateReasonsFieldConstant                    = "blocking_reasons"
	logMessageRepositoryDiscoveryFailedConstant       = "Repository discovery failed"
	logMessageRepositoryDefaultUpdateFailedConstant   = "Default branch update failed"
	defaultBranchAlreadyMatchesMessageConstant        = "Default branch already matches target"
	logFieldRepositoryRootsConstant                   = "roots"
	logFieldRepositoryPathConstant                    = "repository"
	logFieldTargetBranchConstant                      = "target_branch"
)

// RepositoryDiscoverer locates Git repositories beneath provided roots.
type RepositoryDiscoverer interface {
	DiscoverRepositories(roots []string) ([]string, error)
}

// ServiceProvider constructs a migration executor from dependencies.
type ServiceProvider func(dependencies ServiceDependencies) (MigrationExecutor, error)

type commandOptions struct {
	debugLoggingEnabled bool
	repositoryRoots     []string
	targetBranch        BranchName
}

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the branch-default Cobra command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	Executor                     CommandExecutor
	WorkingDirectory             string
	RepositoryDiscoverer         RepositoryDiscoverer
	ServiceProvider              ServiceProvider
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the branch-default command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:           commandUseConstant,
		Short:         commandShortDescriptionConstant,
		Long:          commandLongDescriptionConstant,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE:          builder.runDefault,
	}

	command.Flags().String(targetBranchFlagNameConstant, string(BranchMaster), targetBranchFlagUsageConstant)

	return command, nil
}

func (builder *CommandBuilder) runDefault(command *cobra.Command, arguments []string) error {
	options, optionsError := builder.parseOptions(command, arguments)
	if optionsError != nil {
		return optionsError
	}

	logger := builder.resolveLogger(options.debugLoggingEnabled)

	executor, executorError := builder.resolveExecutor(logger)
	if executorError != nil {
		return executorError
	}

	repositoryManager, managerError := gitrepo.NewRepositoryManager(executor)
	if managerError != nil {
		return fmt.Errorf(repositoryManagerCreationErrorTemplate, managerError)
	}

	githubClient, githubClientError := githubcli.NewClient(executor)
	if githubClientError != nil {
		return fmt.Errorf(githubClientCreationErrorTemplate, githubClientError)
	}

	service, serviceError := builder.resolveService(ServiceDependencies{
		Logger:            logger,
		RepositoryManager: repositoryManager,
		GitHubClient:      githubClient,
		GitExecutor:       executor,
	})
	if serviceError != nil {
		return serviceError
	}

	repositoryDiscoverer := builder.resolveRepositoryDiscoverer()
	repositories, discoveryError := repositoryDiscoverer.DiscoverRepositories(options.repositoryRoots)
	if discoveryError != nil {
		logger.Error(
			logMessageRepositoryDiscoveryFailedConstant,
			zap.Strings(logFieldRepositoryRootsConstant, options.repositoryRoots),
			zap.Error(discoveryError),
		)
		return fmt.Errorf(repositoryDiscoveryErrorTemplateConstant, discoveryError)
	}

	var migrationErrors []error

	for _, repositoryPath := range repositories {
		normalizedRepositoryPath := filepath.Clean(repositoryPath)

		remoteURL, remoteError := repositoryManager.GetRemoteURL(command.Context(), normalizedRepositoryPath, defaultRemoteNameConstant)
		if remoteError != nil {
			if errors.Is(remoteError, context.Canceled) || errors.Is(remoteError, context.DeadlineExceeded) {
				return remoteError
			}
			failure := fmt.Errorf(repositoryResolutionErrorTemplateConstant, remoteError)
			builder.logDefaultBranchFailure(logger, normalizedRepositoryPath, failure)
			migrationErrors = append(migrationErrors, failure)
			continue
		}

		parsedRemote, parseError := gitrepo.ParseRemoteURL(remoteURL)
		if parseError != nil {
			failure := fmt.Errorf(repositoryResolutionErrorTemplateConstant, parseError)
			builder.logDefaultBranchFailure(logger, normalizedRepositoryPath, failure)
			migrationErrors = append(migrationErrors, failure)
			continue
		}

		repositoryIdentifier := fmt.Sprintf(repositoryOwnerRepositoryFormatConstant, parsedRemote.Owner, parsedRemote.Repository)

		metadata, metadataError := githubClient.ResolveRepoMetadata(command.Context(), repositoryIdentifier)
		if metadataError != nil {
			if errors.Is(metadataError, context.Canceled) || errors.Is(metadataError, context.DeadlineExceeded) {
				return metadataError
			}
			failure := fmt.Errorf(repositoryMetadataResolutionErrorTemplateConstant, metadataError)
			builder.logDefaultBranchFailure(logger, normalizedRepositoryPath, failure)
			migrationErrors = append(migrationErrors, failure)
			continue
		}

		sourceBranchName := strings.TrimSpace(metadata.DefaultBranch)
		if len(sourceBranchName) == 0 {
			failure := fmt.Errorf("%s: %s", repositoryMetadataDefaultBranchMissingMessage, repositoryIdentifier)
			builder.logDefaultBranchFailure(logger, normalizedRepositoryPath, failure)
			migrationErrors = append(migrationErrors, failure)
			continue
		}

		sourceBranch := BranchName(sourceBranchName)
		if sourceBranch == options.targetBranch {
			if logger != nil {
				logger.Info(
					defaultBranchAlreadyMatchesMessageConstant,
					zap.String(logFieldRepositoryPathConstant, normalizedRepositoryPath),
					zap.String(logFieldTargetBranchConstant, string(options.targetBranch)),
				)
			}
			continue
		}

		migrationOptions := MigrationOptions{
			RepositoryPath:       normalizedRepositoryPath,
			RepositoryRemoteName: defaultRemoteNameConstant,
			RepositoryIdentifier: repositoryIdentifier,
			WorkflowsDirectory:   workflowsDirectoryConstant,
			SourceBranch:         sourceBranch,
			TargetBranch:         options.targetBranch,
			PushUpdates:          true,
			EnableDebugLogging:   options.debugLoggingEnabled,
		}

		result, migrationError := service.Execute(command.Context(), migrationOptions)
		if migrationError != nil {
			if errors.Is(migrationError, context.Canceled) || errors.Is(migrationError, context.DeadlineExceeded) {
				return migrationError
			}
			wrappedError := fmt.Errorf(defaultCommandExecutionErrorTemplateConstant, migrationError)
			builder.logDefaultBranchFailure(logger, normalizedRepositoryPath, wrappedError)
			migrationErrors = append(migrationErrors, wrappedError)
			continue
		}

		builder.logSummary(logger, normalizedRepositoryPath, result)
	}

	if len(migrationErrors) > 0 {
		return errors.Join(migrationErrors...)
	}

	return nil
}

func (builder *CommandBuilder) parseOptions(command *cobra.Command, arguments []string) (commandOptions, error) {
	configuration := builder.resolveConfiguration()

	debugEnabled := configuration.EnableDebugLogging
	if command != nil {
		contextAccessor := utils.NewCommandContextAccessor()
		if logLevel, available := contextAccessor.LogLevel(command.Context()); available {
			if strings.EqualFold(logLevel, string(utils.LogLevelDebug)) {
				debugEnabled = true
			}
		}
	}

	repositoryRoots, resolveRootsError := rootutils.Resolve(command, arguments, configuration.RepositoryRoots)
	if resolveRootsError != nil {
		return commandOptions{}, resolveRootsError
	}

	targetBranchName := strings.TrimSpace(configuration.TargetBranch)
	if len(targetBranchName) == 0 {
		targetBranchName = string(BranchMaster)
	}

	if command != nil {
		if command.Flags().Changed(targetBranchFlagNameConstant) {
			flagValue, _ := command.Flags().GetString(targetBranchFlagNameConstant)
			targetBranchName = strings.TrimSpace(flagValue)
		}
	}

	if len(targetBranchName) == 0 {
		targetBranchName = string(BranchMaster)
	}

	targetBranch := BranchName(targetBranchName)

	return commandOptions{
		debugLoggingEnabled: debugEnabled,
		repositoryRoots:     repositoryRoots,
		targetBranch:        targetBranch,
	}, nil
}

func (builder *CommandBuilder) resolveLogger(enableDebug bool) *zap.Logger {
	var logger *zap.Logger
	if builder.LoggerProvider != nil {
		logger = builder.LoggerProvider()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if enableDebug {
		logger = logger.WithOptions(zap.IncreaseLevel(zapcore.DebugLevel))
	}
	return logger
}

func (builder *CommandBuilder) resolveExecutor(logger *zap.Logger) (CommandExecutor, error) {
	if builder.Executor != nil {
		return builder.Executor, nil
	}

	commandRunner := execshell.NewOSCommandRunner()
	humanReadableLogging := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadableLogging = builder.HumanReadableLoggingProvider()
	}
	shellExecutor, creationError := execshell.NewShellExecutor(logger, commandRunner, humanReadableLogging)
	if creationError != nil {
		return nil, creationError
	}
	return shellExecutor, nil
}

func (builder *CommandBuilder) resolveRepositoryDiscoverer() RepositoryDiscoverer {
	if builder.RepositoryDiscoverer != nil {
		return builder.RepositoryDiscoverer
	}
	return discovery.NewFilesystemRepositoryDiscoverer()
}

func (builder *CommandBuilder) resolveService(dependencies ServiceDependencies) (MigrationExecutor, error) {
	if builder.ServiceProvider != nil {
		return builder.ServiceProvider(dependencies)
	}
	return NewService(dependencies)
}

func (builder *CommandBuilder) resolveConfiguration() CommandConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultCommandConfiguration()
	}

	provided := builder.ConfigurationProvider()
	return provided.Sanitize()
}

func (builder *CommandBuilder) logDefaultBranchFailure(logger *zap.Logger, repositoryPath string, failure error) {
	if logger == nil {
		return
	}

	logger.Warn(
		logMessageRepositoryDefaultUpdateFailedConstant,
		zap.String(logFieldRepositoryPathConstant, repositoryPath),
		zap.Error(failure),
	)
}

func (builder *CommandBuilder) logSummary(logger *zap.Logger, repositoryPath string, result MigrationResult) {
	if logger == nil {
		return
	}

	logger.Info(
		defaultBranchUpdatedMessageConstant,
		zap.String(logFieldRepositoryPathConstant, repositoryPath),
		zap.Strings(migratedWorkflowFilesFieldConstant, result.WorkflowOutcome.UpdatedFiles),
		zap.Bool(defaultBranchUpdatedFieldConstant, result.DefaultBranchUpdated),
		zap.Bool(pagesConfigurationUpdatedFieldConstant, result.PagesConfigurationUpdated),
		zap.Ints(retargetedPullRequestsFieldConstant, result.RetargetedPullRequests),
		zap.Bool(safeToDeleteFieldConstant, result.SafetyStatus.SafeToDelete),
	)

	if !result.SafetyStatus.SafeToDelete {
		logger.Warn(
			safetyGatesBlockingMessageConstant,
			zap.String(logFieldRepositoryPathConstant, repositoryPath),
			zap.Strings(safetyGateReasonsFieldConstant, result.SafetyStatus.BlockingReasons),
		)
	}
}
