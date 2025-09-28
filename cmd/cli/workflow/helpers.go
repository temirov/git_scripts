package workflow

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/repos/prompt"
	"github.com/temirov/gix/internal/repos/shared"
)

// LoggerProvider yields a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// PrompterFactory constructs confirmation prompters scoped to a command.
type PrompterFactory func(*cobra.Command) shared.ConfirmationPrompter

// DetermineRoots selects the effective repository roots from flags and configuration.
func DetermineRoots(flagValues []string, configured []string, preferFlag bool) []string {
	if preferFlag {
		trimmed := workflowConfigurationRepositoryPathSanitizer.Sanitize(flagValues)
		if len(trimmed) > 0 {
			return trimmed
		}
	}

	configuredRoots := workflowConfigurationRepositoryPathSanitizer.Sanitize(configured)
	if len(configuredRoots) > 0 {
		return configuredRoots
	}

	trimmedFlagRoots := workflowConfigurationRepositoryPathSanitizer.Sanitize(flagValues)
	if len(trimmedFlagRoots) > 0 {
		return trimmedFlagRoots
	}

	return nil
}

func resolveLogger(provider LoggerProvider) *zap.Logger {
	if provider == nil {
		return zap.NewNop()
	}
	logger := provider()
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

func resolvePrompter(factory PrompterFactory, command *cobra.Command) shared.ConfirmationPrompter {
	if factory != nil {
		prompter := factory(command)
		if prompter != nil {
			return prompter
		}
	}
	return prompt.NewIOConfirmationPrompter(command.InOrStdin(), command.OutOrStdout())
}

func displayCommandHelp(command *cobra.Command) error {
	if command == nil {
		return nil
	}
	return command.Help()
}
