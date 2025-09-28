package repos_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	repos "github.com/temirov/gix/cmd/cli/repos"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/repos/shared"
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
	remotesMissingRootsMessage       = "no repository roots provided; specify --root or configure defaults"
	remotesRelativeRootConstant      = "relative/remotes-root"
	remotesHomeRootSuffixConstant    = "remotes-home-root"
)

func TestRemotesCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                    string
		configuration           repos.RemotesConfiguration
		arguments               []string
		expectedRoots           []string
		expectedRootsBuilder    func(testing.TB) []string
		expectRemoteUpdates     int
		expectPromptInvocations int
		expectError             bool
		expectedErrorMessage    string
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
			name:                 "error_when_roots_missing",
			configuration:        repos.RemotesConfiguration{},
			arguments:            []string{},
			expectError:          true,
			expectedErrorMessage: remotesMissingRootsMessage,
		},
		{
			name: "configuration_expands_home_relative_root",
			configuration: repos.RemotesConfiguration{
				DryRun:          true,
				AssumeYes:       true,
				RepositoryRoots: []string{"~/" + remotesHomeRootSuffixConstant},
			},
			arguments: []string{},
			expectedRootsBuilder: func(testingInstance testing.TB) []string {
				homeDirectory, homeError := os.UserHomeDir()
				require.NoError(testingInstance, homeError)
				expandedRoot := filepath.Join(homeDirectory, remotesHomeRootSuffixConstant)
				return []string{expandedRoot}
			},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name: "arguments_preserve_relative_roots",
			configuration: repos.RemotesConfiguration{
				DryRun:          false,
				AssumeYes:       false,
				RepositoryRoots: nil,
			},
			arguments: []string{
				remotesAssumeYesFlagConstant,
				remotesDryRunFlagConstant,
				remotesRelativeRootConstant,
			},
			expectedRoots:           []string{remotesRelativeRootConstant},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name:          "arguments_expand_home_relative_root",
			configuration: repos.RemotesConfiguration{},
			arguments: []string{
				remotesAssumeYesFlagConstant,
				remotesDryRunFlagConstant,
				"~/" + remotesHomeRootSuffixConstant,
			},
			expectedRootsBuilder: func(testingInstance testing.TB) []string {
				homeDirectory, homeError := os.UserHomeDir()
				require.NoError(testingInstance, homeError)
				expandedRoot := filepath.Join(homeDirectory, remotesHomeRootSuffixConstant)
				return []string{expandedRoot}
			},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
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
			stdoutBuffer := &bytes.Buffer{}
			stderrBuffer := &bytes.Buffer{}
			command.SetOut(stdoutBuffer)
			command.SetErr(stderrBuffer)
			command.SetArgs(testCase.arguments)

			executionError := command.Execute()
			if testCase.expectError {
				require.Error(subtest, executionError)
				require.Equal(subtest, testCase.expectedErrorMessage, executionError.Error())
				combinedOutput := stdoutBuffer.String() + stderrBuffer.String()
				require.Contains(subtest, combinedOutput, command.UseLine())
				require.Empty(subtest, discoverer.receivedRoots)
				require.Zero(subtest, prompter.calls)
				require.Zero(subtest, len(manager.setCalls))
				return
			}

			require.NoError(subtest, executionError)

			expectedRoots := testCase.expectedRoots
			if testCase.expectedRootsBuilder != nil {
				expectedRoots = testCase.expectedRootsBuilder(subtest)
			}
			require.Equal(subtest, expectedRoots, discoverer.receivedRoots)
			require.Equal(subtest, testCase.expectPromptInvocations, prompter.calls)
			require.Equal(subtest, testCase.expectRemoteUpdates, len(manager.setCalls))
		})
	}
}
