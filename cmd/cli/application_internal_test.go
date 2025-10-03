package cli

import (
	"context"
	"testing"

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
	require.NoError(t, rootCommand.PersistentFlags().Set(branchFlagNameConstant, "main"))

	initializationError := application.initializeConfiguration(rootCommand)
	require.NoError(t, initializationError)

	branchContext, branchExists := application.commandContextAccessor.BranchContext(rootCommand.Context())
	require.True(t, branchExists)
	require.Equal(t, "main", branchContext.Name)
	require.True(t, branchContext.RequireClean)

	executionFlags, executionFlagsAvailable := application.commandContextAccessor.ExecutionFlags(rootCommand.Context())
	require.True(t, executionFlagsAvailable)
	require.True(t, executionFlags.DryRun)
	require.True(t, executionFlags.AssumeYes)
	require.Equal(t, "custom-remote", executionFlags.Remote)
}
