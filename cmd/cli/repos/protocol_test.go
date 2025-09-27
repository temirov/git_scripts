package repos_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	repos "github.com/temirov/git_scripts/cmd/cli/repos"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	protocolFromFlagConstant       = "--from"
	protocolToFlagConstant         = "--to"
	protocolYesFlagConstant        = "--yes"
	protocolDryRunFlagConstant     = "--dry-run"
	protocolConfiguredRootConstant = "/tmp/protocol-config-root"
	protocolSSHRemoteURL           = "ssh://git@github.com/origin/example.git"
	protocolHTTPSRemoteURL         = "https://github.com/origin/example.git"
	protocolMissingRootsMessage    = "no repository roots provided; specify --root or configure defaults"
)

func TestProtocolCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                    string
		configuration           repos.ProtocolConfiguration
		arguments               []string
		initialRemoteURL        string
		expectedRoots           []string
		expectRemoteUpdates     int
		expectPromptInvocations int
		expectError             bool
		expectedErrorMessage    string
	}{
		{
			name: "error_when_roots_missing",
			configuration: repos.ProtocolConfiguration{
				FromProtocol: string(shared.RemoteProtocolHTTPS),
				ToProtocol:   string(shared.RemoteProtocolSSH),
			},
			arguments:            []string{},
			initialRemoteURL:     protocolHTTPSRemoteURL,
			expectError:          true,
			expectedErrorMessage: protocolMissingRootsMessage,
		},
		{
			name: "configuration_supplies_protocols",
			configuration: repos.ProtocolConfiguration{
				DryRun:          true,
				AssumeYes:       false,
				RepositoryRoots: []string{protocolConfiguredRootConstant},
				FromProtocol:    string(shared.RemoteProtocolHTTPS),
				ToProtocol:      string(shared.RemoteProtocolSSH),
			},
			arguments:               []string{},
			initialRemoteURL:        protocolHTTPSRemoteURL,
			expectedRoots:           []string{protocolConfiguredRootConstant},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name: "flags_override_configuration",
			configuration: repos.ProtocolConfiguration{
				DryRun:          false,
				AssumeYes:       false,
				RepositoryRoots: []string{protocolConfiguredRootConstant},
				FromProtocol:    string(shared.RemoteProtocolSSH),
				ToProtocol:      string(shared.RemoteProtocolHTTPS),
			},
			arguments: []string{
				protocolFromFlagConstant,
				string(shared.RemoteProtocolHTTPS),
				protocolToFlagConstant,
				string(shared.RemoteProtocolSSH),
				protocolYesFlagConstant,
				protocolDryRunFlagConstant,
				remotesCLIRepositoryRootConstant,
			},
			initialRemoteURL:        protocolHTTPSRemoteURL,
			expectedRoots:           []string{remotesCLIRepositoryRootConstant},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name: "configuration_triggers_remote_update",
			configuration: repos.ProtocolConfiguration{
				DryRun:          false,
				AssumeYes:       true,
				RepositoryRoots: []string{protocolConfiguredRootConstant},
				FromProtocol:    string(shared.RemoteProtocolHTTPS),
				ToProtocol:      string(shared.RemoteProtocolSSH),
			},
			arguments:               []string{},
			initialRemoteURL:        protocolHTTPSRemoteURL,
			expectedRoots:           []string{protocolConfiguredRootConstant},
			expectRemoteUpdates:     1,
			expectPromptInvocations: 0,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			discoverer := &fakeRepositoryDiscoverer{repositories: []string{remotesDiscoveredRepository}}
			executor := &fakeGitExecutor{}
			manager := &fakeGitRepositoryManager{remoteURL: testCase.initialRemoteURL, currentBranch: remotesMetadataDefaultBranch}
			resolver := &fakeGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: remotesCanonicalRepository, DefaultBranch: remotesMetadataDefaultBranch}}
			prompter := &recordingPrompter{confirmResult: true}

			builder := repos.ProtocolCommandBuilder{
				LoggerProvider: func() *zap.Logger { return zap.NewNop() },
				Discoverer:     discoverer,
				GitExecutor:    executor,
				GitManager:     manager,
				GitHubResolver: resolver,
				PrompterFactory: func(*cobra.Command) shared.ConfirmationPrompter {
					return prompter
				},
				ConfigurationProvider: func() repos.ProtocolConfiguration {
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

			require.Equal(subtest, testCase.expectedRoots, discoverer.receivedRoots)
			require.Equal(subtest, testCase.expectPromptInvocations, prompter.calls)
			require.Equal(subtest, testCase.expectRemoteUpdates, len(manager.setCalls))
			if testCase.expectRemoteUpdates > 0 {
				require.NotEmpty(subtest, manager.setCalls)
				require.Equal(subtest, string(shared.RemoteProtocolSSH), detectProtocol(manager.setCalls[len(manager.setCalls)-1].remoteURL))
			}
		})
	}
}

func detectProtocol(remoteURL string) string {
	switch {
	case len(remoteURL) == 0:
		return ""
	case strings.HasPrefix(remoteURL, shared.SSHProtocolURLPrefixConstant):
		return string(shared.RemoteProtocolSSH)
	case strings.HasPrefix(remoteURL, shared.HTTPSProtocolURLPrefixConstant):
		return string(shared.RemoteProtocolHTTPS)
	default:
		return ""
	}
}
