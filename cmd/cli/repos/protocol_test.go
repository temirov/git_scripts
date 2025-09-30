package repos_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	repos "github.com/temirov/gix/cmd/cli/repos"
	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/repos/shared"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

const (
	protocolFromFlagConstant       = "--from"
	protocolToFlagConstant         = "--to"
	protocolYesFlagConstant        = "--" + flagutils.AssumeYesFlagName
	protocolDryRunFlagConstant     = "--" + flagutils.DryRunFlagName
	protocolConfiguredRootConstant = "/tmp/protocol-config-root"
	protocolSSHRemoteURL           = "ssh://git@github.com/origin/example.git"
	protocolHTTPSRemoteURL         = "https://github.com/origin/example.git"
	protocolMissingRootsMessage    = "no repository roots provided; specify --root or configure defaults"
	protocolRelativeRootConstant   = "relative/protocol-root"
	protocolHomeRootSuffixConstant = "protocol-home-root"
)

func TestProtocolCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                    string
		configuration           repos.ProtocolConfiguration
		arguments               []string
		initialRemoteURL        string
		expectedRoots           []string
		expectedRootsBuilder    func(testing.TB) []string
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
		{
			name: "configuration_expands_home_relative_root",
			configuration: repos.ProtocolConfiguration{
				DryRun:          true,
				AssumeYes:       true,
				RepositoryRoots: []string{"~/" + protocolHomeRootSuffixConstant},
				FromProtocol:    string(shared.RemoteProtocolHTTPS),
				ToProtocol:      string(shared.RemoteProtocolSSH),
			},
			arguments:            []string{},
			initialRemoteURL:     protocolHTTPSRemoteURL,
			expectedRootsBuilder: protocolHomeRootBuilder,
			expectRemoteUpdates:  0,
		},
		{
			name: "arguments_preserve_relative_roots",
			configuration: repos.ProtocolConfiguration{
				FromProtocol: string(shared.RemoteProtocolHTTPS),
				ToProtocol:   string(shared.RemoteProtocolSSH),
			},
			arguments: []string{
				protocolFromFlagConstant,
				string(shared.RemoteProtocolHTTPS),
				protocolToFlagConstant,
				string(shared.RemoteProtocolSSH),
				protocolYesFlagConstant,
				protocolDryRunFlagConstant,
				protocolRelativeRootConstant,
			},
			initialRemoteURL:        protocolHTTPSRemoteURL,
			expectedRoots:           []string{protocolRelativeRootConstant},
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
		{
			name: "arguments_expand_home_relative_root",
			configuration: repos.ProtocolConfiguration{
				FromProtocol: string(shared.RemoteProtocolHTTPS),
				ToProtocol:   string(shared.RemoteProtocolSSH),
			},
			arguments: []string{
				protocolFromFlagConstant,
				string(shared.RemoteProtocolHTTPS),
				protocolToFlagConstant,
				string(shared.RemoteProtocolSSH),
				protocolYesFlagConstant,
				protocolDryRunFlagConstant,
				"~/" + protocolHomeRootSuffixConstant,
			},
			initialRemoteURL:        protocolHTTPSRemoteURL,
			expectedRootsBuilder:    protocolHomeRootBuilder,
			expectRemoteUpdates:     0,
			expectPromptInvocations: 0,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			discoverer := &fakeRepositoryDiscoverer{repositories: []string{remotesDiscoveredRepository}}
			executor := &fakeGitExecutor{}
			manager := &fakeGitRepositoryManager{remoteURL: testCase.initialRemoteURL, currentBranch: remotesMetadataDefaultBranch, panicOnCurrentBranchLookup: true}
			resolver := &fakeGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: remotesCanonicalRepository, DefaultBranch: remotesMetadataDefaultBranch}}
			prompter := &recordingPrompter{result: shared.ConfirmationResult{Confirmed: true}}

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
			bindGlobalProtocolFlags(command)

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
			if testCase.expectRemoteUpdates > 0 {
				require.NotEmpty(subtest, manager.setCalls)
				require.Equal(subtest, string(shared.RemoteProtocolSSH), detectProtocol(manager.setCalls[len(manager.setCalls)-1].remoteURL))
			}
		})
	}
}

func bindGlobalProtocolFlags(command *cobra.Command) {
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
	flagutils.BindExecutionFlags(command, flagutils.ExecutionDefaults{}, flagutils.ExecutionFlagDefinitions{
		DryRun:    flagutils.ExecutionFlagDefinition{Name: flagutils.DryRunFlagName, Usage: flagutils.DryRunFlagUsage, Enabled: true},
		AssumeYes: flagutils.ExecutionFlagDefinition{Name: flagutils.AssumeYesFlagName, Usage: flagutils.AssumeYesFlagUsage, Shorthand: flagutils.AssumeYesFlagShorthand, Enabled: true},
	})
	command.PersistentFlags().String(flagutils.RemoteFlagName, "", flagutils.RemoteFlagUsage)
	command.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		contextAccessor := utils.NewCommandContextAccessor()
		executionFlags := utils.ExecutionFlags{}
		if dryRunValue, dryRunChanged, dryRunError := flagutils.BoolFlag(cmd, flagutils.DryRunFlagName); dryRunError == nil {
			executionFlags.DryRun = dryRunValue
			executionFlags.DryRunSet = dryRunChanged
		}
		if assumeYesValue, assumeYesChanged, assumeYesError := flagutils.BoolFlag(cmd, flagutils.AssumeYesFlagName); assumeYesError == nil {
			executionFlags.AssumeYes = assumeYesValue
			executionFlags.AssumeYesSet = assumeYesChanged
		}
		if remoteValue, remoteChanged, remoteError := flagutils.StringFlag(cmd, flagutils.RemoteFlagName); remoteError == nil {
			executionFlags.Remote = strings.TrimSpace(remoteValue)
			executionFlags.RemoteSet = remoteChanged && len(strings.TrimSpace(remoteValue)) > 0
		}
		updatedContext := contextAccessor.WithExecutionFlags(cmd.Context(), executionFlags)
		cmd.SetContext(updatedContext)
		return nil
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

func protocolHomeRootBuilder(testingInstance testing.TB) []string {
	homeDirectory, homeError := os.UserHomeDir()
	require.NoError(testingInstance, homeError)
	expandedRoot := filepath.Join(homeDirectory, protocolHomeRootSuffixConstant)
	return []string{expandedRoot}
}
