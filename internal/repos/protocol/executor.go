package protocol

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/temirov/git_scripts/internal/repos/remotes"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	ownerRepoErrorMessage       = "ERROR: cannot derive owner/repo for protocol conversion in %s\n"
	targetErrorMessage          = "ERROR: cannot build target URL for protocol '%s' in %s\n"
	planMessage                 = "PLAN-CONVERT: %s origin %s → %s\n"
	promptTemplate              = "Convert 'origin' in '%s' (%s → %s)? [y/N] "
	declinedMessage             = "CONVERT-SKIP: user declined for %s\n"
	successMessage              = "CONVERT-DONE: %s origin now %s\n"
	failureMessage              = "ERROR: failed to set origin to %s in %s\n"
	gitProtocolPrefixConstant   = "git@github.com:"
	sshProtocolPrefixConstant   = "ssh://git@github.com/"
	httpsProtocolPrefixConstant = "https://github.com/"
)

// Options configures the protocol conversion workflow.
type Options struct {
	RepositoryPath           string
	OriginOwnerRepository    string
	CanonicalOwnerRepository string
	CurrentProtocol          shared.RemoteProtocol
	TargetProtocol           shared.RemoteProtocol
	DryRun                   bool
	AssumeYes                bool
}

// Dependencies supplies collaborators required for protocol conversion.
type Dependencies struct {
	GitManager shared.GitRepositoryManager
	Prompter   shared.ConfirmationPrompter
	Output     io.Writer
	Errors     io.Writer
}

// Executor orchestrates protocol conversions for repository remotes.
type Executor struct {
	dependencies Dependencies
}

// NewExecutor constructs an Executor with the provided dependencies.
func NewExecutor(dependencies Dependencies) *Executor {
	return &Executor{dependencies: dependencies}
}

// Execute performs the conversion using the executor's dependencies.
func (executor *Executor) Execute(executionContext context.Context, options Options) {
	if executor.dependencies.GitManager == nil {
		executor.printfError(failureMessage, "", options.RepositoryPath)
		return
	}

	currentURL, fetchError := executor.dependencies.GitManager.GetRemoteURL(executionContext, options.RepositoryPath, shared.OriginRemoteNameConstant)
	if fetchError != nil {
		executor.printfError(failureMessage, "", options.RepositoryPath)
		return
	}

	currentProtocol := detectProtocol(currentURL)
	if currentProtocol != options.CurrentProtocol {
		return
	}

	ownerRepo := strings.TrimSpace(options.CanonicalOwnerRepository)
	if len(ownerRepo) == 0 {
		ownerRepo = strings.TrimSpace(options.OriginOwnerRepository)
	}

	if len(ownerRepo) == 0 {
		executor.printfError(ownerRepoErrorMessage, options.RepositoryPath)
		return
	}

	targetURL, targetError := remotes.BuildRemoteURL(options.TargetProtocol, ownerRepo)
	if targetError != nil {
		executor.printfError(targetErrorMessage, string(options.TargetProtocol), options.RepositoryPath)
		return
	}

	if options.DryRun {
		executor.printfOutput(planMessage, options.RepositoryPath, currentURL, targetURL)
		return
	}

	if !options.AssumeYes && executor.dependencies.Prompter != nil {
		prompt := fmt.Sprintf(promptTemplate, options.RepositoryPath, currentProtocol, options.TargetProtocol)
		confirmed, promptError := executor.dependencies.Prompter.Confirm(prompt)
		if promptError != nil {
			executor.printfError(failureMessage, targetURL, options.RepositoryPath)
			return
		}
		if !confirmed {
			executor.printfOutput(declinedMessage, options.RepositoryPath)
			return
		}
	}

	updateError := executor.dependencies.GitManager.SetRemoteURL(executionContext, options.RepositoryPath, shared.OriginRemoteNameConstant, targetURL)
	if updateError != nil {
		executor.printfError(failureMessage, targetURL, options.RepositoryPath)
		return
	}

	executor.printfOutput(successMessage, options.RepositoryPath, targetURL)
}

// Execute performs the conversion using transient executor state.
func Execute(executionContext context.Context, dependencies Dependencies, options Options) {
	NewExecutor(dependencies).Execute(executionContext, options)
}

func (executor *Executor) printfOutput(format string, arguments ...any) {
	if executor.dependencies.Output == nil {
		return
	}
	fmt.Fprintf(executor.dependencies.Output, format, arguments...)
}

func (executor *Executor) printfError(format string, arguments ...any) {
	if executor.dependencies.Errors == nil {
		return
	}
	fmt.Fprintf(executor.dependencies.Errors, format, arguments...)
}

func detectProtocol(remoteURL string) shared.RemoteProtocol {
	switch {
	case strings.HasPrefix(remoteURL, gitProtocolPrefixConstant):
		return shared.RemoteProtocolGit
	case strings.HasPrefix(remoteURL, sshProtocolPrefixConstant):
		return shared.RemoteProtocolSSH
	case strings.HasPrefix(remoteURL, httpsProtocolPrefixConstant):
		return shared.RemoteProtocolHTTPS
	default:
		return shared.RemoteProtocolOther
	}
}
