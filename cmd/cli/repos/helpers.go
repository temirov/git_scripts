package repos

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/repos/prompt"
	"github.com/temirov/git_scripts/internal/repos/shared"
	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

const (
	missingRepositoryRootsErrorMessageConstant = "no repository roots provided; specify --root or configure defaults"
)

var repositoryHomeDirectoryExpander = pathutils.NewHomeExpander()

// LoggerProvider yields a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// PrompterFactory creates confirmation prompters scoped to a Cobra command.
type PrompterFactory func(*cobra.Command) shared.ConfirmationPrompter

func determineRepositoryRoots(arguments []string, configuredRoots []string) []string {
	roots := trimRoots(arguments)
	if len(roots) > 0 {
		return roots
	}

	configured := trimRoots(configuredRoots)
	if len(configured) > 0 {
		return configured
	}

	return nil
}

func requireRepositoryRoots(command *cobra.Command, arguments []string, configuredRoots []string) ([]string, error) {
	resolvedRoots := determineRepositoryRoots(arguments, configuredRoots)
	if len(resolvedRoots) > 0 {
		return resolvedRoots, nil
	}

	if command != nil {
		_ = command.Help()
	}

	return nil, errors.New(missingRepositoryRootsErrorMessageConstant)
}

func trimRoots(raw []string) []string {
	trimmed := make([]string, 0, len(raw))
	for _, argument := range raw {
		candidate := strings.TrimSpace(argument)
		if len(candidate) == 0 {
			continue
		}
		if isBooleanLiteral(candidate) {
			continue
		}
		expanded := repositoryHomeDirectoryExpander.Expand(candidate)
		trimmed = append(trimmed, expanded)
	}
	return trimmed
}

func isBooleanLiteral(value string) bool {
	switch strings.ToLower(value) {
	case "true", "false":
		return true
	default:
		return false
	}
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
