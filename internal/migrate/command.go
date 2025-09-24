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

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
	"github.com/temirov/git_scripts/internal/repos/discovery"
)

const (
	branchCommandUseConstant                     = "branch"
	branchCommandShortDescriptionConstant        = "Manage repository branches"
	branchCommandLongDescriptionConstant         = "Branch utilities for Git repositories."
	migrateCommandUseConstant                    = "migrate [root ...]"
	migrateCommandShortDescriptionConstant       = "Migrate repository defaults from main to master"
	migrateCommandLongDescriptionConstant        = "branch migrate retargets workflows, updates GitHub configuration, and evaluates safety gates before switching the default branch."
	migrateCommandExecutionErrorTemplateConstant = "branch migration failed: %w"
	debugFlagNameConstant                        = "debug"
	debugFlagDescriptionConstant                 = "Enable verbose debug logging for migration diagnostics"
	defaultRemoteNameConstant                    = "origin"
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
	defaultRepositoryRootConstant                = "."
)

// RepositoryDiscoverer locates Git repositories beneath provided roots.
type RepositoryDiscoverer interface {
	DiscoverRepositories(roots []string) ([]string, error)
}

// ServiceProvider constructs a migration executor from dependencies.
type ServiceProvider func(dependencies ServiceDependencies) (MigrationExecutor, error)

type commandOptions struct {
	enableDebug     bool
	repositoryRoots []string
}

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the Cobra command hierarchy for branch migration.
type CommandBuilder struct {
	LoggerProvider       LoggerProvider
	Executor             CommandExecutor
	WorkingDirectory     string
	RepositoryDiscoverer RepositoryDiscoverer
	ServiceProvider      ServiceProvider
}

// Build constructs the branch command with the migrate subcommand.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	branchCommand := &cobra.Command{
		Use:   branchCommandUseConstant,
		Short: branchCommandShortDescriptionConstant,
		Long:  branchCommandLongDescriptionConstant,
	}

	migrateCommand := &cobra.Command{
		Use:           migrateCommandUseConstant,
		Short:         migrateCommandShortDescriptionConstant,
		Long:          migrateCommandLongDescriptionConstant,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          builder.runMigrate,
	}

	migrateCommand.Flags().Bool(debugFlagNameConstant, false, debugFlagDescriptionConstant)

	branchCommand.AddCommand(migrateCommand)

	return branchCommand, nil
}

func (builder *CommandBuilder) runMigrate(command *cobra.Command, arguments []string) error {
	options, optionsError := builder.parseOptions(command, arguments)
	if optionsError != nil {
		return optionsError
	}

	logger := builder.resolveLogger(options.enableDebug)

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
			SourceBranch:         BranchMain,
			TargetBranch:         BranchMaster,
			PushUpdates:          true,
			EnableDebugLogging:   options.enableDebug,
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
	debugEnabled, _ := command.Flags().GetBool(debugFlagNameConstant)
	repositoryRoots := builder.determineRepositoryRoots(arguments)
	return commandOptions{enableDebug: debugEnabled, repositoryRoots: repositoryRoots}, nil
}

func (builder *CommandBuilder) determineRepositoryRoots(arguments []string) []string {
	repositoryRoots := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		trimmedRoot := strings.TrimSpace(argument)
		if len(trimmedRoot) == 0 {
			continue
		}
		repositoryRoots = append(repositoryRoots, trimmedRoot)
	}

	if len(repositoryRoots) > 0 {
		return repositoryRoots
	}

	trimmedWorkingDirectory := strings.TrimSpace(builder.WorkingDirectory)
	if len(trimmedWorkingDirectory) > 0 {
		return []string{trimmedWorkingDirectory}
	}

	return []string{defaultRepositoryRootConstant}
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
	shellExecutor, creationError := execshell.NewShellExecutor(logger, commandRunner)
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
