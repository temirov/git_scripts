package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	flagutils "github.com/temirov/gix/internal/utils/flags"
)

func TestApplicationCommonDefaultsApplied(t *testing.T) {
	operations, buildError := newOperationConfigurations([]ApplicationOperationConfiguration{
		{
			Name: reposRenameOperationNameConstant,
			Options: map[string]any{
				"roots": []string{"/tmp/rename"},
			},
		},
		{
			Name: workflowCommandOperationNameConstant,
			Options: map[string]any{
				"roots": []string{"/tmp/workflow"},
			},
		},
	})
	require.NoError(t, buildError)

	application := &Application{
		logger: zap.NewNop(),
		configuration: ApplicationConfiguration{
			Common: ApplicationCommonConfiguration{
				DryRun:       true,
				AssumeYes:    true,
				RequireClean: true,
			},
		},
		operationConfigurations: operations,
	}

	renameConfiguration := application.reposRenameConfiguration()
	require.True(t, renameConfiguration.DryRun)
	require.True(t, renameConfiguration.AssumeYes)
	require.True(t, renameConfiguration.RequireCleanWorktree)
	require.False(t, renameConfiguration.IncludeOwner)

	workflowConfiguration := application.workflowCommandConfiguration()
	require.True(t, workflowConfiguration.DryRun)
	require.True(t, workflowConfiguration.AssumeYes)
	require.True(t, workflowConfiguration.RequireClean)
}

func TestApplicationOperationOverridesTakePriority(t *testing.T) {
	operations, buildError := newOperationConfigurations([]ApplicationOperationConfiguration{
		{
			Name: reposRenameOperationNameConstant,
			Options: map[string]any{
				"dry_run":       false,
				"assume_yes":    false,
				"require_clean": false,
				"include_owner": true,
				"roots":         []string{"/tmp/rename"},
			},
		},
		{
			Name: workflowCommandOperationNameConstant,
			Options: map[string]any{
				"dry_run":       false,
				"assume_yes":    false,
				"require_clean": false,
				"roots":         []string{"/tmp/workflow"},
			},
		},
	})
	require.NoError(t, buildError)

	application := &Application{
		logger: zap.NewNop(),
		configuration: ApplicationConfiguration{
			Common: ApplicationCommonConfiguration{
				DryRun:       true,
				AssumeYes:    true,
				RequireClean: true,
			},
		},
		operationConfigurations: operations,
	}

	renameConfiguration := application.reposRenameConfiguration()
	require.False(t, renameConfiguration.DryRun)
	require.False(t, renameConfiguration.AssumeYes)
	require.False(t, renameConfiguration.RequireCleanWorktree)
	require.True(t, renameConfiguration.IncludeOwner)

	workflowConfiguration := application.workflowCommandConfiguration()
	require.False(t, workflowConfiguration.DryRun)
	require.False(t, workflowConfiguration.AssumeYes)
	require.False(t, workflowConfiguration.RequireClean)
}

func TestInitializeConfigurationAttachesBranchContext(t *testing.T) {
	application := NewApplication()
	rootCommand := application.rootCommand
	rootCommand.SetContext(context.Background())

	require.NoError(t, rootCommand.PersistentFlags().Set(flagutils.DryRunFlagName, "true"))
	require.NoError(t, rootCommand.PersistentFlags().Set(flagutils.AssumeYesFlagName, "true"))
	require.NoError(t, rootCommand.PersistentFlags().Set(flagutils.RemoteFlagName, "custom-remote"))

	initializationError := application.initializeConfiguration(rootCommand)
	require.NoError(t, initializationError)

	branchContext, branchExists := application.commandContextAccessor.BranchContext(rootCommand.Context())
	require.True(t, branchExists)
	require.Empty(t, branchContext.Name)
	require.True(t, branchContext.RequireClean)

	executionFlags, executionFlagsAvailable := application.commandContextAccessor.ExecutionFlags(rootCommand.Context())
	require.True(t, executionFlagsAvailable)
	require.True(t, executionFlags.DryRun)
	require.True(t, executionFlags.AssumeYes)
	require.Equal(t, "custom-remote", executionFlags.Remote)
}

func TestRootCommandToggleHelpFormatting(t *testing.T) {
	application := NewApplication()
	usage := application.rootCommand.PersistentFlags().FlagUsages()

	require.Contains(t, usage, "--dry-run <yes|NO>")
	require.Contains(t, usage, "--yes <yes|NO>")
	require.NotContains(t, usage, "__toggle_true__")
	require.NotContains(t, usage, "toggle[")
}

func TestApplicationCommandHierarchyAndAliases(t *testing.T) {
	application := NewApplication()
	rootCommand := application.rootCommand

	auditCommand, _, auditError := rootCommand.Find([]string{"a"})
	require.NoError(t, auditError)
	require.Equal(t, auditOperationNameConstant, auditCommand.Name())

	workflowCommand, _, workflowError := rootCommand.Find([]string{"w"})
	require.NoError(t, workflowError)
	require.Equal(t, workflowCommandOperationNameConstant, workflowCommand.Name())

	repoRenameCommand, _, renameError := rootCommand.Find([]string{"r", "folder", "rename"})
	require.NoError(t, renameError)
	require.Equal(t, "rename", repoRenameCommand.Name())
	require.NotNil(t, repoRenameCommand.Parent())
	require.Equal(t, "folder", repoRenameCommand.Parent().Name())
	require.NotNil(t, repoRenameCommand.Parent().Parent())
	require.Equal(t, "repo", repoRenameCommand.Parent().Parent().Name())

	repoRemoteCanonicalCommand, _, canonicalError := rootCommand.Find([]string{"r", "remote", "update-to-canonical"})
	require.NoError(t, canonicalError)
	require.Equal(t, "update-to-canonical", repoRemoteCanonicalCommand.Name())
	require.NotNil(t, repoRemoteCanonicalCommand.Parent())
	require.Equal(t, "remote", repoRemoteCanonicalCommand.Parent().Name())

	repoRemoteProtocolCommand, _, protocolError := rootCommand.Find([]string{"r", "remote", "update-protocol"})
	require.NoError(t, protocolError)
	require.Equal(t, "update-protocol", repoRemoteProtocolCommand.Name())
	require.NotNil(t, repoRemoteProtocolCommand.Parent())
	require.Equal(t, "remote", repoRemoteProtocolCommand.Parent().Name())

	repoPullRequestsCommand, _, pullRequestsError := rootCommand.Find([]string{"r", "prs", "delete"})
	require.NoError(t, pullRequestsError)
	require.Equal(t, "delete", repoPullRequestsCommand.Name())
	require.NotNil(t, repoPullRequestsCommand.Parent())
	require.Equal(t, "prs", repoPullRequestsCommand.Parent().Name())

	repoPackagesCommand, _, packagesError := rootCommand.Find([]string{"r", "packages", "delete"})
	require.NoError(t, packagesError)
	require.Equal(t, "delete", repoPackagesCommand.Name())
	require.NotNil(t, repoPackagesCommand.Parent())
	require.Equal(t, "packages", repoPackagesCommand.Parent().Name())

	releaseCommand, _, releaseError := rootCommand.Find([]string{"r", "release"})
	require.NoError(t, releaseError)
	require.Equal(t, "release", releaseCommand.Name())
	require.NotNil(t, releaseCommand.Parent())
	require.Equal(t, "repo", releaseCommand.Parent().Name())

	branchMigrateCommand, _, branchMigrateError := rootCommand.Find([]string{"b", "migrate"})
	require.NoError(t, branchMigrateError)
	require.Equal(t, "migrate", branchMigrateCommand.Name())
	require.NotNil(t, branchMigrateCommand.Parent())
	require.Equal(t, "branch", branchMigrateCommand.Parent().Name())

	branchChangeCommand, _, branchChangeError := rootCommand.Find([]string{"b", "cd"})
	require.NoError(t, branchChangeError)
	require.Equal(t, "cd", branchChangeCommand.Name())
	require.NotNil(t, branchChangeCommand.Parent())
	require.Equal(t, "branch", branchChangeCommand.Parent().Name())

	commitMessageCommand, _, commitMessageError := rootCommand.Find([]string{"b", "commit", "message"})
	require.NoError(t, commitMessageError)
	require.Equal(t, "message", commitMessageCommand.Name())
	require.NotNil(t, commitMessageCommand.Parent())
	require.Equal(t, "commit", commitMessageCommand.Parent().Name())
	require.NotNil(t, commitMessageCommand.Parent().Parent())
	require.Equal(t, "branch", commitMessageCommand.Parent().Parent().Name())

	changelogMessageCommand, _, changelogMessageError := rootCommand.Find([]string{"r", "changelog", "message"})
	require.NoError(t, changelogMessageError)
	require.Equal(t, "message", changelogMessageCommand.Name())
	require.NotNil(t, changelogMessageCommand.Parent())
	require.Equal(t, "changelog", changelogMessageCommand.Parent().Name())
	require.NotNil(t, changelogMessageCommand.Parent().Parent())
	require.Equal(t, "repo", changelogMessageCommand.Parent().Parent().Name())

	_, _, legacyRenameError := rootCommand.Find([]string{"repo-folders-rename"})
	require.Error(t, legacyRenameError)
	require.Contains(t, legacyRenameError.Error(), "unknown command")

	_, _, legacyRemoteError := rootCommand.Find([]string{"repo-remote-update"})
	require.Error(t, legacyRemoteError)
	require.Contains(t, legacyRemoteError.Error(), "unknown command")

	_, _, legacyProtocolError := rootCommand.Find([]string{"repo-protocol-convert"})
	require.Error(t, legacyProtocolError)
	require.Contains(t, legacyProtocolError.Error(), "unknown command")

	_, _, legacyPullRequestsError := rootCommand.Find([]string{"repo-prs-purge"})
	require.Error(t, legacyPullRequestsError)
	require.Contains(t, legacyPullRequestsError.Error(), "unknown command")

	_, _, legacyPackagesError := rootCommand.Find([]string{"repo-packages-purge"})
	require.Error(t, legacyPackagesError)
	require.Contains(t, legacyPackagesError.Error(), "unknown command")
}

func TestApplicationHierarchicalCommandsLoadExpectedOperations(t *testing.T) {
	application := NewApplication()
	rootCommand := application.rootCommand

	repoRenameCommand, _, renameError := rootCommand.Find([]string{"r", "folder", "rename"})
	require.NoError(t, renameError)
	require.Equal(t, []string{reposRenameOperationNameConstant}, application.operationsRequiredForCommand(repoRenameCommand))

	repoRemoteCanonicalCommand, _, canonicalError := rootCommand.Find([]string{"r", "remote", "update-to-canonical"})
	require.NoError(t, canonicalError)
	require.Equal(t, []string{reposRemotesOperationNameConstant}, application.operationsRequiredForCommand(repoRemoteCanonicalCommand))

	repoRemoteProtocolCommand, _, protocolError := rootCommand.Find([]string{"r", "remote", "update-protocol"})
	require.NoError(t, protocolError)
	require.Equal(t, []string{reposProtocolOperationNameConstant}, application.operationsRequiredForCommand(repoRemoteProtocolCommand))

	repoPullRequestsCommand, _, pullRequestsError := rootCommand.Find([]string{"r", "prs", "delete"})
	require.NoError(t, pullRequestsError)
	require.Equal(t, []string{branchCleanupOperationNameConstant}, application.operationsRequiredForCommand(repoPullRequestsCommand))

	repoPackagesCommand, _, packagesError := rootCommand.Find([]string{"r", "packages", "delete"})
	require.NoError(t, packagesError)
	require.Equal(t, []string{packagesPurgeOperationNameConstant}, application.operationsRequiredForCommand(repoPackagesCommand))

	branchMigrateCommand, _, branchMigrateError := rootCommand.Find([]string{"b", "migrate"})
	require.NoError(t, branchMigrateError)
	require.Equal(t, []string{branchMigrateOperationNameConstant}, application.operationsRequiredForCommand(branchMigrateCommand))

	commitMessageCommand, _, commitMessageError := rootCommand.Find([]string{"b", "commit", "message"})
	require.NoError(t, commitMessageError)
	require.Equal(t, []string{commitMessageOperationNameConstant}, application.operationsRequiredForCommand(commitMessageCommand))

	changelogMessageCommand, _, changelogMessageError := rootCommand.Find([]string{"r", "changelog", "message"})
	require.NoError(t, changelogMessageError)
	require.Equal(t, []string{changelogMessageOperationNameConstant}, application.operationsRequiredForCommand(changelogMessageCommand))
}

func TestReleaseCommandUsageIncludesTagPlaceholder(t *testing.T) {
	application := NewApplication()
	rootCommand := application.rootCommand

	releaseCommand, _, releaseError := rootCommand.Find([]string{"r", "release"})
	require.NoError(t, releaseError)

	require.True(t, strings.HasPrefix(strings.TrimSpace(releaseCommand.Use), repoReleaseCommandUseNameConstant))
	require.Contains(t, releaseCommand.Use, "<tag>")
	require.Contains(t, releaseCommand.Long, "Provide the tag as the first argument")
	require.Contains(t, releaseCommand.Example, "gix repo release")
}

func TestBranchChangeCommandUsageIncludesBranchPlaceholder(t *testing.T) {
	application := NewApplication()
	rootCommand := application.rootCommand

	branchChangeCommand, _, branchChangeError := rootCommand.Find([]string{"b", "cd"})
	require.NoError(t, branchChangeError)

	require.True(t, strings.HasPrefix(strings.TrimSpace(branchChangeCommand.Use), branchChangeCommandUseNameConstant))
	require.Contains(t, branchChangeCommand.Use, "<branch>")
	require.Contains(t, branchChangeCommand.Long, "Provide the branch name as the first argument")
	require.Contains(t, branchChangeCommand.Example, "gix branch cd")
}

func TestWorkflowCommandUsageIncludesConfigurationPlaceholder(t *testing.T) {
	application := NewApplication()
	rootCommand := application.rootCommand

	workflowCommand, _, workflowError := rootCommand.Find([]string{"w"})
	require.NoError(t, workflowError)

	require.Contains(t, workflowCommand.Use, "<configuration>")
	require.Contains(t, workflowCommand.Long, "Provide the configuration path as the first argument")
	require.Contains(t, workflowCommand.Example, "gix workflow")
}

func TestRepoReleaseConfigurationUsesEmbeddedDefaults(t *testing.T) {
	application := NewApplication()

	command := &cobra.Command{Use: "test-command"}
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	require.NoError(t, application.initializeConfiguration(command))

	configuration := application.repoReleaseConfiguration()
	require.Equal(t, []string{"."}, configuration.RepositoryRoots)
	require.Equal(t, "origin", configuration.RemoteName)
}

func TestInitializeConfigurationMergesEmbeddedRepoReleaseDefaults(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configurationPath := filepath.Join(temporaryDirectory, "config.yaml")

	configurationContent := `common:
  log_level: info
  log_format: console
operations:
  - operation: repo-folders-rename
    with:
      roots:
        - ./custom
`
	require.NoError(t, os.WriteFile(configurationPath, []byte(configurationContent), 0o644))

	application := NewApplication()
	application.configurationFilePath = configurationPath

	command := &cobra.Command{Use: "test-command"}
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	require.NoError(t, application.initializeConfiguration(command))

	options, lookupError := application.operationConfigurations.Lookup(repoReleaseOperationNameConstant)
	require.NoError(t, lookupError)
	require.NotNil(t, options)

	releaseConfiguration := application.repoReleaseConfiguration()
	require.Equal(t, []string{"."}, releaseConfiguration.RepositoryRoots)
	require.Equal(t, "origin", releaseConfiguration.RemoteName)
}
