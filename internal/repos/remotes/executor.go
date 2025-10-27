package remotes

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/temirov/gix/internal/repos/shared"
)

const (
	skipParseMessage                 = "UPDATE-REMOTE-SKIP: %s (error: could not parse origin owner/repo)\n"
	skipCanonicalMessage             = "UPDATE-REMOTE-SKIP: %s (no upstream: no canonical redirect found)\n"
	skipSameMessage                  = "UPDATE-REMOTE-SKIP: %s (already canonical)\n"
	skipTargetMessage                = "UPDATE-REMOTE-SKIP: %s (error: could not construct target URL)\n"
	skipOwnerConstraintMessage       = "UPDATE-REMOTE-SKIP: %s (owner constraint unmet: required --owner %s but detected owner %s)\n"
	planMessage                      = "PLAN-UPDATE-REMOTE: %s origin %s → %s\n"
	promptTemplate                   = "Update 'origin' in '%s' to canonical (%s → %s)? [a/N/y] "
	declinedMessage                  = "UPDATE-REMOTE-SKIP: user declined for %s\n"
	successMessage                   = "UPDATE-REMOTE-DONE: %s origin now %s\n"
	failureMessage                   = "UPDATE-REMOTE-SKIP: %s (error: failed to set origin URL)\n"
	ownerRepoNotDetectedErrorMessage = "owner repository not detected"
	unknownProtocolErrorTemplate     = "unknown protocol %s"
	gitProtocolURLTemplate           = "git@github.com:%s.git"
	sshProtocolURLTemplate           = "ssh://git@github.com/%s.git"
	httpsProtocolURLTemplate         = "https://github.com/%s.git"
	ownerConstraintUnknownValue      = "unknown"
	ownerRepositorySeparator         = "/"
)

// Options configures the remote update workflow.
type Options struct {
	RepositoryPath           string
	CurrentOriginURL         string
	OriginOwnerRepository    string
	CanonicalOwnerRepository string
	RemoteProtocol           shared.RemoteProtocol
	DryRun                   bool
	AssumeYes                bool
	OwnerConstraint          string
}

// Dependencies captures collaborators required to update remotes.
type Dependencies struct {
	GitManager shared.GitRepositoryManager
	Prompter   shared.ConfirmationPrompter
	Output     io.Writer
}

// Executor orchestrates canonical remote updates.
type Executor struct {
	dependencies Dependencies
}

// NewExecutor constructs an Executor from the provided dependencies.
func NewExecutor(dependencies Dependencies) *Executor {
	return &Executor{dependencies: dependencies}
}

// Execute performs the remote update according to the provided options.
func (executor *Executor) Execute(executionContext context.Context, options Options) {
	trimmedOrigin := strings.TrimSpace(options.OriginOwnerRepository)
	if len(trimmedOrigin) == 0 {
		executor.printfOutput(skipParseMessage, options.RepositoryPath)
		return
	}

	trimmedCanonical := strings.TrimSpace(options.CanonicalOwnerRepository)
	if len(trimmedCanonical) == 0 {
		executor.printfOutput(skipCanonicalMessage, options.RepositoryPath)
		return
	}

	if strings.EqualFold(trimmedOrigin, trimmedCanonical) {
		executor.printfOutput(skipSameMessage, options.RepositoryPath)
		return
	}

	requiredOwner := strings.TrimSpace(options.OwnerConstraint)
	if len(requiredOwner) > 0 {
		actualOwner := canonicalOwner(trimmedCanonical)
		if len(actualOwner) == 0 {
			actualOwner = canonicalOwner(trimmedOrigin)
		}
		if len(actualOwner) == 0 {
			actualOwner = ownerConstraintUnknownValue
		}
		if !strings.EqualFold(actualOwner, requiredOwner) {
			executor.printfOutput(skipOwnerConstraintMessage, options.RepositoryPath, requiredOwner, actualOwner)
			return
		}
	}

	targetURL, targetError := BuildRemoteURL(options.RemoteProtocol, trimmedCanonical)
	if targetError != nil {
		executor.printfOutput(skipTargetMessage, options.RepositoryPath)
		return
	}

	if options.DryRun {
		executor.printfOutput(planMessage, options.RepositoryPath, options.CurrentOriginURL, targetURL)
		return
	}

	if !options.AssumeYes && executor.dependencies.Prompter != nil {
		prompt := fmt.Sprintf(promptTemplate, options.RepositoryPath, trimmedOrigin, trimmedCanonical)
		confirmationResult, promptError := executor.dependencies.Prompter.Confirm(prompt)
		if promptError != nil {
			executor.printfOutput(skipTargetMessage, options.RepositoryPath)
			return
		}
		if !confirmationResult.Confirmed {
			executor.printfOutput(declinedMessage, options.RepositoryPath)
			return
		}
	}

	if executor.dependencies.GitManager == nil {
		executor.printfOutput(failureMessage, options.RepositoryPath)
		return
	}

	updateError := executor.dependencies.GitManager.SetRemoteURL(executionContext, options.RepositoryPath, shared.OriginRemoteNameConstant, targetURL)
	if updateError != nil {
		executor.printfOutput(failureMessage, options.RepositoryPath)
		return
	}

	executor.printfOutput(successMessage, options.RepositoryPath, targetURL)
}

// Execute performs the remote update workflow using transient executor state.
func Execute(executionContext context.Context, dependencies Dependencies, options Options) {
	NewExecutor(dependencies).Execute(executionContext, options)
}

func (executor *Executor) printfOutput(format string, arguments ...any) {
	if executor.dependencies.Output == nil {
		return
	}
	fmt.Fprintf(executor.dependencies.Output, format, arguments...)
}

func canonicalOwner(ownerRepository string) string {
	trimmed := strings.TrimSpace(ownerRepository)
	if len(trimmed) == 0 {
		return ""
	}

	segments := strings.Split(trimmed, ownerRepositorySeparator)
	if len(segments) == 0 {
		return ""
	}

	owner := strings.TrimSpace(segments[0])
	if len(owner) == 0 {
		return ""
	}

	return owner
}

// BuildRemoteURL formats the canonical remote URL for the provided protocol and owner/repository tuple.
func BuildRemoteURL(protocol shared.RemoteProtocol, ownerRepo string) (string, error) {
	trimmedOwnerRepo := strings.TrimSpace(ownerRepo)
	if len(trimmedOwnerRepo) == 0 {
		return "", fmt.Errorf(ownerRepoNotDetectedErrorMessage)
	}

	switch protocol {
	case shared.RemoteProtocolGit:
		return fmt.Sprintf(gitProtocolURLTemplate, trimmedOwnerRepo), nil
	case shared.RemoteProtocolSSH:
		return fmt.Sprintf(sshProtocolURLTemplate, trimmedOwnerRepo), nil
	case shared.RemoteProtocolHTTPS:
		return fmt.Sprintf(httpsProtocolURLTemplate, trimmedOwnerRepo), nil
	default:
		return "", fmt.Errorf(unknownProtocolErrorTemplate, protocol)
	}
}
