package protocol

import (
	"context"
	"fmt"
	"io"
	"strings"

	repoerrors "github.com/temirov/gix/internal/repos/errors"
	"github.com/temirov/gix/internal/repos/remotes"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	ownerRepoErrorMessage = "ERROR: cannot derive owner/repo for protocol conversion in %s\n"
	targetErrorMessage    = "ERROR: cannot build target URL for protocol '%s' in %s\n"
	planMessage           = "PLAN-CONVERT: %s origin %s → %s\n"
	promptTemplate        = "Convert 'origin' in '%s' (%s → %s)? [a/N/y] "
	declinedMessage       = "CONVERT-SKIP: user declined for %s\n"
	successMessage        = "CONVERT-DONE: %s origin now %s\n"
	failureMessage        = "ERROR: failed to set origin to %s in %s\n"
)

// Options configures the protocol conversion workflow.
type Options struct {
	RepositoryPath           shared.RepositoryPath
	OriginOwnerRepository    *shared.OwnerRepository
	CanonicalOwnerRepository *shared.OwnerRepository
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
func (executor *Executor) Execute(executionContext context.Context, options Options) error {
	repositoryPath := options.RepositoryPath.String()

	if executor.dependencies.GitManager == nil {
		return repoerrors.WrapMessage(
			repoerrors.OperationProtocolConvert,
			repositoryPath,
			repoerrors.ErrGitManagerUnavailable,
			fmt.Sprintf(failureMessage, "", repositoryPath),
		)
	}

	currentURL, fetchError := executor.dependencies.GitManager.GetRemoteURL(executionContext, repositoryPath, shared.OriginRemoteNameConstant)
	if fetchError != nil {
		return repoerrors.Wrap(
			repoerrors.OperationProtocolConvert,
			repositoryPath,
			repoerrors.ErrRemoteEnumerationFailed,
			fetchError,
		)
	}

	currentProtocol := detectProtocol(currentURL)
	if currentProtocol != options.CurrentProtocol {
		return nil
	}

	var ownerRepository *shared.OwnerRepository
	if options.CanonicalOwnerRepository != nil {
		ownerRepository = options.CanonicalOwnerRepository
	} else if options.OriginOwnerRepository != nil {
		ownerRepository = options.OriginOwnerRepository
	}

	if ownerRepository == nil {
		return repoerrors.WrapMessage(
			repoerrors.OperationProtocolConvert,
			repositoryPath,
			repoerrors.ErrOriginOwnerMissing,
			fmt.Sprintf(ownerRepoErrorMessage, repositoryPath),
		)
	}

	ownerRepoString := ownerRepository.String()

	targetURL, targetError := remotes.BuildRemoteURL(options.TargetProtocol, ownerRepoString)
	if targetError != nil {
		return repoerrors.WrapMessage(
			repoerrors.OperationProtocolConvert,
			repositoryPath,
			repoerrors.ErrUnknownProtocol,
			fmt.Sprintf(targetErrorMessage, string(options.TargetProtocol), repositoryPath),
		)
	}

	if options.DryRun {
		executor.printfOutput(planMessage, repositoryPath, currentURL, targetURL)
		return nil
	}

	if !options.AssumeYes && executor.dependencies.Prompter != nil {
		prompt := fmt.Sprintf(promptTemplate, repositoryPath, currentProtocol, options.TargetProtocol)
		confirmationResult, promptError := executor.dependencies.Prompter.Confirm(prompt)
		if promptError != nil {
			return repoerrors.WrapMessage(
				repoerrors.OperationProtocolConvert,
				repositoryPath,
				repoerrors.ErrUserConfirmationFailed,
				fmt.Sprintf(failureMessage, targetURL, repositoryPath),
			)
		}
		if !confirmationResult.Confirmed {
			executor.printfOutput(declinedMessage, repositoryPath)
			return nil
		}
	}

	updateError := executor.dependencies.GitManager.SetRemoteURL(executionContext, repositoryPath, shared.OriginRemoteNameConstant, targetURL)
	if updateError != nil {
		return repoerrors.WrapMessage(
			repoerrors.OperationProtocolConvert,
			repositoryPath,
			repoerrors.ErrRemoteUpdateFailed,
			fmt.Sprintf(failureMessage, targetURL, repositoryPath),
		)
	}

	executor.printfOutput(successMessage, repositoryPath, targetURL)
	return nil
}

// Execute performs the conversion using transient executor state.
func Execute(executionContext context.Context, dependencies Dependencies, options Options) error {
	return NewExecutor(dependencies).Execute(executionContext, options)
}

func (executor *Executor) printfOutput(format string, arguments ...any) {
	if executor.dependencies.Output == nil {
		return
	}
	fmt.Fprintf(executor.dependencies.Output, format, arguments...)
}

func detectProtocol(remoteURL string) shared.RemoteProtocol {
	switch {
	case strings.HasPrefix(remoteURL, shared.GitProtocolURLPrefixConstant):
		return shared.RemoteProtocolGit
	case strings.HasPrefix(remoteURL, shared.SSHProtocolURLPrefixConstant):
		return shared.RemoteProtocolSSH
	case strings.HasPrefix(remoteURL, shared.HTTPSProtocolURLPrefixConstant):
		return shared.RemoteProtocolHTTPS
	default:
		return shared.RemoteProtocolOther
	}
}
