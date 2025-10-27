package rename_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/repos/rename"
	"github.com/temirov/gix/internal/repos/shared"
)

type stubFileSystem struct {
	existingPaths      map[string]bool
	renamedPairs       [][2]string
	createdDirectories []string
	absBase            string
	absError           error
	renameError        error
	fileContents       map[string][]byte
}

func (fileSystem *stubFileSystem) Stat(path string) (fs.FileInfo, error) {
	if fileSystem.existingPaths == nil {
		fileSystem.existingPaths = map[string]bool{}
	}
	if fileSystem.existingPaths[path] {
		return stubFileInfo{}, nil
	}
	return nil, errors.New("not exists")
}

func (fileSystem *stubFileSystem) Rename(oldPath string, newPath string) error {
	if fileSystem.renameError != nil {
		return fileSystem.renameError
	}
	fileSystem.renamedPairs = append(fileSystem.renamedPairs, [2]string{oldPath, newPath})
	if fileSystem.existingPaths == nil {
		fileSystem.existingPaths = map[string]bool{}
	}
	fileSystem.existingPaths[newPath] = true
	delete(fileSystem.existingPaths, oldPath)
	return nil
}

func (fileSystem *stubFileSystem) Abs(path string) (string, error) {
	if fileSystem.absError != nil {
		return "", fileSystem.absError
	}
	if len(fileSystem.absBase) == 0 {
		return path, nil
	}
	return filepath.Join(fileSystem.absBase, filepath.Base(path)), nil
}

func (fileSystem *stubFileSystem) MkdirAll(path string, permissions fs.FileMode) error {
	if fileSystem.existingPaths == nil {
		fileSystem.existingPaths = map[string]bool{}
	}
	fileSystem.createdDirectories = append(fileSystem.createdDirectories, path)
	fileSystem.existingPaths[path] = true
	return nil
}

func (fileSystem *stubFileSystem) ReadFile(path string) ([]byte, error) {
	if fileSystem.fileContents == nil {
		fileSystem.fileContents = map[string][]byte{}
	}
	if contents, exists := fileSystem.fileContents[path]; exists {
		duplicate := make([]byte, len(contents))
		copy(duplicate, contents)
		return duplicate, nil
	}
	return nil, errors.New("not exists")
}

func (fileSystem *stubFileSystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	if fileSystem.existingPaths == nil {
		fileSystem.existingPaths = map[string]bool{}
	}
	if fileSystem.fileContents == nil {
		fileSystem.fileContents = map[string][]byte{}
	}
	duplicate := make([]byte, len(data))
	copy(duplicate, data)
	fileSystem.fileContents[path] = duplicate
	fileSystem.existingPaths[path] = true
	return nil
}

type stubFileInfo struct{}

func (stubFileInfo) Name() string       { return "" }
func (stubFileInfo) Size() int64        { return 0 }
func (stubFileInfo) Mode() fs.FileMode  { return 0 }
func (stubFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (stubFileInfo) IsDir() bool        { return true }
func (stubFileInfo) Sys() any           { return nil }

type stubGitManager struct {
	clean bool
}

func (manager stubGitManager) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
	return manager.clean, nil
}

func (manager stubGitManager) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
	return "", nil
}

func (manager stubGitManager) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
	return "", nil
}

func (manager stubGitManager) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
	return nil
}

type stubPrompter struct {
	result          shared.ConfirmationResult
	callError       error
	recordedPrompts []string
}

func (prompter *stubPrompter) Confirm(prompt string) (shared.ConfirmationResult, error) {
	if prompter == nil {
		return shared.ConfirmationResult{}, nil
	}
	prompter.recordedPrompts = append(prompter.recordedPrompts, prompt)
	if prompter.callError != nil {
		return shared.ConfirmationResult{}, prompter.callError
	}
	return prompter.result, nil
}

type stubClock struct{}

func (stubClock) Now() time.Time { return time.Unix(0, 0) }

const (
	renameTestRootDirectory          = "/tmp"
	renameTestLegacyFolderPath       = "/tmp/legacy"
	renameTestProjectFolderPath      = "/tmp/project"
	renameTestTargetFolderPath       = "/tmp/example"
	renameTestDesiredFolderName      = "example"
	renameTestOwnerSegment           = "owner"
	renameTestOwnerDesiredFolderName = "owner/example"
	renameTestOwnerDirectoryPath     = "/tmp/owner"
)

func TestExecutorBehaviors(testInstance *testing.T) {
	testCases := []struct {
		name                       string
		options                    rename.Options
		fileSystem                 *stubFileSystem
		gitManager                 shared.GitRepositoryManager
		prompter                   shared.ConfirmationPrompter
		expectedOutput             string
		expectedErrors             string
		expectedRenames            int
		expectedCreatedDirectories []string
	}{
		{
			name: "dry_run_plan_ready",
			options: rename.Options{
				RepositoryPath:       renameTestLegacyFolderPath,
				DesiredFolderName:    renameTestDesiredFolderName,
				DryRun:               true,
				RequireCleanWorktree: true,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:    true,
					renameTestTargetFolderPath: false,
				},
			},
			gitManager:      stubGitManager{clean: true},
			expectedOutput:  fmt.Sprintf("PLAN-OK: %s → %s\n", renameTestLegacyFolderPath, renameTestTargetFolderPath),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "dry_run_missing_parent_without_creation",
			options: rename.Options{
				RepositoryPath:          renameTestLegacyFolderPath,
				DesiredFolderName:       renameTestOwnerDesiredFolderName,
				DryRun:                  true,
				RequireCleanWorktree:    true,
				EnsureParentDirectories: false,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			expectedOutput:  fmt.Sprintf("PLAN-SKIP (target parent missing): %s\n", renameTestOwnerDirectoryPath),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "dry_run_missing_parent_with_creation",
			options: rename.Options{
				RepositoryPath:          renameTestLegacyFolderPath,
				DesiredFolderName:       renameTestOwnerDesiredFolderName,
				DryRun:                  true,
				RequireCleanWorktree:    true,
				EnsureParentDirectories: true,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			expectedOutput:  fmt.Sprintf("PLAN-OK: %s → %s\n", renameTestLegacyFolderPath, filepath.Join(renameTestRootDirectory, renameTestOwnerDesiredFolderName)),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "prompter_declines",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: renameTestDesiredFolderName,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
					renameTestTargetFolderPath:  false,
				},
			},
			gitManager:      stubGitManager{clean: true},
			prompter:        &stubPrompter{result: shared.ConfirmationResult{Confirmed: false}},
			expectedOutput:  fmt.Sprintf("SKIP: %s\n", renameTestProjectFolderPath),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "prompter_accepts_once",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: renameTestDesiredFolderName,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			prompter:        &stubPrompter{result: shared.ConfirmationResult{Confirmed: true}},
			expectedOutput:  fmt.Sprintf("Renamed %s → %s\n", renameTestProjectFolderPath, renameTestTargetFolderPath),
			expectedErrors:  "",
			expectedRenames: 1,
		},
		{
			name: "prompter_accepts_all",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: renameTestDesiredFolderName,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			prompter:        &stubPrompter{result: shared.ConfirmationResult{Confirmed: true, ApplyToAll: true}},
			expectedOutput:  fmt.Sprintf("Renamed %s → %s\n", renameTestProjectFolderPath, renameTestTargetFolderPath),
			expectedErrors:  "",
			expectedRenames: 1,
		},
		{
			name: "prompter_error",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: renameTestDesiredFolderName,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			prompter:        &stubPrompter{callError: errors.New("read failure")},
			expectedOutput:  "",
			expectedErrors:  fmt.Sprintf("ERROR: rename failed for %s → %s\n", renameTestProjectFolderPath, renameTestTargetFolderPath),
			expectedRenames: 0,
		},
		{
			name: "assume_yes_skips_prompt",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: renameTestDesiredFolderName,
				AssumeYes:         true,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			expectedOutput:  fmt.Sprintf("Renamed %s → %s\n", renameTestProjectFolderPath, renameTestTargetFolderPath),
			expectedErrors:  "",
			expectedRenames: 1,
		},
		{
			name: "skip_dirty_worktree",
			options: rename.Options{
				RepositoryPath:       renameTestProjectFolderPath,
				DesiredFolderName:    renameTestDesiredFolderName,
				RequireCleanWorktree: true,
				AssumeYes:            true,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: false},
			expectedOutput:  fmt.Sprintf("SKIP (dirty worktree): %s\n", renameTestProjectFolderPath),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "already_normalized_skip",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: filepath.Base(renameTestProjectFolderPath),
				AssumeYes:         true,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			expectedOutput:  fmt.Sprintf("SKIP (already normalized): %s\n", renameTestProjectFolderPath),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "execute_missing_parent_without_creation",
			options: rename.Options{
				RepositoryPath:          renameTestProjectFolderPath,
				DesiredFolderName:       renameTestOwnerDesiredFolderName,
				AssumeYes:               true,
				EnsureParentDirectories: false,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:      stubGitManager{clean: true},
			expectedOutput:  "",
			expectedErrors:  fmt.Sprintf("ERROR: target parent missing: %s\n", renameTestOwnerDirectoryPath),
			expectedRenames: 0,
		},
		{
			name: "execute_missing_parent_with_creation",
			options: rename.Options{
				RepositoryPath:          renameTestProjectFolderPath,
				DesiredFolderName:       renameTestOwnerDesiredFolderName,
				AssumeYes:               true,
				EnsureParentDirectories: true,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:     true,
					renameTestProjectFolderPath: true,
				},
			},
			gitManager:                 stubGitManager{clean: true},
			expectedOutput:             fmt.Sprintf("Renamed %s → %s\n", renameTestProjectFolderPath, filepath.Join(renameTestRootDirectory, renameTestOwnerDesiredFolderName)),
			expectedErrors:             "",
			expectedRenames:            1,
			expectedCreatedDirectories: []string{renameTestOwnerDirectoryPath},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testingInstance *testing.T) {
			outputBuffer := &bytes.Buffer{}
			errorBuffer := &bytes.Buffer{}
			executor := rename.NewExecutor(rename.Dependencies{
				FileSystem: testCase.fileSystem,
				GitManager: testCase.gitManager,
				Prompter:   testCase.prompter,
				Clock:      stubClock{},
				Output:     outputBuffer,
				Errors:     errorBuffer,
			})

			executor.Execute(context.Background(), testCase.options)
			require.Equal(testingInstance, testCase.expectedOutput, outputBuffer.String())
			require.Equal(testingInstance, testCase.expectedErrors, errorBuffer.String())
			require.Len(testingInstance, testCase.fileSystem.renamedPairs, testCase.expectedRenames)
			require.Equal(testingInstance, testCase.expectedCreatedDirectories, testCase.fileSystem.createdDirectories)
		})
	}
}

func TestExecutorPromptsAdvertiseApplyAll(testInstance *testing.T) {
	commandPrompter := &stubPrompter{}
	fileSystem := &stubFileSystem{existingPaths: map[string]bool{
		renameTestRootDirectory:     true,
		renameTestProjectFolderPath: true,
		renameTestTargetFolderPath:  false,
	}}
	dependencies := rename.Dependencies{
		FileSystem: fileSystem,
		GitManager: stubGitManager{clean: true},
		Prompter:   commandPrompter,
		Output:     &bytes.Buffer{},
		Errors:     &bytes.Buffer{},
	}
	renamer := rename.NewExecutor(dependencies)
	renamer.Execute(context.Background(), rename.Options{RepositoryPath: renameTestProjectFolderPath, DesiredFolderName: renameTestDesiredFolderName})
	require.Equal(testInstance, []string{fmt.Sprintf("Rename '%s' → '%s'? [a/N/y] ", renameTestProjectFolderPath, renameTestTargetFolderPath)}, commandPrompter.recordedPrompts)
}
