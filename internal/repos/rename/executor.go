package rename

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	planSkipAlreadyMessage       = "PLAN-SKIP (already named): %s\n"
	planSkipDirtyMessage         = "PLAN-SKIP (dirty worktree): %s\n"
	planSkipParentMissingMessage = "PLAN-SKIP (target parent missing): %s\n"
	planSkipExistsMessage        = "PLAN-SKIP (target exists): %s\n"
	planCaseOnlyMessage          = "PLAN-CASE-ONLY: %s → %s (two-step move required)\n"
	planReadyMessage             = "PLAN-OK: %s → %s\n"
	errorAlreadyNamedMessage     = "ERROR: already named: %s\n"
	errorDirtyMessage            = "ERROR: dirty worktree: %s\n"
	errorParentMissingMessage    = "ERROR: target parent missing: %s\n"
	errorTargetExistsMessage     = "ERROR: target exists: %s\n"
	promptTemplate               = "Rename '%s' → '%s'? [y/N] "
	skipMessage                  = "SKIP: %s\n"
	successMessage               = "Renamed %s → %s\n"
	failureMessage               = "ERROR: rename failed for %s → %s\n"
	intermediateRenameTemplate   = "%s.rename.%d"
)

// Options configures a rename execution.
type Options struct {
	RepositoryPath       string
	DesiredFolderName    string
	DryRun               bool
	RequireCleanWorktree bool
	AssumeYes            bool
}

// Dependencies supplies collaborators required to evaluate rename operations.
type Dependencies struct {
	FileSystem shared.FileSystem
	GitManager shared.GitRepositoryManager
	Prompter   shared.ConfirmationPrompter
	Clock      shared.Clock
	Output     io.Writer
	Errors     io.Writer
}

// Executor orchestrates rename planning and execution for repositories.
type Executor struct {
	dependencies Dependencies
}

// NewExecutor constructs an Executor from the provided dependencies.
func NewExecutor(dependencies Dependencies) *Executor {
	if dependencies.Clock == nil {
		dependencies.Clock = shared.SystemClock{}
	}
	return &Executor{dependencies: dependencies}
}

// Execute performs the rename workflow using the executor's dependencies.
func (executor *Executor) Execute(executionContext context.Context, options Options) {
	desiredName := strings.TrimSpace(options.DesiredFolderName)
	if len(desiredName) == 0 {
		return
	}

	if executor.dependencies.FileSystem == nil {
		executor.printfError(failureMessage, options.RepositoryPath, desiredName)
		return
	}

	oldAbsolutePath, absError := executor.dependencies.FileSystem.Abs(options.RepositoryPath)
	if absError != nil {
		executor.printfError(failureMessage, options.RepositoryPath, desiredName)
		return
	}

	parentDirectory := filepath.Dir(oldAbsolutePath)
	newAbsolutePath := filepath.Join(parentDirectory, desiredName)

	if options.DryRun {
		executor.printPlan(executionContext, oldAbsolutePath, newAbsolutePath, options.RequireCleanWorktree)
		return
	}

	if !executor.validatePrerequisites(executionContext, oldAbsolutePath, newAbsolutePath, options.RequireCleanWorktree) {
		return
	}

	if !options.AssumeYes && executor.dependencies.Prompter != nil {
		prompt := fmt.Sprintf(promptTemplate, oldAbsolutePath, newAbsolutePath)
		confirmed, promptError := executor.dependencies.Prompter.Confirm(prompt)
		if promptError != nil {
			executor.printfError(failureMessage, oldAbsolutePath, newAbsolutePath)
			return
		}
		if !confirmed {
			executor.printfOutput(skipMessage, oldAbsolutePath)
			return
		}
	}

	if executor.performRename(oldAbsolutePath, newAbsolutePath) {
		executor.printfOutput(successMessage, oldAbsolutePath, newAbsolutePath)
	} else {
		executor.printfError(failureMessage, oldAbsolutePath, newAbsolutePath)
	}
}

// Execute performs the rename workflow using transient executor state.
func Execute(executionContext context.Context, dependencies Dependencies, options Options) {
	NewExecutor(dependencies).Execute(executionContext, options)
}

func (executor *Executor) printPlan(executionContext context.Context, oldAbsolutePath string, newAbsolutePath string, requireClean bool) {
	switch {
	case oldAbsolutePath == newAbsolutePath:
		executor.printfOutput(planSkipAlreadyMessage, oldAbsolutePath)
		return
	case requireClean && !executor.isClean(executionContext, oldAbsolutePath):
		executor.printfOutput(planSkipDirtyMessage, oldAbsolutePath)
		return
	case !executor.parentExists(newAbsolutePath):
		executor.printfOutput(planSkipParentMissingMessage, filepath.Dir(newAbsolutePath))
		return
	case executor.targetExists(newAbsolutePath):
		executor.printfOutput(planSkipExistsMessage, newAbsolutePath)
		return
	}

	if isCaseOnlyRename(oldAbsolutePath, newAbsolutePath) {
		executor.printfOutput(planCaseOnlyMessage, oldAbsolutePath, newAbsolutePath)
		return
	}

	executor.printfOutput(planReadyMessage, oldAbsolutePath, newAbsolutePath)
}

func (executor *Executor) validatePrerequisites(executionContext context.Context, oldAbsolutePath string, newAbsolutePath string, requireClean bool) bool {
	if oldAbsolutePath == newAbsolutePath {
		executor.printfError(errorAlreadyNamedMessage, oldAbsolutePath)
		return false
	}

	if requireClean && !executor.isClean(executionContext, oldAbsolutePath) {
		executor.printfError(errorDirtyMessage, oldAbsolutePath)
		return false
	}

	if !executor.parentExists(newAbsolutePath) {
		executor.printfError(errorParentMissingMessage, filepath.Dir(newAbsolutePath))
		return false
	}

	if executor.targetExists(newAbsolutePath) {
		executor.printfError(errorTargetExistsMessage, newAbsolutePath)
		return false
	}

	return true
}

func (executor *Executor) isClean(executionContext context.Context, repositoryPath string) bool {
	if executor.dependencies.GitManager == nil {
		return false
	}

	clean, cleanError := executor.dependencies.GitManager.CheckCleanWorktree(executionContext, repositoryPath)
	if cleanError != nil {
		return false
	}
	return clean
}

func (executor *Executor) parentExists(path string) bool {
	if executor.dependencies.FileSystem == nil {
		return false
	}
	_, statError := executor.dependencies.FileSystem.Stat(filepath.Dir(path))
	return statError == nil
}

func (executor *Executor) targetExists(path string) bool {
	if executor.dependencies.FileSystem == nil {
		return false
	}
	_, statError := executor.dependencies.FileSystem.Stat(path)
	return statError == nil
}

func (executor *Executor) performRename(oldAbsolutePath string, newAbsolutePath string) bool {
	if executor.dependencies.FileSystem == nil {
		return false
	}

	if isCaseOnlyRename(oldAbsolutePath, newAbsolutePath) {
		timestamp := executor.dependencies.Clock.Now().UnixNano()
		intermediatePath := computeIntermediateRenamePath(oldAbsolutePath, timestamp)
		if renameError := executor.dependencies.FileSystem.Rename(oldAbsolutePath, intermediatePath); renameError != nil {
			return false
		}
		if renameError := executor.dependencies.FileSystem.Rename(intermediatePath, newAbsolutePath); renameError != nil {
			_ = executor.dependencies.FileSystem.Rename(intermediatePath, oldAbsolutePath)
			return false
		}
		return true
	}

	if renameError := executor.dependencies.FileSystem.Rename(oldAbsolutePath, newAbsolutePath); renameError != nil {
		return false
	}
	return true
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

func isCaseOnlyRename(oldPath string, newPath string) bool {
	return strings.EqualFold(oldPath, newPath) && oldPath != newPath
}

func computeIntermediateRenamePath(oldPath string, timestamp int64) string {
	return fmt.Sprintf(intermediateRenameTemplate, oldPath, timestamp)
}
