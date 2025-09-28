package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/temirov/gix/internal/repos/rename"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	renameRefreshErrorTemplateConstant = "failed to refresh repository after rename: %w"
)

// RenameOperation normalizes repository directory names to match canonical GitHub names.
type RenameOperation struct {
	RequireCleanWorktree bool
}

// Name identifies the operation type.
func (operation *RenameOperation) Name() string {
	return string(OperationTypeRenameDirectories)
}

// Execute applies rename operations for repositories with desired folder names.
func (operation *RenameOperation) Execute(executionContext context.Context, environment *Environment, state *State) error {
	if environment == nil || state == nil {
		return nil
	}

	dependencies := rename.Dependencies{
		FileSystem: environment.FileSystem,
		GitManager: environment.RepositoryManager,
		Prompter:   environment.Prompter,
		Clock:      shared.SystemClock{},
		Output:     environment.Output,
		Errors:     environment.Errors,
	}

	for repositoryIndex := range state.Repositories {
		repository := state.Repositories[repositoryIndex]
		desiredName := strings.TrimSpace(repository.Inspection.DesiredFolderName)
		if len(desiredName) == 0 {
			continue
		}

		originalPath := repository.Path

		options := rename.Options{
			RepositoryPath:       originalPath,
			DesiredFolderName:    desiredName,
			DryRun:               environment.DryRun,
			RequireCleanWorktree: operation.RequireCleanWorktree,
			AssumeYes:            environment.AssumeYes,
		}

		rename.Execute(executionContext, dependencies, options)

		if environment.DryRun {
			continue
		}

		newPath := filepath.Join(filepath.Dir(originalPath), desiredName)
		if !renameCompleted(environment.FileSystem, originalPath, newPath) {
			continue
		}

		if updateError := state.UpdateRepositoryPath(repositoryIndex, newPath); updateError != nil {
			return updateError
		}

		if refreshError := repository.Refresh(executionContext, environment.AuditService); refreshError != nil {
			return fmt.Errorf(renameRefreshErrorTemplateConstant, refreshError)
		}
	}

	return nil
}

func renameCompleted(fileSystem shared.FileSystem, originalPath string, newPath string) bool {
	if fileSystem == nil {
		return false
	}

	newInfo, newStatError := fileSystem.Stat(newPath)
	if newStatError != nil {
		return false
	}

	originalInfo, originalStatError := fileSystem.Stat(originalPath)
	if originalStatError != nil {
		return true
	}

	return os.SameFile(originalInfo, newInfo)
}
