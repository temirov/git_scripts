package branches

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/temirov/gix/internal/workflow"
)

const (
	taskActionNameBranchCleanup  = "repo.branches.cleanup"
	defaultBranchCleanupLimit    = 100
	branchCleanupRemoteError     = "branch cleanup action requires 'remote'"
	branchCleanupLimitParseError = "branch cleanup action requires numeric 'limit': %w"
)

func init() {
	workflow.RegisterTaskAction(taskActionNameBranchCleanup, handleBranchCleanupAction)
}

func handleBranchCleanupAction(ctx context.Context, environment *workflow.Environment, repository *workflow.RepositoryState, parameters map[string]any) error {
	if environment == nil || environment.GitExecutor == nil || repository == nil {
		return nil
	}

	remoteValue, remoteExists := parameters["remote"]
	remoteString := strings.TrimSpace(stringify(remoteValue))
	if !remoteExists || len(remoteString) == 0 {
		return errors.New(branchCleanupRemoteError)
	}

	limitValue, _ := parameters["limit"]
	cleanupLimit := defaultBranchCleanupLimit
	if trimmedLimit := strings.TrimSpace(stringify(limitValue)); len(trimmedLimit) > 0 {
		parsedLimit, parseError := strconv.Atoi(trimmedLimit)
		if parseError != nil {
			return fmt.Errorf(branchCleanupLimitParseError, parseError)
		}
		cleanupLimit = parsedLimit
	}

	service, serviceError := NewService(environment.Logger, environment.GitExecutor, environment.Prompter)
	if serviceError != nil {
		return serviceError
	}

	assumeYes := false
	if environment.PromptState != nil && environment.PromptState.IsAssumeYesEnabled() {
		assumeYes = true
	}

	options := CleanupOptions{
		RemoteName:       remoteString,
		PullRequestLimit: cleanupLimit,
		DryRun:           environment.DryRun,
		WorkingDirectory: repository.Path,
		AssumeYes:        assumeYes,
	}

	return service.Cleanup(ctx, options)
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}
