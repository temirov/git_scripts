package audit_test

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/internal/audit"
	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
)

type stubDiscoverer struct {
	repositories []string
}

func (discoverer stubDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	return discoverer.repositories, nil
}

type stubGitExecutor struct {
	outputs map[string]execshell.ExecutionResult
}

func (executor stubGitExecutor) ExecuteGit(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	key := strings.Join(details.Arguments, " ")
	if result, found := executor.outputs[key]; found {
		return result, nil
	}
	return execshell.ExecutionResult{}, fmt.Errorf("unexpected git command: %s", key)
}

func (executor stubGitExecutor) ExecuteGitHubCLI(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	key := strings.Join(details.Arguments, " ")
	return execshell.ExecutionResult{}, fmt.Errorf("unexpected github command: %s", key)
}

type stubGitManager struct {
	cleanWorktree bool
	branchName    string
	remoteURL     string
	setError      error
}

func (manager stubGitManager) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
	return manager.cleanWorktree, nil
}

func (manager stubGitManager) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
	return manager.branchName, nil
}

func (manager stubGitManager) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
	return manager.remoteURL, nil
}

func (manager stubGitManager) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
	return manager.setError
}

type stubGitHubResolver struct {
	metadata githubcli.RepositoryMetadata
	err      error
}

func (resolver stubGitHubResolver) ResolveRepoMetadata(ctx context.Context, repository string) (githubcli.RepositoryMetadata, error) {
	if resolver.err != nil {
		return githubcli.RepositoryMetadata{}, resolver.err
	}
	return resolver.metadata, nil
}

type stubFileSystem struct {
	existingPaths map[string]bool
	renameError   error
}

func (fileSystem stubFileSystem) Stat(path string) (fs.FileInfo, error) {
	if fileSystem.existingPaths[path] {
		return stubFileInfo{nameValue: filepath.Base(path)}, nil
	}
	return nil, os.ErrNotExist
}

func (fileSystem stubFileSystem) Rename(oldPath string, newPath string) error {
	return fileSystem.renameError
}

func (fileSystem stubFileSystem) Abs(path string) (string, error) {
	return path, nil
}

type stubFileInfo struct {
	nameValue string
}

func (info stubFileInfo) Name() string  { return info.nameValue }
func (stubFileInfo) Size() int64        { return 0 }
func (stubFileInfo) Mode() fs.FileMode  { return 0 }
func (stubFileInfo) ModTime() time.Time { return time.Time{} }
func (stubFileInfo) IsDir() bool        { return true }
func (stubFileInfo) Sys() any           { return nil }

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

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Unix(0, 0)
}

func TestServiceRunBehaviors(testInstance *testing.T) {
	testCases := []struct {
		name            string
		options         audit.CommandOptions
		discoverer      audit.RepositoryDiscoverer
		executorOutputs map[string]execshell.ExecutionResult
		gitManager      audit.GitRepositoryManager
		githubResolver  audit.GitHubMetadataResolver
		fileSystem      audit.FileSystem
		prompter        audit.ConfirmationPrompter
		expectedOutput  string
		expectedError   string
	}{
		{
			name: "audit_report",
			options: audit.CommandOptions{
				AuditReport: true,
				Clock:       fixedClock{},
			},
			discoverer: stubDiscoverer{repositories: []string{"/tmp/example"}},
			executorOutputs: map[string]execshell.ExecutionResult{
				"rev-parse --is-inside-work-tree": {StandardOutput: "true"},
			},
			gitManager: stubGitManager{
				cleanWorktree: true,
				branchName:    "main",
				remoteURL:     "https://github.com/origin/example.git",
			},
			githubResolver: stubGitHubResolver{
				metadata: githubcli.RepositoryMetadata{
					NameWithOwner: "canonical/example",
					DefaultBranch: "main",
				},
			},
			fileSystem:     stubFileSystem{existingPaths: map[string]bool{"/tmp": true, "/tmp/example": true}},
			prompter:       stubPrompter{response: true},
			expectedOutput: "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical\ncanonical/example,example,yes,main,main,n/a,https,no\n",
			expectedError:  "",
		},
		{
			name: "rename_dry_run",
			options: audit.CommandOptions{
				RenameRepositories:   true,
				DryRun:               true,
				RequireCleanWorktree: true,
				Clock:                fixedClock{},
			},
			discoverer: stubDiscoverer{repositories: []string{"/tmp/legacy"}},
			executorOutputs: map[string]execshell.ExecutionResult{
				"rev-parse --is-inside-work-tree": {StandardOutput: "true"},
			},
			gitManager: stubGitManager{
				cleanWorktree: true,
				branchName:    "main",
				remoteURL:     "https://github.com/origin/example.git",
			},
			githubResolver: stubGitHubResolver{
				metadata: githubcli.RepositoryMetadata{
					NameWithOwner: "origin/example",
					DefaultBranch: "main",
				},
			},
			fileSystem: stubFileSystem{
				existingPaths: map[string]bool{
					"/tmp":         true,
					"/tmp/example": false,
				},
			},
			prompter:       stubPrompter{response: true},
			expectedOutput: "PLAN-OK: /tmp/legacy → /tmp/example\n",
			expectedError:  "",
		},
		{
			name: "update_remote_dry_run",
			options: audit.CommandOptions{
				UpdateRemotes: true,
				DryRun:        true,
				Clock:         fixedClock{},
			},
			discoverer: stubDiscoverer{repositories: []string{"/tmp/example"}},
			executorOutputs: map[string]execshell.ExecutionResult{
				"rev-parse --is-inside-work-tree": {StandardOutput: "true"},
			},
			gitManager: stubGitManager{
				cleanWorktree: true,
				branchName:    "main",
				remoteURL:     "https://github.com/origin/example.git",
			},
			githubResolver: stubGitHubResolver{
				metadata: githubcli.RepositoryMetadata{
					NameWithOwner: "canonical/example",
					DefaultBranch: "main",
				},
			},
			fileSystem:     stubFileSystem{existingPaths: map[string]bool{"/tmp": true, "/tmp/example": true}},
			prompter:       stubPrompter{response: true},
			expectedOutput: "PLAN-UPDATE-REMOTE: /tmp/example origin https://github.com/origin/example.git → https://github.com/canonical/example.git\n",
			expectedError:  "",
		},
		{
			name: "protocol_convert_dry_run",
			options: audit.CommandOptions{
				DryRun:       true,
				ProtocolFrom: audit.RemoteProtocolHTTPS,
				ProtocolTo:   audit.RemoteProtocolSSH,
				Clock:        fixedClock{},
			},
			discoverer: stubDiscoverer{repositories: []string{"/tmp/example"}},
			executorOutputs: map[string]execshell.ExecutionResult{
				"rev-parse --is-inside-work-tree": {StandardOutput: "true"},
			},
			gitManager: stubGitManager{
				cleanWorktree: true,
				branchName:    "main",
				remoteURL:     "https://github.com/origin/example.git",
			},
			githubResolver: stubGitHubResolver{
				metadata: githubcli.RepositoryMetadata{
					NameWithOwner: "origin/example",
					DefaultBranch: "main",
				},
			},
			fileSystem:     stubFileSystem{existingPaths: map[string]bool{"/tmp": true, "/tmp/example": true}},
			prompter:       stubPrompter{response: true},
			expectedOutput: "PLAN-CONVERT: /tmp/example origin https://github.com/origin/example.git → ssh://git@github.com/origin/example.git\n",
			expectedError:  "",
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf("%d_%s", testCaseIndex, testCase.name), func(testInstance *testing.T) {
			outputBuffer := &bytes.Buffer{}
			errorBuffer := &bytes.Buffer{}

			service := audit.NewService(
				testCase.discoverer,
				testCase.gitManager,
				stubGitExecutor{outputs: testCase.executorOutputs},
				testCase.githubResolver,
				testCase.fileSystem,
				testCase.prompter,
				outputBuffer,
				errorBuffer,
				testCase.options.Clock,
			)

			runError := service.Run(context.Background(), testCase.options)
			require.NoError(testInstance, runError)
			require.Equal(testInstance, testCase.expectedOutput, outputBuffer.String())
			require.Equal(testInstance, testCase.expectedError, errorBuffer.String())
		})
	}
}
