package repos

import (
	"errors"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/repos/prompt"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	pathutils "github.com/temirov/gix/internal/utils/path"
)

const (
	missingRepositoryRootsErrorMessageConstant = "no repository roots provided; specify --root or configure defaults"
)

var repositoryPathSanitizer = pathutils.NewRepositoryPathSanitizerWithConfiguration(nil, pathutils.RepositoryPathSanitizerConfiguration{ExcludeBooleanLiteralCandidates: true})

// LoggerProvider yields a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// PrompterFactory creates confirmation prompters scoped to a Cobra command.
type PrompterFactory func(*cobra.Command) shared.ConfirmationPrompter

func determineRepositoryRoots(command *cobra.Command, arguments []string, configuredRoots []string) []string {
	flagRoots := resolveRootFlagValues(command)
	if len(flagRoots) > 0 {
		return flagRoots
	}

	argumentRoots := repositoryPathSanitizer.Sanitize(arguments)
	if len(argumentRoots) > 0 {
		return argumentRoots
	}

	configured := repositoryPathSanitizer.Sanitize(configuredRoots)
	if len(configured) > 0 {
		return configured
	}

	return nil
}

func requireRepositoryRoots(command *cobra.Command, arguments []string, configuredRoots []string) ([]string, error) {
	resolvedRoots := determineRepositoryRoots(command, arguments, configuredRoots)
	if len(resolvedRoots) > 0 {
		return resolvedRoots, nil
	}

	if command != nil {
		_ = command.Help()
	}

	return nil, errors.New(missingRepositoryRootsErrorMessageConstant)
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

// cascadingConfirmationPrompter forwards confirmations while tracking apply-to-all decisions.
type cascadingConfirmationPrompter struct {
	basePrompter shared.ConfirmationPrompter
	assumeYes    bool
}

func newCascadingConfirmationPrompter(base shared.ConfirmationPrompter, initialAssumeYes bool) *cascadingConfirmationPrompter {
	return &cascadingConfirmationPrompter{basePrompter: base, assumeYes: initialAssumeYes}
}

func (prompter *cascadingConfirmationPrompter) Confirm(prompt string) (shared.ConfirmationResult, error) {
	if prompter.basePrompter == nil {
		return shared.ConfirmationResult{}, nil
	}
	result, err := prompter.basePrompter.Confirm(prompt)
	if err != nil {
		return shared.ConfirmationResult{}, err
	}
	if result.ApplyToAll {
		prompter.assumeYes = true
	}
	return result, nil
}

func (prompter *cascadingConfirmationPrompter) AssumeYes() bool {
	return prompter.assumeYes
}

func displayCommandHelp(command *cobra.Command) error {
	if command == nil {
		return nil
	}
	return command.Help()
}

func resolveRootFlagValues(command *cobra.Command) []string {
	if command == nil {
		return nil
	}
	roots, rootsError := command.Flags().GetStringSlice(flagutils.DefaultRootFlagName)
	if rootsError != nil {
		return nil
	}
	return repositoryPathSanitizer.Sanitize(roots)
}
