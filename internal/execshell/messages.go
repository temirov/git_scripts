package execshell

import (
	"fmt"
	"strings"
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

// CommandMessageFormatter builds human-readable messages for command lifecycle events.
type CommandMessageFormatter struct{}

// BuildStartedMessage formats the message describing a command about to run.
func (formatter CommandMessageFormatter) BuildStartedMessage(command ShellCommand) string {
	return fmt.Sprintf(commandStartedMessageTemplateConstant, formatter.formatCommandLabel(command))
}

// BuildSuccessMessage formats the message describing a completed command with a zero exit code.
func (formatter CommandMessageFormatter) BuildSuccessMessage(command ShellCommand) string {
	return fmt.Sprintf(commandCompletedMessageTemplateConstant, formatter.formatCommandLabel(command))
}

// BuildFailureMessage formats the message describing a command that returned a non-zero exit code.
func (formatter CommandMessageFormatter) BuildFailureMessage(command ShellCommand, result ExecutionResult) string {
	baseMessage := fmt.Sprintf(commandFailedExitCodeMessageTemplateConstant, formatter.formatCommandLabel(command), result.ExitCode)
	standardErrorSuffix := formatter.formatStandardErrorSuffix(result.StandardError)
	if len(standardErrorSuffix) == 0 {
		return baseMessage
	}
	return baseMessage + standardErrorSuffix
}

// BuildExecutionFailureMessage formats the message describing an unexpected execution failure.
func (formatter CommandMessageFormatter) BuildExecutionFailureMessage(command ShellCommand, failure error) string {
	failureMessage := unknownFailureMessageConstant
	if failure != nil {
		failureMessage = failure.Error()
	}
	return fmt.Sprintf(commandExecutionFailureMessageTemplateConstant, formatter.formatCommandLabel(command), failureMessage)
}

func (formatter CommandMessageFormatter) formatCommandLabel(command ShellCommand) string {
	commandLabel := string(command.Name)
	if len(command.Details.Arguments) > 0 {
		commandLabel = fmt.Sprintf("%s %s", commandLabel, strings.Join(command.Details.Arguments, commandArgumentsJoinSeparatorConstant))
	}
	workingDirectorySuffix := formatter.formatWorkingDirectorySuffix(command)
	return fmt.Sprintf(commandLabelTemplateConstant, commandLabel, workingDirectorySuffix)
}

func (formatter CommandMessageFormatter) formatWorkingDirectorySuffix(command ShellCommand) string {
	trimmedWorkingDirectory := strings.TrimSpace(command.Details.WorkingDirectory)
	if len(trimmedWorkingDirectory) == 0 {
		return emptyStringConstant
	}
	return fmt.Sprintf(workingDirectorySuffixTemplateConstant, trimmedWorkingDirectory)
}

func (formatter CommandMessageFormatter) formatStandardErrorSuffix(standardError string) string {
	trimmedStandardError := strings.TrimSpace(standardError)
	if len(trimmedStandardError) == 0 {
		return emptyStringConstant
	}
	return fmt.Sprintf(standardErrorSuffixTemplateConstant, trimmedStandardError)
}
