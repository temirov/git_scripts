package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
)

const (
	repositoryPathFieldNameConstant            = "repository_path"
	remoteNameFieldNameConstant                = "remote_name"
	repositoryIdentifierFieldNameConstant      = "repository_identifier"
	workflowsDirectoryFieldNameConstant        = "workflows_directory"
	sourceBranchFieldNameConstant              = "source_branch"
	targetBranchFieldNameConstant              = "target_branch"
	gitAddCommandNameConstant                  = "add"
	gitAllFlagConstant                         = "-A"
	gitCommitCommandNameConstant               = "commit"
	gitMessageFlagConstant                     = "-m"
	gitPushCommandNameConstant                 = "push"
	workflowCommitMessageTemplateConstant      = "CI: switch workflow branch filters to %s"
	cleanWorktreeRequiredMessageConstant       = "repository worktree must be clean before migration"
	repositoryManagerMissingMessageConstant    = "repository manager not configured"
	githubClientMissingMessageConstant         = "GitHub client not configured"
	gitExecutorMissingMessageConstant          = "git executor not configured"
	workflowRewriteErrorTemplateConstant       = "workflow rewrite failed: %w"
	workflowStageErrorTemplateConstant         = "unable to stage workflow updates: %w"
	workflowCommitErrorTemplateConstant        = "unable to commit workflow updates: %w"
	workflowPushErrorTemplateConstant          = "unable to push workflow updates: %w"
	pagesUpdateErrorTemplateConstant           = "GitHub Pages update failed: %w"
	defaultBranchUpdateErrorTemplateConstant   = "unable to update default branch: %w"
	pullRequestListErrorTemplateConstant       = "unable to list pull requests: %w"
	pullRequestRetargetErrorTemplateConstant   = "unable to retarget pull request #%d: %w"
	branchProtectionCheckErrorTemplateConstant = "unable to determine branch protection: %w"
)

// InvalidInputError describes migration option validation failures.
type InvalidInputError struct {
	FieldName string
	Message   string
}

// Error describes the invalid input.
func (inputError InvalidInputError) Error() string {
	return fmt.Sprintf("%s: %s", inputError.FieldName, inputError.Message)
}

// ServiceDependencies describes required collaborators for migration.
type ServiceDependencies struct {
	Logger            *zap.Logger
	RepositoryManager *gitrepo.RepositoryManager
	GitHubClient      GitHubOperations
	GitExecutor       CommandExecutor
}

// MigrationOptions configures the migrate workflow.
type MigrationOptions struct {
	RepositoryPath       string
	RepositoryRemoteName string
	RepositoryIdentifier string
	WorkflowsDirectory   string
	SourceBranch         BranchName
	TargetBranch         BranchName
	PushUpdates          bool
	EnableDebugLogging   bool
}

// WorkflowOutcome captures workflow rewrite results.
type WorkflowOutcome struct {
	UpdatedFiles            []string
	RemainingMainReferences bool
}

// MigrationResult captures the observable outcomes.
type MigrationResult struct {
	WorkflowOutcome           WorkflowOutcome
	PagesConfigurationUpdated bool
	DefaultBranchUpdated      bool
	RetargetedPullRequests    []int
	SafetyStatus              SafetyStatus
}

// Service orchestrates the branch migration workflow.
type Service struct {
	logger            *zap.Logger
	repositoryManager *gitrepo.RepositoryManager
	gitHubClient      GitHubOperations
	gitExecutor       CommandExecutor
	workflowRewriter  *WorkflowRewriter
	pagesManager      *PagesManager
	safetyEvaluator   SafetyEvaluator
}

var (
	errRepositoryManagerMissing = errors.New(repositoryManagerMissingMessageConstant)
	errGitHubClientMissing      = errors.New(githubClientMissingMessageConstant)
	errGitExecutorMissing       = errors.New(gitExecutorMissingMessageConstant)
	errCleanWorktreeRequired    = errors.New(cleanWorktreeRequiredMessageConstant)
)

// NewService constructs a Service with the provided dependencies.
func NewService(dependencies ServiceDependencies) (*Service, error) {
	if dependencies.RepositoryManager == nil {
		return nil, errRepositoryManagerMissing
	}
	if dependencies.GitHubClient == nil {
		return nil, errGitHubClientMissing
	}
	if dependencies.GitExecutor == nil {
		return nil, errGitExecutorMissing
	}

	logger := dependencies.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	workflowRewriter := NewWorkflowRewriter(logger)
	pagesManager := NewPagesManager(logger, dependencies.GitHubClient)

	service := &Service{
		logger:            logger,
		repositoryManager: dependencies.RepositoryManager,
		gitHubClient:      dependencies.GitHubClient,
		gitExecutor:       dependencies.GitExecutor,
		workflowRewriter:  workflowRewriter,
		pagesManager:      pagesManager,
		safetyEvaluator:   SafetyEvaluator{},
	}

	return service, nil
}

// Execute performs the migration workflow.
func (service *Service) Execute(executionContext context.Context, options MigrationOptions) (MigrationResult, error) {
	if validationError := service.validateOptions(options); validationError != nil {
		return MigrationResult{}, validationError
	}

	cleanWorktree, cleanError := service.repositoryManager.CheckCleanWorktree(executionContext, options.RepositoryPath)
	if cleanError != nil {
		return MigrationResult{}, cleanError
	}
	if !cleanWorktree {
		return MigrationResult{}, errCleanWorktreeRequired
	}

	workflowOutcome, rewriteError := service.workflowRewriter.Rewrite(executionContext, WorkflowRewriteConfig{
		RepositoryPath:     options.RepositoryPath,
		WorkflowsDirectory: options.WorkflowsDirectory,
		SourceBranch:       options.SourceBranch,
		TargetBranch:       options.TargetBranch,
	})
	if rewriteError != nil {
		return MigrationResult{}, fmt.Errorf(workflowRewriteErrorTemplateConstant, rewriteError)
	}

	workflowCommitted, workflowCommitError := service.commitWorkflowChanges(executionContext, options, workflowOutcome)
	if workflowCommitError != nil {
		return MigrationResult{}, workflowCommitError
	}

	if workflowCommitted && options.PushUpdates {
		if pushError := service.pushWorkflowChanges(executionContext, options); pushError != nil {
			return MigrationResult{}, pushError
		}
	}

	pagesUpdated, pagesError := service.pagesManager.EnsureLegacyBranch(executionContext, PagesUpdateConfig{
		RepositoryIdentifier: options.RepositoryIdentifier,
		SourceBranch:         options.SourceBranch,
		TargetBranch:         options.TargetBranch,
	})
	if pagesError != nil {
		return MigrationResult{}, fmt.Errorf(pagesUpdateErrorTemplateConstant, pagesError)
	}

	if err := service.gitHubClient.SetDefaultBranch(executionContext, options.RepositoryIdentifier, string(options.TargetBranch)); err != nil {
		return MigrationResult{}, fmt.Errorf(defaultBranchUpdateErrorTemplateConstant, err)
	}

	pullRequests, listError := service.gitHubClient.ListPullRequests(executionContext, options.RepositoryIdentifier, githubcli.PullRequestListOptions{
		State:       githubcli.PullRequestStateOpen,
		BaseBranch:  string(options.SourceBranch),
		ResultLimit: defaultPullRequestQueryLimit,
	})
	if listError != nil {
		return MigrationResult{}, fmt.Errorf(pullRequestListErrorTemplateConstant, listError)
	}

	retargeted, retargetError := service.retargetPullRequests(executionContext, options, pullRequests)
	if retargetError != nil {
		return MigrationResult{}, retargetError
	}

	branchProtected, protectionError := service.gitHubClient.CheckBranchProtection(executionContext, options.RepositoryIdentifier, string(options.SourceBranch))
	if protectionError != nil {
		return MigrationResult{}, fmt.Errorf(branchProtectionCheckErrorTemplateConstant, protectionError)
	}

	safetyStatus := service.safetyEvaluator.Evaluate(SafetyInputs{
		OpenPullRequestCount: len(pullRequests),
		BranchProtected:      branchProtected,
		WorkflowMentions:     workflowOutcome.RemainingMainReferences,
	})

	result := MigrationResult{
		WorkflowOutcome:           workflowOutcome,
		PagesConfigurationUpdated: pagesUpdated,
		DefaultBranchUpdated:      true,
		RetargetedPullRequests:    retargeted,
		SafetyStatus:              safetyStatus,
	}

	return result, nil
}

func (service *Service) validateOptions(options MigrationOptions) error {
	if len(strings.TrimSpace(options.RepositoryPath)) == 0 {
		return InvalidInputError{FieldName: repositoryPathFieldNameConstant, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(options.RepositoryRemoteName)) == 0 {
		return InvalidInputError{FieldName: remoteNameFieldNameConstant, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(options.RepositoryIdentifier)) == 0 {
		return InvalidInputError{FieldName: repositoryIdentifierFieldNameConstant, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(options.WorkflowsDirectory)) == 0 {
		return InvalidInputError{FieldName: workflowsDirectoryFieldNameConstant, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(string(options.SourceBranch))) == 0 {
		return InvalidInputError{FieldName: sourceBranchFieldNameConstant, Message: requiredValueMessageConstant}
	}
	if len(strings.TrimSpace(string(options.TargetBranch))) == 0 {
		return InvalidInputError{FieldName: targetBranchFieldNameConstant, Message: requiredValueMessageConstant}
	}
	return nil
}

func (service *Service) commitWorkflowChanges(executionContext context.Context, options MigrationOptions, outcome WorkflowOutcome) (bool, error) {
	if len(outcome.UpdatedFiles) == 0 {
		return false, nil
	}

	addArguments := []string{gitAddCommandNameConstant, gitAllFlagConstant, options.WorkflowsDirectory}
	if _, stageError := service.gitExecutor.ExecuteGit(executionContext, execshell.CommandDetails{
		Arguments:        addArguments,
		WorkingDirectory: options.RepositoryPath,
	}); stageError != nil {
		return false, fmt.Errorf(workflowStageErrorTemplateConstant, stageError)
	}

	commitMessage := fmt.Sprintf(workflowCommitMessageTemplateConstant, string(options.TargetBranch))
	_, commitError := service.gitExecutor.ExecuteGit(executionContext, execshell.CommandDetails{
		Arguments:        []string{gitCommitCommandNameConstant, gitMessageFlagConstant, commitMessage},
		WorkingDirectory: options.RepositoryPath,
	})
	if commitError != nil {
		var commandFailure execshell.CommandFailedError
		if errors.As(commitError, &commandFailure) {
			service.logger.Info("No workflow changes to commit", zap.String(workflowsDirectoryFieldNameConstant, options.WorkflowsDirectory))
			return false, nil
		}
		return false, fmt.Errorf(workflowCommitErrorTemplateConstant, commitError)
	}

	return true, nil
}

func (service *Service) pushWorkflowChanges(executionContext context.Context, options MigrationOptions) error {
	pushArguments := []string{gitPushCommandNameConstant, options.RepositoryRemoteName, string(options.TargetBranch)}
	if _, pushError := service.gitExecutor.ExecuteGit(executionContext, execshell.CommandDetails{
		Arguments:        pushArguments,
		WorkingDirectory: options.RepositoryPath,
	}); pushError != nil {
		return fmt.Errorf(workflowPushErrorTemplateConstant, pushError)
	}
	return nil
}

func (service *Service) retargetPullRequests(executionContext context.Context, options MigrationOptions, pullRequests []githubcli.PullRequest) ([]int, error) {
	retargeted := make([]int, 0, len(pullRequests))
	for _, pullRequest := range pullRequests {
		retargetError := service.gitHubClient.UpdatePullRequestBase(executionContext, options.RepositoryIdentifier, pullRequest.Number, string(options.TargetBranch))
		if retargetError != nil {
			return nil, fmt.Errorf(pullRequestRetargetErrorTemplateConstant, pullRequest.Number, retargetError)
		}
		retargeted = append(retargeted, pullRequest.Number)
	}
	return retargeted, nil
}

const defaultPullRequestQueryLimit = 100

// WorkflowRewriteConfig describes the workflow rewrite inputs.
type WorkflowRewriteConfig struct {
	RepositoryPath     string
	WorkflowsDirectory string
	SourceBranch       BranchName
	TargetBranch       BranchName
}

// PagesUpdateConfig describes GitHub Pages update inputs.
type PagesUpdateConfig struct {
	RepositoryIdentifier string
	SourceBranch         BranchName
	TargetBranch         BranchName
}
