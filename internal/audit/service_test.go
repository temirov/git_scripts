package audit_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

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
	return nil
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

func TestServiceRunBehaviors(testInstance *testing.T) {
	testCases := []struct {
		name            string
		options         audit.CommandOptions
		discoverer      audit.RepositoryDiscoverer
		executorOutputs map[string]execshell.ExecutionResult
		gitManager      audit.GitRepositoryManager
		githubResolver  audit.GitHubMetadataResolver
		expectedOutput  string
		expectedError   string
	}{
		{
			name: "audit_report",
			options: audit.CommandOptions{
				Roots: []string{"/tmp/example"},
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
			expectedOutput: "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical\ncanonical/example,example,yes,main,main,n/a,https,no\n",
			expectedError:  "",
		},
		{
			name: "audit_debug",
			options: audit.CommandOptions{
				DebugOutput: true,
				Roots:       []string{"/tmp/example"},
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
			expectedOutput: "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical\ncanonical/example,example,yes,main,main,n/a,https,no\n",
			expectedError:  "DEBUG: discovered 1 candidate repos under: /tmp/example\nDEBUG: checking /tmp/example\n",
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
				outputBuffer,
				errorBuffer,
			)

			runError := service.Run(context.Background(), testCase.options)
			require.NoError(testInstance, runError)
			require.Equal(testInstance, testCase.expectedOutput, outputBuffer.String())
			require.Equal(testInstance, testCase.expectedError, errorBuffer.String())
		})
	}
}
