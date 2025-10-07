package rename

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/temirov/gix/internal/repos/shared"
)

const (
	planSkipAlreadyMessage            = "PLAN-SKIP (already named): %s\n"
	planSkipDirtyMessage              = "PLAN-SKIP (dirty worktree): %s\n"
	planSkipParentMissingMessage      = "PLAN-SKIP (target parent missing): %s\n"
	planSkipParentNotDirectoryMessage = "PLAN-SKIP (target parent not directory): %s\n"
	planSkipExistsMessage             = "PLAN-SKIP (target exists): %s\n"
	planCaseOnlyMessage               = "PLAN-CASE-ONLY: %s → %s (two-step move required)\n"
	planReadyMessage                  = "PLAN-OK: %s → %s\n"
	errorAlreadyNamedMessage          = "ERROR: already named: %s\n"
	errorParentMissingMessage         = "ERROR: target parent missing: %s\n"
	errorParentNotDirectoryMessage    = "ERROR: target parent is not a directory: %s\n"
	errorTargetExistsMessage          = "ERROR: target exists: %s\n"
	promptTemplate                    = "Rename '%s' → '%s'? [a/N/y] "
	skipMessage                       = "SKIP: %s\n"
	skipDirtyMessage                  = "SKIP (dirty worktree): %s\n"
	successMessage                    = "Renamed %s → %s\n"
	failureMessage                    = "ERROR: rename failed for %s → %s\n"
	intermediateRenameTemplate        = "%s.rename.%d"
	parentDirectoryPermissionConstant = fs.FileMode(0o755)
)

// Options configures a rename execution.
type Options struct {
	RepositoryPath          string
	DesiredFolderName       string
	DryRun                  bool
	RequireCleanWorktree    bool
	AssumeYes               bool
	IncludeOwner            bool
	EnsureParentDirectories bool
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
		executor.printPlan(executionContext, oldAbsolutePath, newAbsolutePath, options.RequireCleanWorktree, options.EnsureParentDirectories)
		return
	}

	if !executor.validatePrerequisites(executionContext, oldAbsolutePath, newAbsolutePath, options.RequireCleanWorktree, options.EnsureParentDirectories) {
		return
	}

	if !options.AssumeYes && executor.dependencies.Prompter != nil {
		prompt := fmt.Sprintf(promptTemplate, oldAbsolutePath, newAbsolutePath)
		confirmationResult, promptError := executor.dependencies.Prompter.Confirm(prompt)
		if promptError != nil {
			executor.printfError(failureMessage, oldAbsolutePath, newAbsolutePath)
			return
		}
		if !confirmationResult.Confirmed {
			executor.printfOutput(skipMessage, oldAbsolutePath)
			return
		}
	}

	if !executor.ensureParentDirectory(newAbsolutePath, options.EnsureParentDirectories) {
		executor.printfError(failureMessage, oldAbsolutePath, newAbsolutePath)
		return
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

func (executor *Executor) printPlan(executionContext context.Context, oldAbsolutePath string, newAbsolutePath string, requireClean bool, ensureParentDirectories bool) {
	caseOnlyRename := isCaseOnlyRename(oldAbsolutePath, newAbsolutePath)
	parentDetails := executor.parentDirectoryDetails(newAbsolutePath)

	switch {
	case oldAbsolutePath == newAbsolutePath:
		executor.printfOutput(planSkipAlreadyMessage, oldAbsolutePath)
		return
	case requireClean && !executor.isClean(executionContext, oldAbsolutePath):
		executor.printfOutput(planSkipDirtyMessage, oldAbsolutePath)
		return
	case parentDetails.exists && !parentDetails.isDirectory:
		executor.printfOutput(planSkipParentNotDirectoryMessage, parentDetails.path)
		return
	case !ensureParentDirectories && !parentDetails.exists:
		executor.printfOutput(planSkipParentMissingMessage, parentDetails.path)
		return
	case executor.targetExists(newAbsolutePath) && !caseOnlyRename:
		executor.printfOutput(planSkipExistsMessage, newAbsolutePath)
		return
	}

	if caseOnlyRename {
		executor.printfOutput(planCaseOnlyMessage, oldAbsolutePath, newAbsolutePath)
		return
	}

	executor.printfOutput(planReadyMessage, oldAbsolutePath, newAbsolutePath)
}

func (executor *Executor) validatePrerequisites(executionContext context.Context, oldAbsolutePath string, newAbsolutePath string, requireClean bool, ensureParentDirectories bool) bool {
	caseOnlyRename := isCaseOnlyRename(oldAbsolutePath, newAbsolutePath)
	parentDetails := executor.parentDirectoryDetails(newAbsolutePath)

	if oldAbsolutePath == newAbsolutePath {
		executor.printfError(errorAlreadyNamedMessage, oldAbsolutePath)
		return false
	}

	if requireClean && !executor.isClean(executionContext, oldAbsolutePath) {
		executor.printfOutput(skipDirtyMessage, oldAbsolutePath)
		return false
	}

	if parentDetails.exists && !parentDetails.isDirectory {
		executor.printfError(errorParentNotDirectoryMessage, parentDetails.path)
		return false
	}

	if !ensureParentDirectories && !parentDetails.exists {
		executor.printfError(errorParentMissingMessage, parentDetails.path)
		return false
	}

	if executor.targetExists(newAbsolutePath) && !caseOnlyRename {
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

func (executor *Executor) parentDirectoryDetails(path string) parentDirectoryInformation {
	parentPath := filepath.Dir(path)
	details := parentDirectoryInformation{path: parentPath}

	if executor.dependencies.FileSystem == nil {
		return details
	}

	info, statError := executor.dependencies.FileSystem.Stat(parentPath)
	if statError != nil {
		return details
	}

	details.exists = true
	details.isDirectory = info.IsDir()
	return details
}

func (executor *Executor) targetExists(path string) bool {
	if executor.dependencies.FileSystem == nil {
		return false
	}
	_, statError := executor.dependencies.FileSystem.Stat(path)
	return statError == nil
}

func (executor *Executor) ensureParentDirectory(newAbsolutePath string, ensureParentDirectories bool) bool {
	if !ensureParentDirectories {
		return true
	}

	parentDetails := executor.parentDirectoryDetails(newAbsolutePath)
	if parentDetails.exists {
		return parentDetails.isDirectory
	}

	if executor.dependencies.FileSystem == nil {
		return false
	}

	creationError := executor.dependencies.FileSystem.MkdirAll(parentDetails.path, parentDirectoryPermissionConstant)
	return creationError == nil
}

func (executor *Executor) performRename(oldAbsolutePath string, newAbsolutePath string) bool {
	if executor.dependencies.FileSystem == nil {
		return false
	}

	if executor.dependencies.Clock == nil {
		executor.dependencies.Clock = shared.SystemClock{}
	}

	if executor.dependencies.GitManager == nil {
		return false
	}

	renameError := executor.dependencies.FileSystem.Rename(oldAbsolutePath, newAbsolutePath)
	if renameError == nil {
		return true
	}

	for attempt := 0; attempt < 5; attempt++ {
		intermediate := fmt.Sprintf(intermediateRenameTemplate, oldAbsolutePath, attempt)
		if renameError = executor.dependencies.FileSystem.Rename(oldAbsolutePath, intermediate); renameError != nil {
			continue
		}
		if renameError = executor.dependencies.FileSystem.Rename(intermediate, newAbsolutePath); renameError == nil {
			return true
		}
	}

	return false
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

// parentDirectoryInformation describes the state of a parent directory for rename planning.
type parentDirectoryInformation struct {
	path        string
	exists      bool
	isDirectory bool
}
