package ui

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	commandStartedMessageTemplateConstant          = "Running %s"
	commandCompletedMessageTemplateConstant        = "Completed %s"
	commandFailedExitCodeMessageTemplateConstant   = "%s failed with exit code %d"
	commandExecutionFailureMessageTemplateConstant = "%s failed: %s"
	commandLabelTemplateConstant                   = "%s%s"
	workingDirectorySuffixTemplateConstant         = " (in %s)"
	commandArgumentsJoinSeparatorConstant          = " "
	standardErrorSuffixTemplateConstant            = ": %s"
	unknownFailureMessageConstant                  = "unknown error"
	emptyStringConstant                            = ""
)

// CommandEventFormatter builds human-readable messages for command lifecycle events.
type CommandEventFormatter struct{}

// BuildStartedMessage formats the message describing a command about to run.
func (formatter CommandEventFormatter) BuildStartedMessage(command execshell.ShellCommand) string {
	return fmt.Sprintf(commandStartedMessageTemplateConstant, formatter.formatCommandLabel(command))
}

// BuildSuccessMessage formats the message describing a completed command with a zero exit code.
func (formatter CommandEventFormatter) BuildSuccessMessage(command execshell.ShellCommand) string {
	return fmt.Sprintf(commandCompletedMessageTemplateConstant, formatter.formatCommandLabel(command))
}

// BuildFailureMessage formats the message describing a command that returned a non-zero exit code.
func (formatter CommandEventFormatter) BuildFailureMessage(command execshell.ShellCommand, result execshell.ExecutionResult) string {
	baseMessage := fmt.Sprintf(commandFailedExitCodeMessageTemplateConstant, formatter.formatCommandLabel(command), result.ExitCode)
	standardErrorSuffix := formatter.formatStandardErrorSuffix(result.StandardError)
	if len(standardErrorSuffix) == 0 {
		return baseMessage
	}
	return baseMessage + standardErrorSuffix
}

// BuildExecutionFailureMessage formats the message describing an unexpected execution failure.
func (formatter CommandEventFormatter) BuildExecutionFailureMessage(command execshell.ShellCommand, failure error) string {
	failureMessage := unknownFailureMessageConstant
	if failure != nil {
		failureMessage = failure.Error()
	}
	return fmt.Sprintf(commandExecutionFailureMessageTemplateConstant, formatter.formatCommandLabel(command), failureMessage)
}

func (formatter CommandEventFormatter) formatCommandLabel(command execshell.ShellCommand) string {
	commandParts := []string{string(command.Name)}
	if len(command.Details.Arguments) > 0 {
		commandParts = append(commandParts, strings.Join(command.Details.Arguments, commandArgumentsJoinSeparatorConstant))
	}
	commandLabel := strings.Join(commandParts, commandArgumentsJoinSeparatorConstant)
	workingDirectorySuffix := formatter.formatWorkingDirectorySuffix(command)
	return fmt.Sprintf(commandLabelTemplateConstant, commandLabel, workingDirectorySuffix)
}

func (formatter CommandEventFormatter) formatWorkingDirectorySuffix(command execshell.ShellCommand) string {
	trimmedWorkingDirectory := strings.TrimSpace(command.Details.WorkingDirectory)
	if len(trimmedWorkingDirectory) == 0 {
		return emptyStringConstant
	}
	return fmt.Sprintf(workingDirectorySuffixTemplateConstant, trimmedWorkingDirectory)
}

func (formatter CommandEventFormatter) formatStandardErrorSuffix(standardError string) string {
	trimmedStandardError := strings.TrimSpace(standardError)
	if len(trimmedStandardError) == 0 {
		return emptyStringConstant
	}
	return fmt.Sprintf(standardErrorSuffixTemplateConstant, trimmedStandardError)
}

// ConsoleCommandEventLogger renders command lifecycle events using a zap logger configured for human-readable output.
type ConsoleCommandEventLogger struct {
	logger    *zap.Logger
	formatter CommandEventFormatter
}

// NewConsoleCommandEventLogger constructs a console event logger backed by the provided zap logger.
func NewConsoleCommandEventLogger(logger *zap.Logger) *ConsoleCommandEventLogger {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ConsoleCommandEventLogger{logger: logger, formatter: CommandEventFormatter{}}
}

// CommandStarted implements execshell.CommandEventObserver by logging command start notifications.
func (eventLogger *ConsoleCommandEventLogger) CommandStarted(command execshell.ShellCommand) {
	if eventLogger == nil {
		return
	}
	eventLogger.logger.Info(eventLogger.formatter.BuildStartedMessage(command))
}

// CommandCompleted implements execshell.CommandEventObserver by logging command completion notifications.
func (eventLogger *ConsoleCommandEventLogger) CommandCompleted(command execshell.ShellCommand, result execshell.ExecutionResult) {
	if eventLogger == nil {
		return
	}
	if result.ExitCode == 0 {
		eventLogger.logger.Info(eventLogger.formatter.BuildSuccessMessage(command))
		return
	}
	eventLogger.logger.Warn(eventLogger.formatter.BuildFailureMessage(command, result))
}

// CommandExecutionFailed implements execshell.CommandEventObserver by logging unexpected execution failures.
func (eventLogger *ConsoleCommandEventLogger) CommandExecutionFailed(command execshell.ShellCommand, failure error) {
	if eventLogger == nil {
		return
	}
	eventLogger.logger.Error(eventLogger.formatter.BuildExecutionFailureMessage(command, failure))
}
