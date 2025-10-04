package flags

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAddToggleFlagParsesValues(t *testing.T) {
	testCases := []struct {
		name            string
		arguments       []string
		expectedValue   bool
		expectedChanged bool
	}{
		{name: "DefaultFalse", arguments: []string{}, expectedValue: false, expectedChanged: false},
		{name: "ImplicitTrue", arguments: []string{"--toggle"}, expectedValue: true, expectedChanged: true},
		{name: "ExplicitYes", arguments: []string{"--toggle", "yes"}, expectedValue: true, expectedChanged: true},
		{name: "ExplicitTrueUppercase", arguments: []string{"--toggle", "TRUE"}, expectedValue: true, expectedChanged: true},
		{name: "ExplicitNo", arguments: []string{"--toggle", "no"}, expectedValue: false, expectedChanged: true},
		{name: "ExplicitFalseUppercase", arguments: []string{"--toggle", "FALSE"}, expectedValue: false, expectedChanged: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			command := &cobra.Command{}

			var toggleValue bool
			AddToggleFlag(command.Flags(), &toggleValue, "toggle", "", false, "Toggle flag")

			normalizedArguments := NormalizeToggleArguments(testCase.arguments)
			parseError := command.ParseFlags(normalizedArguments)
			require.NoError(t, parseError)

			require.Equal(t, testCase.expectedValue, toggleValue)

			flag := command.Flags().Lookup("toggle")
			require.NotNil(t, flag)
			require.Equal(t, testCase.expectedChanged, flag.Changed)
		})
	}
}

func TestAddToggleFlagRejectsInvalidValues(t *testing.T) {
	command := &cobra.Command{}

	var toggleValue bool
	AddToggleFlag(command.Flags(), &toggleValue, "toggle", "", false, "Toggle flag")

	normalizedArguments := NormalizeToggleArguments([]string{"--toggle", "maybe"})
	parseError := command.ParseFlags(normalizedArguments)
	require.Error(t, parseError)

	require.Equal(t, false, toggleValue)

	flag := command.Flags().Lookup("toggle")
	require.NotNil(t, flag)
	require.False(t, flag.Changed)
}

func TestNormalizeToggleArgumentsHandlesShorthand(t *testing.T) {
	command := &cobra.Command{}

	var toggleValue bool
	AddToggleFlag(command.Flags(), &toggleValue, "toggle", "t", false, "Toggle flag")

	normalizedArguments := NormalizeToggleArguments([]string{"-t", "no"})
	parseError := command.ParseFlags(normalizedArguments)
	require.NoError(t, parseError)

	require.False(t, toggleValue)

	flag := command.Flags().Lookup("toggle")
	require.NotNil(t, flag)
	require.True(t, flag.Changed)
}
