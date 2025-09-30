package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithRepositoryContextStoresNormalizedValues(t *testing.T) {
	accessor := NewCommandContextAccessor()
	base := context.Background()
	enriched := accessor.WithRepositoryContext(base, RepositoryContext{Owner: "  example ", Name: " repo "})

	repositoryContext, exists := accessor.RepositoryContext(enriched)
	require.True(t, exists)
	require.Equal(t, "example", repositoryContext.Owner)
	require.Equal(t, "repo", repositoryContext.Name)
}

func TestWithRepositoryContextSkipsEmptyValues(t *testing.T) {
	accessor := NewCommandContextAccessor()
	base := context.Background()
	enriched := accessor.WithRepositoryContext(base, RepositoryContext{})

	_, exists := accessor.RepositoryContext(enriched)
	require.False(t, exists)
}

func TestWithBranchContextStoresNormalizedValue(t *testing.T) {
	accessor := NewCommandContextAccessor()
	base := context.Background()
	enriched := accessor.WithBranchContext(base, BranchContext{Name: " main "})

	branchContext, exists := accessor.BranchContext(enriched)
	require.True(t, exists)
	require.Equal(t, "main", branchContext.Name)
}

func TestWithBranchContextSkipsEmptyValue(t *testing.T) {
	accessor := NewCommandContextAccessor()
	base := context.Background()
	enriched := accessor.WithBranchContext(base, BranchContext{})

	_, exists := accessor.BranchContext(enriched)
	require.False(t, exists)
}

func TestWithExecutionFlagsStoresValues(t *testing.T) {
	accessor := NewCommandContextAccessor()
	base := context.Background()
	flags := ExecutionFlags{DryRun: true, DryRunSet: true, AssumeYes: true, AssumeYesSet: true, Remote: "origin", RemoteSet: true}

	enriched := accessor.WithExecutionFlags(base, flags)

	retrieved, exists := accessor.ExecutionFlags(enriched)
	require.True(t, exists)
	require.Equal(t, flags, retrieved)
}

func TestWithExecutionFlagsHandlesMissingContext(t *testing.T) {
	accessor := NewCommandContextAccessor()

	_, exists := accessor.ExecutionFlags(nil)
	require.False(t, exists)
}
