package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyDefaultsSetsRequireCleanWhenMissing(t *testing.T) {
	operation := &RenameOperation{}
	operations := []Operation{operation}

	ApplyDefaults(operations, OperationDefaults{RequireClean: true})

	require.True(t, operation.RequireCleanWorktree)
}

func TestApplyDefaultsRespectsExplicitRequireClean(t *testing.T) {
	operation := &RenameOperation{RequireCleanWorktree: false, requireCleanExplicit: true}
	operations := []Operation{operation}

	ApplyDefaults(operations, OperationDefaults{RequireClean: true})

	require.False(t, operation.RequireCleanWorktree)
}
