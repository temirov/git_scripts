// Package flags provides helpers for binding standardized execution flags to Cobra commands.
package flags

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ExecutionDefaults describes default flag values shared across commands.
type ExecutionDefaults struct {
	DryRun       bool
	AssumeYes    bool
	RequireClean bool
}

// ExecutionFlagDefinition captures a single flag's configuration.
type ExecutionFlagDefinition struct {
	Name      string
	Usage     string
	Shorthand string
	Enabled   bool
}

// ExecutionFlagDefinitions groups execution flag definitions.
type ExecutionFlagDefinitions struct {
	DryRun       ExecutionFlagDefinition
	AssumeYes    ExecutionFlagDefinition
	RequireClean ExecutionFlagDefinition
}

// BindExecutionFlags attaches standardized execution flags to the provided command using persistent scope.
func BindExecutionFlags(command *cobra.Command, defaults ExecutionDefaults, definitions ExecutionFlagDefinitions) {
	if command == nil {
		return
	}

	persistentFlagSet := command.PersistentFlags()

	bindBoolFlag(persistentFlagSet, definitions.DryRun, defaults.DryRun)
	bindBoolFlag(persistentFlagSet, definitions.AssumeYes, defaults.AssumeYes)
	bindBoolFlag(persistentFlagSet, definitions.RequireClean, defaults.RequireClean)
}

func bindBoolFlag(flagSet *pflag.FlagSet, definition ExecutionFlagDefinition, defaultValue bool) {
	if flagSet == nil {
		return
	}
	if !definition.Enabled {
		return
	}
	if len(definition.Name) == 0 {
		return
	}

	if len(definition.Shorthand) > 0 {
		flagSet.BoolP(definition.Name, definition.Shorthand, defaultValue, definition.Usage)
		return
	}

	flagSet.Bool(definition.Name, defaultValue, definition.Usage)
}
