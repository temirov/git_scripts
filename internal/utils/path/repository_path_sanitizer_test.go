package pathutils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

const (
	testCaseAbsolutePathSuffixConstant          = "repository-path-sanitizer"
	testCaseTildeRelativePathConstant           = "Projects/example"
	testCaseWhitespacePrefixConstant            = "  "
	testCaseWhitespaceSuffixConstant            = "\t"
	testCaseBooleanLiteralTrueUppercaseConstant = "TRUE"
	testCaseBooleanLiteralFalseMixedConstant    = "False"
	testCaseSanitizerDefaultCaseNameConstant    = "default_configuration"
	testCaseBooleanFilterCaseNameConstant       = "boolean_filter_configuration"
)

func TestRepositoryPathSanitizerNormalizesInputs(testInstance *testing.T) {
	testInstance.Helper()

	homeDirectory, homeDirectoryError := os.UserHomeDir()
	require.NoError(testInstance, homeDirectoryError)

	temporaryDirectory := testInstance.TempDir()
	absolutePath := filepath.Join(temporaryDirectory, testCaseAbsolutePathSuffixConstant)
	tildeInput := filepath.Join("~", testCaseTildeRelativePathConstant)
	expandedTilde := filepath.Join(homeDirectory, testCaseTildeRelativePathConstant)

	testCases := []struct {
		name            string
		sanitizer       *pathutils.RepositoryPathSanitizer
		inputs          []string
		expectedOutputs []string
	}{
		{
			name:      testCaseSanitizerDefaultCaseNameConstant,
			sanitizer: pathutils.NewRepositoryPathSanitizer(),
			inputs: []string{
				"",
				testCaseWhitespacePrefixConstant + absolutePath + testCaseWhitespaceSuffixConstant,
				testCaseWhitespacePrefixConstant + tildeInput + testCaseWhitespaceSuffixConstant,
			},
			expectedOutputs: []string{absolutePath, expandedTilde},
		},
		{
			name:      testCaseBooleanFilterCaseNameConstant,
			sanitizer: pathutils.NewRepositoryPathSanitizerWithConfiguration(nil, pathutils.RepositoryPathSanitizerConfiguration{ExcludeBooleanLiteralCandidates: true}),
			inputs: []string{
				testCaseBooleanLiteralTrueUppercaseConstant,
				testCaseBooleanLiteralFalseMixedConstant,
				tildeInput,
			},
			expectedOutputs: []string{expandedTilde},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			subTest.Helper()

			sanitized := testCase.sanitizer.Sanitize(testCase.inputs)
			require.Equal(subTest, testCase.expectedOutputs, sanitized)
		})
	}
}

func TestRepositoryPathSanitizerReturnsNilForEmptyResults(testInstance *testing.T) {
	testInstance.Helper()

	sanitizer := pathutils.NewRepositoryPathSanitizer()

	sanitized := sanitizer.Sanitize([]string{"   ", "\n"})
	require.Nil(testInstance, sanitized)
}
