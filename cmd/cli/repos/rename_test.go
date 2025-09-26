package repos_test

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	repos "github.com/temirov/git_scripts/cmd/cli/repos"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	renameDryRunFlagConstant            = "--dry-run"
	renameAssumeYesFlagConstant         = "--yes"
	renameRequireCleanFlagConstant      = "--require-clean"
	renameConfiguredRootConstant        = "/tmp/rename-config-root"
	renameCLIRepositoryRootConstant     = "/tmp/rename-cli-root"
	renameParentDirectoryPathConstant   = "/tmp"
	renameExistingFolderNameConstant    = "rename-folder-current"
	renameDesiredFolderNameConstant     = "rename-folder-canonical"
	renameOriginURLConstant             = "https://github.com/example/rename-folder-current.git"
	renameCanonicalRepositoryConstant   = "example/" + renameDesiredFolderNameConstant
	renameMetadataDefaultBranchConstant = "main"
	renameLocalBranchConstant           = "feature/example"
)

var (
	renameDiscoveredRepositoryPath = filepath.Join(renameParentDirectoryPathConstant, renameExistingFolderNameConstant)
)

func TestRenameCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                string
		configuration       *repos.RenameConfiguration
		arguments           []string
		expectedRoots       []string
		expectedPromptCalls int
		expectedRenameCalls int
		expectedCleanChecks int
	}{
		{
			name:                "defaults_apply_without_configuration",
			configuration:       nil,
			arguments:           []string{},
			expectedRoots:       []string{"."},
			expectedPromptCalls: 1,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
		},
		{
			name: "configuration_enables_dry_run",
			configuration: &repos.RenameConfiguration{
				DryRun:               true,
				AssumeYes:            true,
				RequireCleanWorktree: true,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments:           []string{},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 1,
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
				renameCLIRepositoryRootConstant,
			},
			expectedRoots:       []string{renameCLIRepositoryRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 0,
			expectedCleanChecks: 1,
		},
		{
			name: "flag_enables_assume_yes_without_dry_run",
			configuration: &repos.RenameConfiguration{
				DryRun:               false,
				AssumeYes:            false,
				RequireCleanWorktree: false,
				RepositoryRoots:      []string{renameConfiguredRootConstant},
			},
			arguments: []string{
				renameAssumeYesFlagConstant,
			},
			expectedRoots:       []string{renameConfiguredRootConstant},
			expectedPromptCalls: 0,
			expectedRenameCalls: 1,
			expectedCleanChecks: 0,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			discoverer := &fakeRepositoryDiscoverer{repositories: []string{renameDiscoveredRepositoryPath}}
			executor := &fakeGitExecutor{}
			manager := &fakeGitRepositoryManager{
				remoteURL:        renameOriginURLConstant,
				currentBranch:    renameLocalBranchConstant,
				cleanWorktree:    true,
				cleanWorktreeSet: true,
			}
			resolver := &fakeGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: renameCanonicalRepositoryConstant, DefaultBranch: renameMetadataDefaultBranchConstant}}
			prompter := &recordingPrompter{confirmResult: true}
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

			command.SetContext(context.Background())
			command.SetOut(&bytes.Buffer{})
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(testCase.arguments)

			executionError := command.Execute()
			require.NoError(subtest, executionError)

			require.Equal(subtest, testCase.expectedRoots, discoverer.receivedRoots)
			require.Equal(subtest, testCase.expectedPromptCalls, prompter.calls)
			require.Equal(subtest, testCase.expectedRenameCalls, len(fileSystem.renameOperations))
			require.Equal(subtest, testCase.expectedCleanChecks, manager.checkCleanCalls)
		})
	}
}

type renameOperation struct {
	oldPath string
	newPath string
}

type recordingFileSystem struct {
	existingPaths    map[string]bool
	renameOperations []renameOperation
}

func newRecordingFileSystem(initialPaths []string) *recordingFileSystem {
	pathMap := make(map[string]bool, len(initialPaths))
	for _, path := range initialPaths {
		pathMap[path] = true
	}
	return &recordingFileSystem{existingPaths: pathMap}
}

func (fileSystem *recordingFileSystem) Stat(path string) (fs.FileInfo, error) {
	if fileSystem.existingPaths[path] {
		return staticFileInfo{name: path}, nil
	}
	return nil, fs.ErrNotExist
}

func (fileSystem *recordingFileSystem) Rename(oldPath string, newPath string) error {
	if !fileSystem.existingPaths[oldPath] {
		return fs.ErrNotExist
	}
	fileSystem.renameOperations = append(fileSystem.renameOperations, renameOperation{oldPath: oldPath, newPath: newPath})
	delete(fileSystem.existingPaths, oldPath)
	fileSystem.existingPaths[newPath] = true
	return nil
}

func (fileSystem *recordingFileSystem) Abs(path string) (string, error) {
	return path, nil
}

type staticFileInfo struct {
	name string
}

func (info staticFileInfo) Name() string  { return info.name }
func (staticFileInfo) Size() int64        { return 0 }
func (staticFileInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (staticFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (staticFileInfo) IsDir() bool        { return true }
func (staticFileInfo) Sys() any           { return nil }
