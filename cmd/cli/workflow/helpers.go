package workflow

import (
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/repos/prompt"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	defaultWorkflowRootConstant = "."
)

// LoggerProvider yields a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// PrompterFactory constructs confirmation prompters scoped to a command.
type PrompterFactory func(*cobra.Command) shared.ConfirmationPrompter

func determineRoots(raw []string) []string {
	trimmedRoots := make([]string, 0, len(raw))
	for index := range raw {
		trimmed := strings.TrimSpace(raw[index])
		if len(trimmed) == 0 {
			continue
		}
		trimmedRoots = append(trimmedRoots, trimmed)
	}
	if len(trimmedRoots) == 0 {
		return []string{defaultWorkflowRootConstant}
	}
	return trimmedRoots
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
