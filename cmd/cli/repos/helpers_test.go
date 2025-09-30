package repos

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
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
		flagArgs         []string
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
		{
			name:       "flag_values_take_precedence",
			arguments:  []string{testBooleanLiteralTrueConstant},
			configured: []string{configuredRoot},
			flagArgs: []string{
				"--" + flagutils.DefaultRootFlagName,
				filepath.Join("~", "flag-root"),
			},
			expectedResolved: []string{filepath.Join(homeDirectory, "flag-root")},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			subTest.Helper()

			command := &cobra.Command{}
			flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
			if len(testCase.flagArgs) > 0 {
				parseError := command.ParseFlags(testCase.flagArgs)
				require.NoError(subTest, parseError)
			}

			resolved := determineRepositoryRoots(command, testCase.arguments, testCase.configured)
			require.Equal(subTest, testCase.expectedResolved, resolved)
		})
	}
}

type stubConfirmationPrompter struct {
	results []shared.ConfirmationResult
	err     error
	calls   int
}

func (prompter *stubConfirmationPrompter) Confirm(string) (shared.ConfirmationResult, error) {
	prompter.calls++
	if prompter.err != nil {
		return shared.ConfirmationResult{}, prompter.err
	}
	if len(prompter.results) == 0 {
		return shared.ConfirmationResult{}, nil
	}
	index := prompter.calls - 1
	if index >= len(prompter.results) {
		index = len(prompter.results) - 1
	}
	return prompter.results[index], nil
}

func TestCascadingConfirmationPrompterBehavior(testInstance *testing.T) {
	testCases := []struct {
		name                 string
		initialAssumeYes     bool
		responses            []shared.ConfirmationResult
		promptError          error
		expectAssumeYesAfter bool
		expectError          bool
		expectedPromptCalls  int
	}{
		{
			name:                 "initial_assume_yes_persists",
			initialAssumeYes:     true,
			expectAssumeYesAfter: true,
		},
		{
			name:                 "decline_does_not_set_assume_yes",
			responses:            []shared.ConfirmationResult{{Confirmed: false}},
			expectAssumeYesAfter: false,
			expectedPromptCalls:  1,
		},
		{
			name:                 "single_accept_does_not_set_assume_yes",
			responses:            []shared.ConfirmationResult{{Confirmed: true}},
			expectAssumeYesAfter: false,
			expectedPromptCalls:  1,
		},
		{
			name:                 "apply_all_sets_assume_yes",
			responses:            []shared.ConfirmationResult{{Confirmed: true, ApplyToAll: true}},
			expectAssumeYesAfter: true,
			expectedPromptCalls:  1,
		},
		{
			name:                 "propagates_prompt_error",
			responses:            []shared.ConfirmationResult{{Confirmed: true}},
			promptError:          errors.New("prompt failure"),
			expectAssumeYesAfter: false,
			expectError:          true,
			expectedPromptCalls:  1,
		},
		{
			name:                 "nil_base_prompter_returns_zero_result",
			expectAssumeYesAfter: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			var basePrompter shared.ConfirmationPrompter
			if testCase.responses != nil || testCase.promptError != nil {
				basePrompter = &stubConfirmationPrompter{results: testCase.responses, err: testCase.promptError}
			}
			wrappedPrompter := newCascadingConfirmationPrompter(basePrompter, testCase.initialAssumeYes)

			if testCase.responses != nil || testCase.promptError != nil {
				_, confirmError := wrappedPrompter.Confirm("test prompt")
				if testCase.expectError {
					require.Error(subTest, confirmError)
				} else {
					require.NoError(subTest, confirmError)
				}
			}

			require.Equal(subTest, testCase.expectAssumeYesAfter, wrappedPrompter.AssumeYes())

			if stub, ok := basePrompter.(*stubConfirmationPrompter); ok {
				require.Equal(subTest, testCase.expectedPromptCalls, stub.calls)
			} else {
				require.Zero(subTest, testCase.expectedPromptCalls)
			}
		})
	}
}
