package branches

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	commandUseConstant                    = "pr-cleanup"
	commandShortDescriptionConstant       = "Remove remote and local branches for closed pull requests"
	commandLongDescriptionConstant        = "pr-cleanup removes remote and local Git branches whose pull requests are already closed."
	commandExecutionErrorTemplateConstant = "branch cleanup failed: %w"
	unexpectedArgumentsMessageConstant    = "pr-cleanup does not accept positional arguments"
	flagRemoteNameConstant                = "remote"
	flagRemoteDescriptionConstant         = "Name of the remote containing pull request branches"
	flagLimitNameConstant                 = "limit"
	flagLimitDescriptionConstant          = "Maximum number of closed pull requests to examine"
	flagDryRunNameConstant                = "dry-run"
	flagDryRunDescriptionConstant         = "Preview deletions without making changes"
)

var errUnexpectedArguments = errors.New(unexpectedArgumentsMessageConstant)

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the Cobra command for branch cleanup.
type CommandBuilder struct {
	LoggerProvider   LoggerProvider
	Executor         CommandExecutor
	WorkingDirectory string
}

// Build constructs the pr-cleanup command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandUseConstant,
		Short: commandShortDescriptionConstant,
		Long:  commandLongDescriptionConstant,
		RunE:  builder.run,
	}

	command.Flags().String(flagRemoteNameConstant, defaultRemoteNameConstant, flagRemoteDescriptionConstant)
	command.Flags().Int(flagLimitNameConstant, defaultPullRequestLimitConstant, flagLimitDescriptionConstant)
	command.Flags().Bool(flagDryRunNameConstant, false, flagDryRunDescriptionConstant)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	if len(arguments) > 0 {
		return errUnexpectedArguments
	}

	options, optionsError := builder.parseOptions(command)
	if optionsError != nil {
		return optionsError
	}

	logger := builder.resolveLogger()
	executor, executorError := builder.resolveExecutor(logger)
	if executorError != nil {
		return executorError
	}

	service, serviceError := NewService(logger, executor)
	if serviceError != nil {
		return serviceError
	}

	cleanupError := service.Cleanup(command.Context(), options)
	if cleanupError != nil {
		return fmt.Errorf(commandExecutionErrorTemplateConstant, cleanupError)
	}

	return nil
}

func (builder *CommandBuilder) parseOptions(command *cobra.Command) (CleanupOptions, error) {
	remoteNameValue, _ := command.Flags().GetString(flagRemoteNameConstant)
	trimmedRemoteName := strings.TrimSpace(remoteNameValue)
	if len(trimmedRemoteName) == 0 {
		trimmedRemoteName = defaultRemoteNameConstant
	}

	limitValue, _ := command.Flags().GetInt(flagLimitNameConstant)
	if limitValue == 0 {
		limitValue = defaultPullRequestLimitConstant
	}

	dryRunValue, _ := command.Flags().GetBool(flagDryRunNameConstant)

	cleanupOptions := CleanupOptions{
		RemoteName:       trimmedRemoteName,
		PullRequestLimit: limitValue,
		DryRun:           dryRunValue,
		WorkingDirectory: builder.WorkingDirectory,
	}

	return cleanupOptions, nil
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
