package execshell

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

const (
	gitCommandNameStringConstant              = "git"
	githubCLICommandNameStringConstant        = "gh"
	curlCommandNameStringConstant             = "curl"
	loggerNotConfiguredMessageConstant        = "shell executor logger not configured"
	commandRunnerNotConfiguredMessageConstant = "shell executor command runner not configured"
	commandNameMissingMessageConstant         = "shell command name not provided"
	commandStartMessageConstant               = "command execution starting"
	commandSuccessMessageConstant             = "command execution completed"
	commandFailureMessageConstant             = "command returned non-zero status"
	commandRunnerErrorMessageConstant         = "command execution error"
	commandNameFieldNameConstant              = "command"
	commandArgumentsFieldNameConstant         = "arguments"
	workingDirectoryFieldNameConstant         = "working_directory"
	exitCodeFieldNameConstant                 = "exit_code"
	standardErrorFieldNameConstant            = "stderr"
)

// CommandName identifies a supported executable name.
type CommandName string

// Supported command names.
const (
	CommandGit    CommandName = CommandName(gitCommandNameStringConstant)
	CommandGitHub CommandName = CommandName(githubCLICommandNameStringConstant)
	CommandCurl   CommandName = CommandName(curlCommandNameStringConstant)
)

// CommandDetails describes command invocation properties.
type CommandDetails struct {
	Arguments            []string
	WorkingDirectory     string
	EnvironmentVariables map[string]string
	StandardInput        []byte
}

// ShellCommand represents a fully qualified command invocation.
type ShellCommand struct {
	Name    CommandName
	Details CommandDetails
}

// ExecutionResult captures observable command results.
type ExecutionResult struct {
	StandardOutput string
	StandardError  string
	ExitCode       int
}

// CommandRunner executes shell commands.
type CommandRunner interface {
	Run(executionContext context.Context, command ShellCommand) (ExecutionResult, error)
}

// ShellExecutor orchestrates running shell commands with logging.
type ShellExecutor struct {
	commandRunner        CommandRunner
	logger               *zap.Logger
	humanReadableLogging bool
	messageFormatter     CommandMessageFormatter
}

var (
	// ErrLoggerNotConfigured indicates the logger dependency was missing.
	ErrLoggerNotConfigured = errors.New(loggerNotConfiguredMessageConstant)
	// ErrCommandRunnerNotConfigured indicates the command runner dependency was missing.
	ErrCommandRunnerNotConfigured = errors.New(commandRunnerNotConfiguredMessageConstant)
	// ErrCommandNameMissing indicates the command name was not provided.
	ErrCommandNameMissing = errors.New(commandNameMissingMessageConstant)
)

// CommandFailedError provides details about commands exiting with a non-zero code.
type CommandFailedError struct {
	Command ShellCommand
	Result  ExecutionResult
}

const commandFailureErrorMessageTemplateConstant = "%s command exited with code %d"

// Error describes the failure in a readable format.
func (commandError CommandFailedError) Error() string {
	return fmt.Sprintf(commandFailureErrorMessageTemplateConstant, commandError.Command.Name, commandError.Result.ExitCode)
}

// CommandExecutionError wraps unexpected execution failures from the runner.
type CommandExecutionError struct {
	Command ShellCommand
	Cause   error
}

const commandExecutionErrorMessageTemplateConstant = "%s command execution failed"

// Error describes the underlying runner failure.
func (executionError CommandExecutionError) Error() string {
	return fmt.Sprintf(commandExecutionErrorMessageTemplateConstant, executionError.Command.Name)
}

// Unwrap exposes the underlying error.
func (executionError CommandExecutionError) Unwrap() error {
	return executionError.Cause
}

// NewShellExecutor builds an executor for the provided runner and logger.
func NewShellExecutor(logger *zap.Logger, commandRunner CommandRunner, humanReadableLogging bool) (*ShellExecutor, error) {
	if logger == nil {
		return nil, ErrLoggerNotConfigured
	}
	if commandRunner == nil {
		return nil, ErrCommandRunnerNotConfigured
	}
	return &ShellExecutor{
		commandRunner:        commandRunner,
		logger:               logger,
		humanReadableLogging: humanReadableLogging,
		messageFormatter:     CommandMessageFormatter{},
	}, nil
}

// Execute runs the provided shell command and logs lifecycle events.
func (executor *ShellExecutor) Execute(executionContext context.Context, command ShellCommand) (ExecutionResult, error) {
	if len(command.Name) == 0 {
		return ExecutionResult{}, ErrCommandNameMissing
	}

	if executor.humanReadableLogging {
		executor.logger.Info(executor.messageFormatter.BuildStartedMessage(command))
	} else {
		executor.logger.Debug(commandStartMessageConstant,
			zap.String(commandNameFieldNameConstant, string(command.Name)),
			zap.Strings(commandArgumentsFieldNameConstant, command.Details.Arguments),
			zap.String(workingDirectoryFieldNameConstant, command.Details.WorkingDirectory),
		)
	}

	executionResult, runnerError := executor.commandRunner.Run(executionContext, command)
	if runnerError != nil {
		if executor.humanReadableLogging {
			executor.logger.Error(executor.messageFormatter.BuildExecutionFailureMessage(command, runnerError))
		} else {
			executor.logger.Error(commandRunnerErrorMessageConstant,
				zap.String(commandNameFieldNameConstant, string(command.Name)),
				zap.Error(runnerError),
			)
		}
		return ExecutionResult{}, CommandExecutionError{Command: command, Cause: runnerError}
	}

	if executionResult.ExitCode != 0 {
		if executor.humanReadableLogging {
			executor.logger.Warn(executor.messageFormatter.BuildFailureMessage(command, executionResult))
		} else {
			executor.logger.Warn(commandFailureMessageConstant,
				zap.String(commandNameFieldNameConstant, string(command.Name)),
				zap.Int(exitCodeFieldNameConstant, executionResult.ExitCode),
				zap.String(standardErrorFieldNameConstant, executionResult.StandardError),
			)
		}
		return ExecutionResult{}, CommandFailedError{Command: command, Result: executionResult}
	}

	if executor.humanReadableLogging {
		executor.logger.Info(executor.messageFormatter.BuildSuccessMessage(command))
	} else {
		executor.logger.Debug(commandSuccessMessageConstant,
			zap.String(commandNameFieldNameConstant, string(command.Name)),
			zap.Int(exitCodeFieldNameConstant, executionResult.ExitCode),
		)
	}
	return executionResult, nil
}

// ExecuteGit runs the git executable with the provided details.
func (executor *ShellExecutor) ExecuteGit(executionContext context.Context, details CommandDetails) (ExecutionResult, error) {
	return executor.Execute(executionContext, ShellCommand{Name: CommandGit, Details: details})
}

// ExecuteGitHubCLI runs the GitHub CLI executable with the provided details.
func (executor *ShellExecutor) ExecuteGitHubCLI(executionContext context.Context, details CommandDetails) (ExecutionResult, error) {
	return executor.Execute(executionContext, ShellCommand{Name: CommandGitHub, Details: details})
}

// ExecuteCurl runs the curl executable with the provided details.
func (executor *ShellExecutor) ExecuteCurl(executionContext context.Context, details CommandDetails) (ExecutionResult, error) {
	return executor.Execute(executionContext, ShellCommand{Name: CommandCurl, Details: details})
}
