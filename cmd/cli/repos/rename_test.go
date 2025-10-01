package repos_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	renameDryRunFlagConstant            = "--" + flagutils.DryRunFlagName
	renameAssumeYesFlagConstant         = "--" + flagutils.AssumeYesFlagName
	renameRequireCleanFlagConstant      = "--require-clean"
	renameRootFlagConstant              = "--" + flagutils.DefaultRootFlagName
	renameConfiguredRootConstant        = "/tmp/rename-config-root"
	renameCLIRepositoryRootConstant     = "/tmp/rename-cli-root"
	renameDiscoveredRepositoryPath      = "/tmp/rename-repo"
	renameOriginURLConstant             = "https://github.com/origin/example.git"
	renameCanonicalRepositoryConstant   = "canonical/example"
	renameMetadataDefaultBranchConstant = "main"
	renameLocalBranchConstant           = "main"
	renameMissingRootsMessage           = "no repository roots provided; specify --root or configure defaults"
	renameRelativeRootConstant          = "relative/rename-root"
	renameHomeRootSuffixConstant        = "rename-home-root"
	renameParentDirectoryPathConstant   = "/tmp"
)

func TestRenameCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                 string
		configuration        *repos.RenameConfiguration
		arguments            []string
		expectedRoots        []string
		expectedRootsBuilder func(testing.TB) []string
		expectError          bool
		expectedErrorMessage string
		expectedPromptCalls  int
		expectedRenameCalls  int
		expectedCleanChecks  int
	}{
		{
			name: "configuration_supplies_defaults",
			configuration: &repos.RenameConfiguration{
				DryRun:               true,
				AssumeYes:            true,
				RequireCleanWorktree: false,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments:           []string{},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 0,
		},
		{
			name: "flags_override_configuration",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            false,
				RequireCleanWorktree: false,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameDryRunFlagConstant,
				renameAssumeYesFlagConstant,
				renameRequireCleanFlagConstant,
				renameRootFlagConstant, renameCLIRepositoryRootConstant,
			},
			expectedRoots:       []string{renameCLIRepositoryRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 1,
		},
		{
			name:                 "error_when_roots_missing",
			configuration:        &repos.RenameConfiguration{},
			arguments:            []string{},
			expectError:          true,
			expectedErrorMessage: renameMissingRootsMessage,
		},
		{
			name: "configuration_expands_home_relative_root",
			configuration: &repos.RenameConfiguration{
				DryRun:               true,
				AssumeYes:            true,
				RequireCleanWorktree: true,
				RepositoryRoots:      []string{"~/" + renameHomeRootSuffixConstant},
			},
			arguments: []string{},
			expectedRootsBuilder: func(testingInstance testing.TB) []string {
				homeDirectory, homeError := os.UserHomeDir()
				require.NoError(testingInstance, homeError)
				expandedRoot := filepath.Join(homeDirectory, renameHomeRootSuffixConstant)
				return []string{expandedRoot}
			},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 1,
		},
		{
			name:          "arguments_preserve_relative_roots",
			configuration: &repos.RenameConfiguration{},
			arguments: []string{
				renameDryRunFlagConstant,
				renameAssumeYesFlagConstant,
				renameRequireCleanFlagConstant,
				renameRootFlagConstant, renameRelativeRootConstant,
			},
			expectedRoots:       []string{renameRelativeRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 1,
		},
		{
			name: "arguments_expand_home_relative_root",
			configuration: &repos.RenameConfiguration{
				DryRun:               true,
				AssumeYes:            true,
				RequireCleanWorktree: true,
			},
			arguments: []string{
				renameDryRunFlagConstant,
				renameAssumeYesFlagConstant,
				renameRequireCleanFlagConstant,
				renameRootFlagConstant, "~/" + renameHomeRootSuffixConstant,
			},
			expectedRootsBuilder: func(testingInstance testing.TB) []string {
				homeDirectory, homeError := os.UserHomeDir()
				require.NoError(testingInstance, homeError)
				expandedRoot := filepath.Join(homeDirectory, renameHomeRootSuffixConstant)
				return []string{expandedRoot}
			},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 1,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			discoverer := &fakeRepositoryDiscoverer{repositories: []string{renameDiscoveredRepositoryPath}}
			executor := &fakeGitExecutor{}
			manager := &fakeGitRepositoryManager{
				remoteURL:                  renameOriginURLConstant,
				currentBranch:              renameLocalBranchConstant,
				cleanWorktree:              true,
				cleanWorktreeSet:           true,
				panicOnCurrentBranchLookup: true,
			}
			resolver := &fakeGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: renameCanonicalRepositoryConstant, DefaultBranch: renameMetadataDefaultBranchConstant}}
			prompter := &recordingPrompter{result: shared.ConfirmationResult{Confirmed: true}}
			fileSystem := newRecordingFileSystem([]string{renameParentDirectoryPathConstant, renameDiscoveredRepositoryPath})

			var configurationProvider func() repos.RenameConfiguration
			if testCase.configuration != nil {
				configurationCopy := *testCase.configuration
				configurationProvider = func() repos.RenameConfiguration {
					return configurationCopy
				}
			}

			builder := repos.RenameCommandBuilder{
				LoggerProvider: func() *zap.Logger { return zap.NewNop() },
				Discoverer:     discoverer,
				GitExecutor:    executor,
				GitManager:     manager,
				GitHubResolver: resolver,
				FileSystem:     fileSystem,
				PrompterFactory: func(*cobra.Command) shared.ConfirmationPrompter {
					return prompter
				},
				HumanReadableLoggingProvider: func() bool { return false },
				ConfigurationProvider:        configurationProvider,
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)
			bindGlobalRenameFlags(command)

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
				require.Empty(subtest, fileSystem.renameOperations)
				require.Zero(subtest, manager.checkCleanCalls)
				return
			}

			require.NoError(subtest, executionError)

			expectedRoots := testCase.expectedRoots
			if testCase.expectedRootsBuilder != nil {
				expectedRoots = testCase.expectedRootsBuilder(subtest)
			}
			require.Equal(subtest, expectedRoots, discoverer.receivedRoots)
			require.Equal(subtest, testCase.expectedPromptCalls, prompter.calls)
			require.Equal(subtest, testCase.expectedRenameCalls, len(fileSystem.renameOperations))
			require.Equal(subtest, testCase.expectedCleanChecks, manager.checkCleanCalls)
		})
	}
}

func bindGlobalRenameFlags(command *cobra.Command) {
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

type renameOperation struct {
	oldPath string
	newPath string
}

type recordingFileSystem struct {
	renameOperations []renameOperation
	existingPaths    map[string]struct{}
}

func newRecordingFileSystem(existingPaths []string) *recordingFileSystem {
	pathSet := make(map[string]struct{}, len(existingPaths))
	for index := range existingPaths {
		pathSet[existingPaths[index]] = struct{}{}
	}
	return &recordingFileSystem{existingPaths: pathSet}
}

func (fileSystem *recordingFileSystem) Stat(path string) (fs.FileInfo, error) {
	if fileSystem.Exists(path) {
		return fakeFileInfo{name: filepath.Base(path)}, nil
	}
	return nil, os.ErrNotExist
}

func (fileSystem *recordingFileSystem) Rename(oldPath string, newPath string) error {
	fileSystem.renameOperations = append(fileSystem.renameOperations, renameOperation{oldPath: oldPath, newPath: newPath})
	fileSystem.existingPaths[newPath] = struct{}{}
	delete(fileSystem.existingPaths, oldPath)
	return nil
}

func (fileSystem *recordingFileSystem) Exists(path string) bool {
	_, exists := fileSystem.existingPaths[path]
	return exists
}

func (fileSystem *recordingFileSystem) Abs(path string) (string, error) {
	return filepath.Clean(path), nil
}

type fakeFileInfo struct {
	name string
}

func (info fakeFileInfo) Name() string {
	return info.name
}

func (info fakeFileInfo) Size() int64 {
	return 0
}

func (info fakeFileInfo) Mode() fs.FileMode {
	return fs.ModeDir
}

func (info fakeFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (info fakeFileInfo) IsDir() bool {
	return true
}

func (info fakeFileInfo) Sys() any {
	return nil
}
