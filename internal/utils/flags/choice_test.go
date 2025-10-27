package flags

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatChoiceUsage(t *testing.T) {
	testCases := []struct {
		name           string
		defaultChoice  string
		choices        []string
		description    string
		expectedOutput string
	}{
		{
			name:           "DefaultFirstChoice",
			defaultChoice:  "local",
			choices:        []string{"local", "user"},
			description:    "Write configuration to LOCAL or user scope.",
			expectedOutput: "`<LOCAL|user>` Write configuration to LOCAL or user scope.",
		},
		{
			name:           "DefaultSecondChoice",
			defaultChoice:  "user",
			choices:        []string{"local", "user"},
			description:    "Persist configuration for the selected scope.",
			expectedOutput: "`<local|USER>` Persist configuration for the selected scope.",
		},
		{
			name:           "EmptyDescription",
			defaultChoice:  "alpha",
			choices:        []string{"alpha", "beta"},
			description:    "",
			expectedOutput: "`<ALPHA|beta>`",
		},
		{
			name:           "DuplicateChoicesIgnored",
			defaultChoice:  "beta",
			choices:        []string{"beta", "beta", "alpha", "alpha"},
			description:    "Select between options.",
			expectedOutput: "`<BETA|alpha>` Select between options.",
		},
		{
			name:           "WhitespaceTrimmed",
			defaultChoice:  "primary",
			choices:        []string{" primary ", " secondary "},
			description:    "Pick a palette.",
			expectedOutput: "`<PRIMARY|secondary>` Pick a palette.",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			actual := FormatChoiceUsage(testCase.defaultChoice, testCase.choices, testCase.description)
			require.Equal(t, testCase.expectedOutput, actual)
		})
	}
}
