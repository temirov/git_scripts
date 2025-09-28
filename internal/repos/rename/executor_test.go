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
	existingPaths map[string]bool
	renamedPairs  [][2]string
	absBase       string
	absError      error
	renameError   error
}

func (fileSystem *stubFileSystem) Stat(path string) (fs.FileInfo, error) {
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
	response bool
	err      error
}

func (prompter stubPrompter) Confirm(prompt string) (bool, error) {
	if prompter.err != nil {
		return false, prompter.err
	}
	return prompter.response, nil
}

type stubClock struct{}

func (stubClock) Now() time.Time { return time.Unix(0, 0) }

const (
	renameTestRootDirectory     = "/tmp"
	renameTestLegacyFolderPath  = "/tmp/legacy"
	renameTestProjectFolderPath = "/tmp/project"
	renameTestTargetFolderPath  = "/tmp/example"
	renameTestDesiredFolderName = "example"
)

func TestExecutorBehaviors(testInstance *testing.T) {
	testCases := []struct {
		name            string
		options         rename.Options
		fileSystem      *stubFileSystem
		gitManager      shared.GitRepositoryManager
		prompter        shared.ConfirmationPrompter
		expectedOutput  string
		expectedErrors  string
		expectedRenames int
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
			name: "prompter_declines",
			options: rename.Options{
				RepositoryPath:    renameTestProjectFolderPath,
				DesiredFolderName: renameTestDesiredFolderName,
			},
			fileSystem: &stubFileSystem{
				existingPaths: map[string]bool{
					renameTestRootDirectory:    true,
					renameTestTargetFolderPath: false,
				},
			},
			gitManager:      stubGitManager{clean: true},
			prompter:        stubPrompter{response: false},
			expectedOutput:  fmt.Sprintf("SKIP: %s\n", renameTestProjectFolderPath),
			expectedErrors:  "",
			expectedRenames: 0,
		},
		{
			name: "rename_success",
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
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
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
			require.Equal(testInstance, testCase.expectedOutput, outputBuffer.String())
			require.Equal(testInstance, testCase.expectedErrors, errorBuffer.String())
			require.Len(testInstance, testCase.fileSystem.renamedPairs, testCase.expectedRenames)
		})
	}
}
