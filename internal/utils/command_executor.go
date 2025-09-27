package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const (
	externalToolGitStringConstant             = "git"
	externalToolGitHubCLIStringConstant       = "gh"
	externalToolCurlStringConstant            = "curl"
	processRunnerNotConfiguredMessageConstant = "process runner not configured"
	environmentAssignmentSeparatorConstant    = "="
	environmentAssignmentTemplateConstant     = "%s%s%s"
)

// ExternalToolName identifies a supported executable.
type ExternalToolName string

// CommandOptions describes a tool invocation.
type CommandOptions struct {
	Arguments            []string
	WorkingDirectory     string
	EnvironmentVariables map[string]string
	StandardInput        []byte
}

// ExecutableCommand combines an ExternalToolName with specific options.
type ExecutableCommand struct {
	ToolName ExternalToolName
	CommandOptions
}

// CommandResult captures the observable results of executing a command.
type CommandResult struct {
	StandardOutput string
	StandardError  string
	ExitCode       int
}

// ExternalProcessRunner represents the ability to run executable commands.
type ExternalProcessRunner interface {
	Run(executionContext context.Context, command ExecutableCommand) (CommandResult, error)
}

// CommandExecutor coordinates command construction and execution.
type CommandExecutor struct {
	processRunner ExternalProcessRunner
}

// OSExternalProcessRunner executes commands using the operating system facilities.
type OSExternalProcessRunner struct{}

// Supported tool enumerations.
const (
	ExternalToolGit       ExternalToolName = ExternalToolName(externalToolGitStringConstant)
	ExternalToolGitHubCLI ExternalToolName = ExternalToolName(externalToolGitHubCLIStringConstant)
	ExternalToolCurl      ExternalToolName = ExternalToolName(externalToolCurlStringConstant)
)

// NewCommandExecutor builds a CommandExecutor around the provided runner.
func NewCommandExecutor(processRunner ExternalProcessRunner) *CommandExecutor {
	return &CommandExecutor{processRunner: processRunner}
}

// NewOSExternalProcessRunner creates a runner backed by os/exec.
func NewOSExternalProcessRunner() *OSExternalProcessRunner {
	return &OSExternalProcessRunner{}
}

// Execute runs an arbitrary command using the configured runner.
func (executor *CommandExecutor) Execute(executionContext context.Context, command ExecutableCommand) (CommandResult, error) {
	if executor.processRunner == nil {
		return CommandResult{}, errors.New(processRunnerNotConfiguredMessageConstant)
	}

	return executor.processRunner.Run(executionContext, command)
}

// ExecuteGitCommand runs git with the provided options.
func (executor *CommandExecutor) ExecuteGitCommand(executionContext context.Context, options CommandOptions) (CommandResult, error) {
	gitCommand := ExecutableCommand{ToolName: ExternalToolGit, CommandOptions: options}
	return executor.Execute(executionContext, gitCommand)
}

// ExecuteGitHubCommand runs the GitHub CLI with the provided options.
func (executor *CommandExecutor) ExecuteGitHubCommand(executionContext context.Context, options CommandOptions) (CommandResult, error) {
	githubCommand := ExecutableCommand{ToolName: ExternalToolGitHubCLI, CommandOptions: options}
	return executor.Execute(executionContext, githubCommand)
}

// ExecuteCurlCommand runs curl with the provided options.
func (executor *CommandExecutor) ExecuteCurlCommand(executionContext context.Context, options CommandOptions) (CommandResult, error) {
	curlCommand := ExecutableCommand{ToolName: ExternalToolCurl, CommandOptions: options}
	return executor.Execute(executionContext, curlCommand)
}

// Run executes the command using os/exec facilities.
func (runner *OSExternalProcessRunner) Run(executionContext context.Context, command ExecutableCommand) (CommandResult, error) {
	commandArguments := append([]string{}, command.Arguments...)
	executable := exec.CommandContext(executionContext, string(command.ToolName), commandArguments...)

	if len(command.WorkingDirectory) > 0 {
		executable.Dir = command.WorkingDirectory
	}

	if len(command.EnvironmentVariables) > 0 {
		mergedEnvironment := append([]string{}, os.Environ()...)
		for environmentKey, environmentValue := range command.EnvironmentVariables {
			mergedEnvironment = append(mergedEnvironment, fmt.Sprintf(environmentAssignmentTemplateConstant, environmentKey, environmentAssignmentSeparatorConstant, environmentValue))
		}
		executable.Env = mergedEnvironment
	}

	var standardOutputBuffer bytes.Buffer
	var standardErrorBuffer bytes.Buffer
	executable.Stdout = &standardOutputBuffer
	executable.Stderr = &standardErrorBuffer

	if len(command.StandardInput) > 0 {
		executable.Stdin = bytes.NewReader(command.StandardInput)
	}

	runError := executable.Run()
	if runError != nil {
		exitError := &exec.ExitError{}
		if errors.As(runError, &exitError) {
			return CommandResult{
				StandardOutput: standardOutputBuffer.String(),
				StandardError:  standardErrorBuffer.String(),
				ExitCode:       exitError.ExitCode(),
			}, nil
		}
		return CommandResult{}, runError
	}

	return CommandResult{
		StandardOutput: standardOutputBuffer.String(),
		StandardError:  standardErrorBuffer.String(),
		ExitCode:       0,
	}, nil
}
