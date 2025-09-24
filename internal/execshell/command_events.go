package execshell

// CommandEventObserver receives lifecycle notifications for shell command execution.
type CommandEventObserver interface {
	// CommandStarted notifies observers that command execution is beginning.
	CommandStarted(command ShellCommand)
	// CommandCompleted notifies observers that command execution finished and supplies the result.
	CommandCompleted(command ShellCommand, result ExecutionResult)
	// CommandExecutionFailed reports unexpected failures prior to receiving an execution result.
	CommandExecutionFailed(command ShellCommand, failure error)
}

// noopCommandEventObserver discards all command events.
type noopCommandEventObserver struct{}

// CommandStarted implements CommandEventObserver for the no-op observer.
func (noopCommandEventObserver) CommandStarted(ShellCommand) {}

// CommandCompleted implements CommandEventObserver for the no-op observer.
func (noopCommandEventObserver) CommandCompleted(ShellCommand, ExecutionResult) {}

// CommandExecutionFailed implements CommandEventObserver for the no-op observer.
func (noopCommandEventObserver) CommandExecutionFailed(ShellCommand, error) {}
