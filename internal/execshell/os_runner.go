package execshell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const (
	environmentAssignmentSeparatorConstant = "="
	environmentAssignmentTemplateConstant  = "%s%s%s"
)

// OSCommandRunner executes commands using the operating system facilities.
type OSCommandRunner struct{}

// NewOSCommandRunner constructs a runner backed by os/exec.
func NewOSCommandRunner() *OSCommandRunner {
	return &OSCommandRunner{}
}

// Run executes the supplied command using os/exec.
func (runner *OSCommandRunner) Run(executionContext context.Context, command ShellCommand) (ExecutionResult, error) {
	commandArguments := append([]string{}, command.Details.Arguments...)
	executable := exec.CommandContext(executionContext, string(command.Name), commandArguments...)

	if len(command.Details.WorkingDirectory) > 0 {
		executable.Dir = command.Details.WorkingDirectory
	}

	if len(command.Details.EnvironmentVariables) > 0 {
		mergedEnvironment := append([]string{}, os.Environ()...)
		for environmentKey, environmentValue := range command.Details.EnvironmentVariables {
			mergedEnvironment = append(mergedEnvironment, fmt.Sprintf(environmentAssignmentTemplateConstant, environmentKey, environmentAssignmentSeparatorConstant, environmentValue))
		}
		executable.Env = mergedEnvironment
	}

	var standardOutputBuffer bytes.Buffer
	var standardErrorBuffer bytes.Buffer
	executable.Stdout = &standardOutputBuffer
	executable.Stderr = &standardErrorBuffer

	if len(command.Details.StandardInput) > 0 {
		executable.Stdin = bytes.NewReader(command.Details.StandardInput)
	}

	runError := executable.Run()
	if runError != nil {
		exitError := &exec.ExitError{}
		if errors.As(runError, &exitError) {
			return ExecutionResult{
				StandardOutput: standardOutputBuffer.String(),
				StandardError:  standardErrorBuffer.String(),
				ExitCode:       exitError.ExitCode(),
			}, nil
		}
		return ExecutionResult{}, runError
	}

	return ExecutionResult{
		StandardOutput: standardOutputBuffer.String(),
		StandardError:  standardErrorBuffer.String(),
		ExitCode:       0,
	}, nil
}
