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
	commandUseConstant                           = "branch-migrate"
	commandShortDescriptionConstant              = "Migrate repository defaults from main to master"
	commandLongDescriptionConstant               = "branch-migrate retargets workflows, updates GitHub configuration, and evaluates safety gates before switching the default branch."
	migrateCommandExecutionErrorTemplateConstant = "branch migration failed: %w"
	defaultRemoteNameConstant                    = "origin"
	sourceBranchFlagNameConstant                 = "from"
	sourceBranchFlagUsageConstant                = "Source branch to migrate from"
	targetBranchFlagNameConstant                 = "to"
	targetBranchFlagUsageConstant                = "Target branch to migrate to"
	identicalBranchesErrorMessageConstant        = "--from and --to must differ"
	workflowsDirectoryConstant                   = ".github/workflows"
	repositoryDiscoveryErrorTemplateConstant     = "repository discovery failed: %w"
	repositoryResolutionErrorTemplateConstant    = "unable to resolve repository identifier: %w"
	repositoryOwnerRepositoryFormatConstant      = "%s/%s"
	repositoryManagerCreationErrorTemplate       = "unable to construct repository manager: %w"
	githubClientCreationErrorTemplate            = "unable to construct GitHub client: %w"
	migrationCompletedMessageConstant            = "Branch migration completed"
	migratedWorkflowFilesFieldConstant           = "migrated_workflows"
	defaultBranchUpdatedFieldConstant            = "default_branch_updated"
	pagesConfigurationUpdatedFieldConstant       = "pages_configuration_updated"
	retargetedPullRequestsFieldConstant          = "retargeted_pull_requests"
	safeToDeleteFieldConstant                    = "safe_to_delete"
	safetyGatesBlockingMessageConstant           = "Branch deletion blocked by safety gates"
	safetyGateReasonsFieldConstant               = "blocking_reasons"
	logMessageRepositoryDiscoveryFailedConstant  = "Repository discovery failed"
	logMessageRepositoryMigrationFailedConstant  = "Repository migration failed"
	logFieldRepositoryRootsConstant              = "roots"
	logFieldRepositoryPathConstant               = "repository"
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
	sourceBranch        BranchName
	targetBranch        BranchName
}

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the branch-migrate Cobra command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	Executor                     CommandExecutor
	WorkingDirectory             string
	RepositoryDiscoverer         RepositoryDiscoverer
	ServiceProvider              ServiceProvider
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the branch-migrate command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:           commandUseConstant,
		Short:         commandShortDescriptionConstant,
		Long:          commandLongDescriptionConstant,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE:          builder.runMigrate,
	}

	command.Flags().String(sourceBranchFlagNameConstant, string(BranchMain), sourceBranchFlagUsageConstant)
	command.Flags().String(targetBranchFlagNameConstant, string(BranchMaster), targetBranchFlagUsageConstant)

	return command, nil
}

func (builder *CommandBuilder) runMigrate(command *cobra.Command, arguments []string) error {
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
			builder.logMigrationFailure(logger, normalizedRepositoryPath, failure)
			migrationErrors = append(migrationErrors, failure)
			continue
		}

		parsedRemote, parseError := gitrepo.ParseRemoteURL(remoteURL)
		if parseError != nil {
			failure := fmt.Errorf(repositoryResolutionErrorTemplateConstant, parseError)
			builder.logMigrationFailure(logger, normalizedRepositoryPath, failure)
			migrationErrors = append(migrationErrors, failure)
			continue
		}

		repositoryIdentifier := fmt.Sprintf(repositoryOwnerRepositoryFormatConstant, parsedRemote.Owner, parsedRemote.Repository)

		migrationOptions := MigrationOptions{
			RepositoryPath:       normalizedRepositoryPath,
			RepositoryRemoteName: defaultRemoteNameConstant,
			RepositoryIdentifier: repositoryIdentifier,
			WorkflowsDirectory:   workflowsDirectoryConstant,
			SourceBranch:         options.sourceBranch,
			TargetBranch:         options.targetBranch,
			PushUpdates:          true,
			EnableDebugLogging:   options.debugLoggingEnabled,
		}

		result, migrationError := service.Execute(command.Context(), migrationOptions)
		if migrationError != nil {
			if errors.Is(migrationError, context.Canceled) || errors.Is(migrationError, context.DeadlineExceeded) {
				return migrationError
			}
			wrappedError := fmt.Errorf(migrateCommandExecutionErrorTemplateConstant, migrationError)
			builder.logMigrationFailure(logger, normalizedRepositoryPath, wrappedError)
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

	sourceBranchName := strings.TrimSpace(configuration.SourceBranch)
	if len(sourceBranchName) == 0 {
		sourceBranchName = string(BranchMain)
	}
	targetBranchName := strings.TrimSpace(configuration.TargetBranch)
	if len(targetBranchName) == 0 {
		targetBranchName = string(BranchMaster)
	}

	if command != nil {
		if command.Flags().Changed(sourceBranchFlagNameConstant) {
			flagValue, _ := command.Flags().GetString(sourceBranchFlagNameConstant)
			sourceBranchName = strings.TrimSpace(flagValue)
		}
		if command.Flags().Changed(targetBranchFlagNameConstant) {
			flagValue, _ := command.Flags().GetString(targetBranchFlagNameConstant)
			targetBranchName = strings.TrimSpace(flagValue)
		}
	}

	if len(sourceBranchName) == 0 {
		sourceBranchName = string(BranchMain)
	}
	if len(targetBranchName) == 0 {
		targetBranchName = string(BranchMaster)
	}

	sourceBranch := BranchName(sourceBranchName)
	targetBranch := BranchName(targetBranchName)
	if sourceBranch == targetBranch {
		return commandOptions{}, errors.New(identicalBranchesErrorMessageConstant)
	}

	return commandOptions{
		debugLoggingEnabled: debugEnabled,
		repositoryRoots:     repositoryRoots,
		sourceBranch:        sourceBranch,
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

func (builder *CommandBuilder) logMigrationFailure(logger *zap.Logger, repositoryPath string, failure error) {
	if logger == nil {
		return
	}

	logger.Warn(
		logMessageRepositoryMigrationFailedConstant,
		zap.String(logFieldRepositoryPathConstant, repositoryPath),
		zap.Error(failure),
	)
}

func (builder *CommandBuilder) logSummary(logger *zap.Logger, repositoryPath string, result MigrationResult) {
	if logger == nil {
		return
	}

	logger.Info(
		migrationCompletedMessageConstant,
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
