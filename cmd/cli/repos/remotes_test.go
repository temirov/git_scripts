package repos_test

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	repos "github.com/temirov/git_scripts/cmd/cli/repos"
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
