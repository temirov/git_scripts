package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
)

const (
	branchCommandUseConstant                     = "branch"
	branchCommandShortDescriptionConstant        = "Manage repository branches"
	branchCommandLongDescriptionConstant         = "Branch utilities for Git repositories."
	migrateCommandUseConstant                    = "migrate"
	migrateCommandShortDescriptionConstant       = "Migrate repository defaults from main to master"
	migrateCommandLongDescriptionConstant        = "branch migrate retargets workflows, updates GitHub configuration, and evaluates safety gates before switching the default branch."
	migrateCommandExecutionErrorTemplateConstant = "branch migration failed: %w"
	unexpectedArgumentsMessageConstant           = "branch migrate does not accept positional arguments"
	debugFlagNameConstant                        = "debug"
	debugFlagDescriptionConstant                 = "Enable verbose debug logging for migration diagnostics"
	defaultRemoteNameConstant                    = "origin"
	workflowsDirectoryConstant                   = ".github/workflows"
	repositoryResolutionErrorTemplateConstant    = "unable to resolve repository identifier: %w"
	repositoryOwnerRepositoryFormatConstant      = "%s/%s"
	workingDirectoryResolutionErrorTemplate      = "unable to resolve working directory: %w"
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
)

type commandOptions struct {
	enableDebug bool
}

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the Cobra command hierarchy for branch migration.
type CommandBuilder struct {
	LoggerProvider   LoggerProvider
	Executor         CommandExecutor
	WorkingDirectory string
}

var errUnexpectedArguments = errors.New(unexpectedArgumentsMessageConstant)

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
	if len(arguments) > 0 {
		return errUnexpectedArguments
	}

	options, optionsError := builder.parseOptions(command)
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

	workingDirectory := builder.WorkingDirectory
	if len(strings.TrimSpace(workingDirectory)) == 0 {
		resolvedDirectory, directoryError := os.Getwd()
		if directoryError != nil {
			return fmt.Errorf(workingDirectoryResolutionErrorTemplate, directoryError)
		}
		workingDirectory = resolvedDirectory
	}

	normalizedWorkingDirectory := filepath.Clean(workingDirectory)

	remoteURL, remoteError := repositoryManager.GetRemoteURL(command.Context(), normalizedWorkingDirectory, defaultRemoteNameConstant)
	if remoteError != nil {
		return fmt.Errorf(repositoryResolutionErrorTemplateConstant, remoteError)
	}

	parsedRemote, parseError := gitrepo.ParseRemoteURL(remoteURL)
	if parseError != nil {
		return fmt.Errorf(repositoryResolutionErrorTemplateConstant, parseError)
	}

	repositoryIdentifier := fmt.Sprintf(repositoryOwnerRepositoryFormatConstant, parsedRemote.Owner, parsedRemote.Repository)

	service, serviceError := NewService(ServiceDependencies{
		Logger:            logger,
		RepositoryManager: repositoryManager,
		GitHubClient:      githubClient,
		GitExecutor:       executor,
	})
	if serviceError != nil {
		return serviceError
	}

	migrationOptions := MigrationOptions{
		RepositoryPath:       normalizedWorkingDirectory,
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
		return fmt.Errorf(migrateCommandExecutionErrorTemplateConstant, migrationError)
	}

	builder.logSummary(logger, result)

	return nil
}

func (builder *CommandBuilder) parseOptions(command *cobra.Command) (commandOptions, error) {
	debugEnabled, _ := command.Flags().GetBool(debugFlagNameConstant)
	return commandOptions{enableDebug: debugEnabled}, nil
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

func (builder *CommandBuilder) logSummary(logger *zap.Logger, result MigrationResult) {
	if logger == nil {
		return
	}

	logger.Info(
		migrationCompletedMessageConstant,
		zap.Strings(migratedWorkflowFilesFieldConstant, result.WorkflowOutcome.UpdatedFiles),
		zap.Bool(defaultBranchUpdatedFieldConstant, result.DefaultBranchUpdated),
		zap.Bool(pagesConfigurationUpdatedFieldConstant, result.PagesConfigurationUpdated),
		zap.Ints(retargetedPullRequestsFieldConstant, result.RetargetedPullRequests),
		zap.Bool(safeToDeleteFieldConstant, result.SafetyStatus.SafeToDelete),
	)

	if !result.SafetyStatus.SafeToDelete {
		logger.Warn(safetyGatesBlockingMessageConstant, zap.Strings(safetyGateReasonsFieldConstant, result.SafetyStatus.BlockingReasons))
	}
}
