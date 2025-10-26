package cd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	repositoryPathRequiredMessageConstant    = "repository path must be provided"
	branchNameRequiredMessageConstant        = "branch name must be provided"
	gitExecutorMissingMessageConstant        = "git executor not configured"
	gitFetchFailureTemplateConstant          = "failed to fetch updates: %w"
	gitSwitchFailureTemplateConstant         = "failed to switch to branch %q: %w"
	gitCreateBranchFailureTemplateConstant   = "failed to create branch %q from %s: %w"
	gitPullFailureTemplateConstant           = "failed to pull latest changes: %w"
	defaultRemoteNameConstant                = "origin"
	gitFetchSubcommandConstant               = "fetch"
	gitFetchAllFlagConstant                  = "--all"
	gitFetchPruneFlagConstant                = "--prune"
	gitSwitchSubcommandConstant              = "switch"
	gitCreateBranchFlagConstant              = "-c"
	gitTrackFlagConstant                     = "--track"
	gitPullSubcommandConstant                = "pull"
	gitPullRebaseFlagConstant                = "--rebase"
	gitTerminalPromptEnvironmentNameConstant = "GIT_TERMINAL_PROMPT"
	gitTerminalPromptEnvironmentDisableValue = "0"
)

// ErrRepositoryPathRequired indicates the repository path option was empty.
var ErrRepositoryPathRequired = errors.New(repositoryPathRequiredMessageConstant)

// ErrBranchNameRequired indicates the branch name option was empty.
var ErrBranchNameRequired = errors.New(branchNameRequiredMessageConstant)

// ErrGitExecutorNotConfigured indicates the git executor dependency was missing.
var ErrGitExecutorNotConfigured = errors.New(gitExecutorMissingMessageConstant)

// ServiceDependencies enumerates collaborators required by the service.
type ServiceDependencies struct {
	GitExecutor shared.GitExecutor
}

// Options configure a branch change operation.
type Options struct {
	RepositoryPath  string
	BranchName      string
	RemoteName      string
	CreateIfMissing bool
	DryRun          bool
}

// Result captures the outcome of a branch change.
type Result struct {
	RepositoryPath string
	BranchName     string
	BranchCreated  bool
}

// Service coordinates branch switching across repositories.
type Service struct {
	executor shared.GitExecutor
}

// NewService constructs a Service from the provided dependencies.
func NewService(dependencies ServiceDependencies) (*Service, error) {
	if dependencies.GitExecutor == nil {
		return nil, ErrGitExecutorNotConfigured
	}
	return &Service{executor: dependencies.GitExecutor}, nil
}

// Change switches the repository to the requested branch, creating it from the remote if needed.
func (service *Service) Change(executionContext context.Context, options Options) (Result, error) {
	trimmedRepositoryPath := strings.TrimSpace(options.RepositoryPath)
	if len(trimmedRepositoryPath) == 0 {
		return Result{}, ErrRepositoryPathRequired
	}

	trimmedBranchName := strings.TrimSpace(options.BranchName)
	if len(trimmedBranchName) == 0 {
		return Result{}, ErrBranchNameRequired
	}

	remoteName := strings.TrimSpace(options.RemoteName)
	if len(remoteName) == 0 {
		remoteName = defaultRemoteNameConstant
	}

	if options.DryRun {
		return Result{RepositoryPath: trimmedRepositoryPath, BranchName: trimmedBranchName}, nil
	}

	environment := map[string]string{gitTerminalPromptEnvironmentNameConstant: gitTerminalPromptEnvironmentDisableValue}

	if _, err := service.executor.ExecuteGit(executionContext, execshell.CommandDetails{
		Arguments:            []string{gitFetchSubcommandConstant, gitFetchAllFlagConstant, gitFetchPruneFlagConstant},
		WorkingDirectory:     trimmedRepositoryPath,
		EnvironmentVariables: environment,
	}); err != nil {
		return Result{}, fmt.Errorf(gitFetchFailureTemplateConstant, err)
	}

	branchCreated := false
	switchResultErr := service.trySwitch(executionContext, trimmedRepositoryPath, trimmedBranchName, environment)
	if switchResultErr != nil {
		if !options.CreateIfMissing {
			return Result{}, fmt.Errorf(gitSwitchFailureTemplateConstant, trimmedBranchName, switchResultErr)
		}
		trackReference := fmt.Sprintf("%s/%s", remoteName, trimmedBranchName)
		if _, err := service.executor.ExecuteGit(executionContext, execshell.CommandDetails{
			Arguments:            []string{gitSwitchSubcommandConstant, gitCreateBranchFlagConstant, trimmedBranchName, gitTrackFlagConstant, trackReference},
			WorkingDirectory:     trimmedRepositoryPath,
			EnvironmentVariables: environment,
		}); err != nil {
			return Result{}, fmt.Errorf(gitCreateBranchFailureTemplateConstant, trimmedBranchName, remoteName, err)
		}
		branchCreated = true
	}

	if _, err := service.executor.ExecuteGit(executionContext, execshell.CommandDetails{
		Arguments:            []string{gitPullSubcommandConstant, gitPullRebaseFlagConstant},
		WorkingDirectory:     trimmedRepositoryPath,
		EnvironmentVariables: environment,
	}); err != nil {
		return Result{}, fmt.Errorf(gitPullFailureTemplateConstant, err)
	}

	return Result{RepositoryPath: trimmedRepositoryPath, BranchName: trimmedBranchName, BranchCreated: branchCreated}, nil
}

func (service *Service) trySwitch(executionContext context.Context, repositoryPath string, branchName string, environment map[string]string) error {
	_, err := service.executor.ExecuteGit(executionContext, execshell.CommandDetails{
		Arguments:            []string{gitSwitchSubcommandConstant, branchName},
		WorkingDirectory:     repositoryPath,
		EnvironmentVariables: environment,
	})
	return err
}
