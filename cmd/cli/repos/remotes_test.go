package repos_test

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	repos "github.com/temirov/git_scripts/cmd/cli/repos"
	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	remotesAssumeYesFlagConstant     = "--yes"
	remotesDryRunFlagConstant        = "--dry-run"
	remotesConfiguredRootConstant    = "/tmp/remotes-config-root"
	remotesCLIRepositoryRootConstant = "/tmp/remotes-cli-root"
	remotesDiscoveredRepository      = "/tmp/remotes-repo"
	remotesOriginURLConstant         = "https://github.com/origin/example.git"
	remotesCanonicalRepository       = "canonical/example"
	remotesMetadataDefaultBranch     = "main"
)

func TestRemotesCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                    string
		configuration           repos.RemotesConfiguration
		arguments               []string
		expectedRoots           []string
		expectRemoteUpdates     int
		expectPromptInvocations int
	}{
		{
			name: "configuration_enables_dry_run",
			configuration: repos.RemotesConfiguration{
				DryRun:          true,
				AssumeYes:       false,
				RepositoryRoots: []string{remotesConfiguredRootConstant},
			},
			arguments:               []string{},
			expectedRoots:           []string{remotesConfiguredRootConstant},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name: "flags_override_configuration",
			configuration: repos.RemotesConfiguration{
				DryRun:          false,
				AssumeYes:       false,
				RepositoryRoots: []string{remotesConfiguredRootConstant},
			},
			arguments: []string{
				remotesAssumeYesFlagConstant,
				remotesDryRunFlagConstant,
				remotesCLIRepositoryRootConstant,
			},
			expectedRoots:           []string{remotesCLIRepositoryRootConstant},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name:                    "defaults_apply_without_configuration",
			configuration:           repos.RemotesConfiguration{},
			arguments:               []string{},
			expectedRoots:           []string{"."},
			expectRemoteUpdates:     1,
			expectPromptInvocations: 1,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			discoverer := &fakeRepositoryDiscoverer{repositories: []string{remotesDiscoveredRepository}}
			executor := &fakeGitExecutor{}
			manager := &fakeGitRepositoryManager{remoteURL: remotesOriginURLConstant, currentBranch: remotesMetadataDefaultBranch}
			resolver := &fakeGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: remotesCanonicalRepository, DefaultBranch: remotesMetadataDefaultBranch}}
			prompter := &recordingPrompter{confirmResult: true}

			builder := repos.RemotesCommandBuilder{
				LoggerProvider: func() *zap.Logger { return zap.NewNop() },
				Discoverer:     discoverer,
				GitExecutor:    executor,
				GitManager:     manager,
				GitHubResolver: resolver,
				PrompterFactory: func(*cobra.Command) shared.ConfirmationPrompter {
					return prompter
				},
				ConfigurationProvider: func() repos.RemotesConfiguration {
					return testCase.configuration
				},
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)

			command.SetContext(context.Background())
			command.SetArgs(testCase.arguments)

			executionError := command.Execute()
			require.NoError(subtest, executionError)

			require.Equal(subtest, testCase.expectedRoots, discoverer.receivedRoots)
			require.Equal(subtest, testCase.expectPromptInvocations, prompter.calls)
			require.Equal(subtest, testCase.expectRemoteUpdates, len(manager.setCalls))
		})
	}
}

type fakeRepositoryDiscoverer struct {
	repositories  []string
	receivedRoots []string
}

func (discoverer *fakeRepositoryDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	discoverer.receivedRoots = append([]string{}, roots...)
	return append([]string{}, discoverer.repositories...), nil
}

type fakeGitExecutor struct{}

func (executor *fakeGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	if len(details.Arguments) > 0 && details.Arguments[0] == "rev-parse" {
		return execshell.ExecutionResult{StandardOutput: "true\n"}, nil
	}
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}

func (executor *fakeGitExecutor) ExecuteGitHubCLI(_ context.Context, _ execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}

type fakeGitRepositoryManager struct {
	remoteURL     string
	currentBranch string
	setCalls      []remoteUpdateCall
}

type remoteUpdateCall struct {
	repositoryPath string
	remoteURL      string
}

func (manager *fakeGitRepositoryManager) CheckCleanWorktree(context.Context, string) (bool, error) {
	return true, nil
}

func (manager *fakeGitRepositoryManager) GetCurrentBranch(context.Context, string) (string, error) {
	return manager.currentBranch, nil
}

func (manager *fakeGitRepositoryManager) GetRemoteURL(context.Context, string, string) (string, error) {
	return manager.remoteURL, nil
}

func (manager *fakeGitRepositoryManager) SetRemoteURL(_ context.Context, repositoryPath string, _ string, remoteURL string) error {
	manager.setCalls = append(manager.setCalls, remoteUpdateCall{repositoryPath: repositoryPath, remoteURL: remoteURL})
	manager.remoteURL = remoteURL
	return nil
}

type fakeGitHubResolver struct {
	metadata githubcli.RepositoryMetadata
}

func (resolver *fakeGitHubResolver) ResolveRepoMetadata(context.Context, string) (githubcli.RepositoryMetadata, error) {
	return resolver.metadata, nil
}

type recordingPrompter struct {
	confirmResult bool
	calls         int
}

func (prompter *recordingPrompter) Confirm(string) (bool, error) {
	prompter.calls++
	return prompter.confirmResult, nil
}
