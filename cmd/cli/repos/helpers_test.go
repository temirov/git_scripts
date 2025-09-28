package repos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimRootsSkipsBooleanLiterals(testInstance *testing.T) {
	temporaryDirectory := testInstance.TempDir()
	expectedPath := filepath.Join(temporaryDirectory, "repository")

	inputs := []string{"", "true", "TrUe", "false", "FALSE", expectedPath}
	trimmed := trimRoots(inputs)

	require.Equal(testInstance, []string{expectedPath}, trimmed)
}

func TestDetermineRepositoryRootsUsesConfiguredWhenArgumentsAreBooleanLiterals(testInstance *testing.T) {
	homeDirectory, homeError := os.UserHomeDir()
	require.NoError(testInstance, homeError)

	configured := []string{filepath.Join(homeDirectory, "Development")}
	resolved := determineRepositoryRoots([]string{"true", "FALSE"}, configured)

	require.Equal(testInstance, configured, resolved)
}
