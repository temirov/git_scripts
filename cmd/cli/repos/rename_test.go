package repos_test

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
	renameIncludeOwnerFlagConstant      = "--owner"
	renameRootFlagConstant              = "--" + flagutils.DefaultRootFlagName
	renameConfiguredRootConstant        = "/tmp/rename-config-root"
	renameCLIRepositoryRootConstant     = "/tmp/rename-cli-root"
	renameDiscoveredRepositoryPath      = "/tmp/rename-repo"
	renameCanonicalDirectoryPath        = "/tmp/canonical/example"
	renameOriginURLConstant             = "https://github.com/origin/example.git"
	renameCanonicalRepositoryConstant   = "canonical/example"
	renameOwnerSegmentConstant          = "canonical"
	renameRepositorySegmentConstant     = "example"
	renameMetadataDefaultBranchConstant = "main"
	renameLocalBranchConstant           = "main"
	renameMissingRootsMessage           = "no repository roots provided; specify --roots or configure defaults"
	renameRelativeRootConstant          = "relative/rename-root"
	renameHomeRootSuffixConstant        = "rename-home-root"
	renameParentDirectoryPathConstant   = "/tmp"
)

func TestRenameCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                       string
		configuration              *repos.RenameConfiguration
		arguments                  []string
		discoveredRepositories     []string
		expectedRoots              []string
		expectedRootsBuilder       func(testing.TB) []string
		expectError                bool
		expectedErrorMessage       string
		expectedPromptCalls        int
		expectedRenameCalls        int
		expectedCleanChecks        int
		expectedRenameTargets      []renameOperation
		expectedCreatedDirectories []string
		expectedStdout             string
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
		{
			name: "configuration_enables_include_owner",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            true,
				RequireCleanWorktree: false,
				IncludeOwner:         true,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments:           []string{},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
			expectedRenameTargets: []renameOperation{
				{
					oldPath: renameDiscoveredRepositoryPath,
					newPath: filepath.Join(renameParentDirectoryPathConstant, renameOwnerSegmentConstant, renameRepositorySegmentConstant),
				},
			},
			expectedCreatedDirectories: []string{filepath.Join(renameParentDirectoryPathConstant, renameOwnerSegmentConstant)},
		},
		{
			name: "flag_enables_include_owner",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            true,
				RequireCleanWorktree: false,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameAssumeYesFlagConstant,
				renameIncludeOwnerFlagConstant,
				renameRootFlagConstant, renameCLIRepositoryRootConstant,
			},
			expectedRoots:       []string{renameCLIRepositoryRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
			expectedRenameTargets: []renameOperation{
				{
					oldPath: renameDiscoveredRepositoryPath,
					newPath: filepath.Join(renameParentDirectoryPathConstant, renameOwnerSegmentConstant, renameRepositorySegmentConstant),
				},
			},
			expectedCreatedDirectories: []string{filepath.Join(renameParentDirectoryPathConstant, renameOwnerSegmentConstant)},
		},
		{
			name: "flag_disables_include_owner",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            true,
				RequireCleanWorktree: false,
				IncludeOwner:         true,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameAssumeYesFlagConstant,
				renameIncludeOwnerFlagConstant + "=false",
			},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
			expectedRenameTargets: []renameOperation{
				{
					oldPath: renameDiscoveredRepositoryPath,
					newPath: filepath.Join(renameParentDirectoryPathConstant, renameRepositorySegmentConstant),
				},
			},
			expectedCreatedDirectories: nil,
		},
		{
			name: "flag_disables_include_owner_with_no_literal",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            true,
				RequireCleanWorktree: false,
				IncludeOwner:         true,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameAssumeYesFlagConstant,
				renameIncludeOwnerFlagConstant,
				"no",
			},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
			expectedRenameTargets: []renameOperation{
				{
					oldPath: renameDiscoveredRepositoryPath,
					newPath: filepath.Join(renameParentDirectoryPathConstant, renameRepositorySegmentConstant),
				},
			},
			expectedCreatedDirectories: nil,
		},
		{
			name: "flag_disables_require_clean_with_no_literal",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            true,
				RequireCleanWorktree: true,
				IncludeOwner:         true,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameAssumeYesFlagConstant,
				renameRequireCleanFlagConstant,
				"no",
			},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
			expectedRenameTargets: []renameOperation{
				{
					oldPath: renameDiscoveredRepositoryPath,
					newPath: filepath.Join(renameParentDirectoryPathConstant, renameOwnerSegmentConstant, renameRepositorySegmentConstant),
				},
			},
			expectedCreatedDirectories: []string{filepath.Join(renameParentDirectoryPathConstant, renameOwnerSegmentConstant)},
		},
		{
			name: "already_normalized_skips_with_message",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            true,
				RequireCleanWorktree: false,
				IncludeOwner:         true,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameAssumeYesFlagConstant,
			},
			discoveredRepositories: []string{renameCanonicalDirectoryPath},
			expectedRoots:          []string{renameConfiguredRootConstant},
			expectedPromptCalls:    0,
			expectedRenameCalls:    0,
			expectedCleanChecks:    0,
			expectedStdout:         fmt.Sprintf("SKIP (already normalized): %s\n", renameCanonicalDirectoryPath),
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			repositories := testCase.discoveredRepositories
			if len(repositories) == 0 {
				repositories = []string{renameDiscoveredRepositoryPath}
			}
			discoverer := &fakeRepositoryDiscoverer{repositories: repositories}
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
			existingPaths := []string{renameParentDirectoryPathConstant}
			for _, repositoryPath := range repositories {
				existingPaths = append(existingPaths, repositoryPath)
				parentPath := filepath.Dir(repositoryPath)
				if parentPath != repositoryPath {
					existingPaths = append(existingPaths, parentPath)
				}
			}
			fileSystem := newRecordingFileSystem(existingPaths)

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
			normalizedArguments := flagutils.NormalizeToggleArguments(testCase.arguments)
			command.SetArgs(normalizedArguments)

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
				require.Empty(subtest, fileSystem.createdDirectories)
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
			if len(testCase.expectedRenameTargets) > 0 {
				require.Equal(subtest, testCase.expectedRenameTargets, fileSystem.renameOperations)
			}
			require.Equal(subtest, testCase.expectedCreatedDirectories, fileSystem.createdDirectories)
			if len(testCase.expectedStdout) > 0 {
				require.Equal(subtest, testCase.expectedStdout, stdoutBuffer.String())
			}
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
	renameOperations   []renameOperation
	existingPaths      map[string]struct{}
	createdDirectories []string
	fileContents       map[string][]byte
}

func newRecordingFileSystem(existingPaths []string) *recordingFileSystem {
	pathSet := make(map[string]struct{}, len(existingPaths))
	for index := range existingPaths {
		pathSet[existingPaths[index]] = struct{}{}
	}
	return &recordingFileSystem{existingPaths: pathSet, fileContents: map[string][]byte{}}
}

func (fileSystem *recordingFileSystem) Stat(path string) (fs.FileInfo, error) {
	if fileSystem.Exists(path) {
		return fakeFileInfo{name: filepath.Base(path)}, nil
	}
	return nil, os.ErrNotExist
}

func (fileSystem *recordingFileSystem) Rename(oldPath string, newPath string) error {
	fileSystem.renameOperations = append(fileSystem.renameOperations, renameOperation{oldPath: oldPath, newPath: newPath})
	fileSystem.ensurePathSet()
	fileSystem.existingPaths[newPath] = struct{}{}
	delete(fileSystem.existingPaths, oldPath)
	return nil
}

func (fileSystem *recordingFileSystem) MkdirAll(path string, permissions fs.FileMode) error {
	fileSystem.ensurePathSet()
	fileSystem.createdDirectories = append(fileSystem.createdDirectories, path)
	fileSystem.existingPaths[path] = struct{}{}
	return nil
}

func (fileSystem *recordingFileSystem) ReadFile(path string) ([]byte, error) {
	fileSystem.ensurePathSet()
	if contents, exists := fileSystem.fileContents[path]; exists {
		duplicate := make([]byte, len(contents))
		copy(duplicate, contents)
		return duplicate, nil
	}
	return nil, os.ErrNotExist
}

func (fileSystem *recordingFileSystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	fileSystem.ensurePathSet()
	duplicate := make([]byte, len(data))
	copy(duplicate, data)
	fileSystem.fileContents[path] = duplicate
	fileSystem.existingPaths[path] = struct{}{}
	return nil
}

func (fileSystem *recordingFileSystem) Exists(path string) bool {
	fileSystem.ensurePathSet()
	_, exists := fileSystem.existingPaths[path]
	return exists
}

func (fileSystem *recordingFileSystem) Abs(path string) (string, error) {
	return filepath.Clean(path), nil
}

func (fileSystem *recordingFileSystem) ensurePathSet() {
	if fileSystem.existingPaths == nil {
		fileSystem.existingPaths = map[string]struct{}{}
	}
	if fileSystem.fileContents == nil {
		fileSystem.fileContents = map[string][]byte{}
	}
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
