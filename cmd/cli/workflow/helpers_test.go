package workflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/git_scripts/cmd/cli/workflow"
)

const (
	workflowHelpersAbsoluteSuffixConstant = "workflow-helpers-absolute"
	workflowHelpersRelativePathConstant   = "workflow/helpers/relative"
	workflowHelpersTildePathConstant      = "~/workflow/helpers/home"
	workflowHelpersTildePrefixConstant    = "~/"
	workflowHelpersWhitespace             = "   "
)

func TestDetermineRootsExpandsHomeDirectoryValues(testInstance *testing.T) {
	testInstance.Helper()

	homeDirectory, homeDirectoryError := os.UserHomeDir()
	require.NoError(testInstance, homeDirectoryError)

	trimmedTilde := strings.TrimPrefix(workflowHelpersTildePathConstant, workflowHelpersTildePrefixConstant)
	expectedTilde := filepath.Join(homeDirectory, trimmedTilde)

	temporaryRoot := testInstance.TempDir()
	absoluteRoot := filepath.Join(temporaryRoot, workflowHelpersAbsoluteSuffixConstant)

	testCases := []struct {
		name           string
		flagValues     []string
		configured     []string
		preferFlag     bool
		expectedResult []string
	}{
		{
			name:           "prefer_flag_values_with_home_expansion",
			flagValues:     []string{workflowHelpersWhitespace + workflowHelpersTildePathConstant + workflowHelpersWhitespace},
			configured:     []string{absoluteRoot},
			preferFlag:     true,
			expectedResult: []string{expectedTilde},
		},
		{
			name:           "configured_values_expand_home_directory",
			flagValues:     []string{workflowHelpersRelativePathConstant},
			configured:     []string{workflowHelpersTildePathConstant},
			preferFlag:     false,
			expectedResult: []string{expectedTilde},
		},
		{
			name:           "fall_back_to_flag_values_when_configuration_empty",
			flagValues:     []string{workflowHelpersRelativePathConstant},
			configured:     []string{workflowHelpersWhitespace},
			preferFlag:     false,
			expectedResult: []string{workflowHelpersRelativePathConstant},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			result := workflow.DetermineRoots(testCase.flagValues, testCase.configured, testCase.preferFlag)
			require.Equal(subTest, testCase.expectedResult, result)
		})
	}
}
