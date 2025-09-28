package repos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testBooleanLiteralTrueConstant          = "true"
	testBooleanLiteralFalseConstant         = "FALSE"
	testRepositoryRelativePathConstant      = "projects/example"
	testDetermineRootsArgumentsCaseConstant = "arguments_preferred"
	testDetermineRootsConfigurationCase     = "configuration_used_when_arguments_filtered"
)

func TestDetermineRepositoryRootsSanitizesInputs(testInstance *testing.T) {
	testInstance.Helper()

	homeDirectory, homeDirectoryError := os.UserHomeDir()
	require.NoError(testInstance, homeDirectoryError)

	tildeArgument := filepath.Join("~", testRepositoryRelativePathConstant)
	expectedExpanded := filepath.Join(homeDirectory, testRepositoryRelativePathConstant)
	configuredRoot := filepath.Join(homeDirectory, "configured")

	testCases := []struct {
		name             string
		arguments        []string
		configured       []string
		expectedResolved []string
	}{
		{
			name:             testDetermineRootsArgumentsCaseConstant,
			arguments:        []string{"  " + tildeArgument + "\t"},
			configured:       []string{configuredRoot},
			expectedResolved: []string{expectedExpanded},
		},
		{
			name:             testDetermineRootsConfigurationCase,
			arguments:        []string{"", testBooleanLiteralTrueConstant, testBooleanLiteralFalseConstant},
			configured:       []string{"  " + tildeArgument + "  "},
			expectedResolved: []string{expectedExpanded},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			subTest.Helper()

			resolved := determineRepositoryRoots(testCase.arguments, testCase.configured)
			require.Equal(subTest, testCase.expectedResolved, resolved)
		})
	}
}
