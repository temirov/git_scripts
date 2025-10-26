package cd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/shared"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	commandUseNameConstant               = "branch-cd"
	commandUsageTemplateConstant         = commandUseNameConstant + " <branch>"
	commandExampleTemplateConstant       = "gix branch cd feature/new-branch --roots ~/Development"
	commandShortDescriptionConstant      = "Switch repositories to the selected branch"
	commandLongDescriptionConstant       = "branch-cd fetches updates, switches to the requested branch, creates it if missing, and rebases onto the remote for each repository root. Provide the branch name as the first argument before any optional repository roots or flags, or configure a default branch in the application settings."
	missingBranchMessageConstant         = "branch name is required; provide it as the first argument or configure a default"
	changeSuccessMessageTemplateConstant = "SWITCHED: %s -> %s"
	changeCreatedSuffixConstant          = " (created)"
)

// LoggerProvider yields a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the branch-cd command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	GitExecutor                  shared.GitExecutor
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the branch-cd command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:     commandUsageTemplateConstant,
		Short:   commandShortDescriptionConstant,
		Long:    commandLongDescriptionConstant,
		RunE:    builder.run,
		Args:    cobra.ArbitraryArgs,
		Example: commandExampleTemplateConstant,
	}

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()

	executionFlags, executionFlagsAvailable := flagutils.ResolveExecutionFlags(command)
	dryRun := false
	if executionFlagsAvailable && executionFlags.DryRunSet {
		dryRun = executionFlags.DryRun
	}

	branchName, remainingArgs := builder.resolveBranchName(command, arguments, configuration)
	if branchName == "" {
		if command != nil {
			_ = command.Help()
		}
		return errors.New(missingBranchMessageConstant)
	}

	remoteName := strings.TrimSpace(configuration.RemoteName)
	if executionFlagsAvailable && executionFlags.RemoteSet {
		overridden := strings.TrimSpace(executionFlags.Remote)
		if len(overridden) > 0 {
			remoteName = overridden
		}
	}

	repositoryRoots, rootsError := rootutils.Resolve(command, remainingArgs, configuration.RepositoryRoots)
	if rootsError != nil {
		return rootsError
	}

	logger := builder.resolveLogger()
	humanReadableLogging := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadableLogging = builder.HumanReadableLoggingProvider()
	}

	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, humanReadableLogging)
	if executorError != nil {
		return executorError
	}

	service, serviceError := NewService(ServiceDependencies{GitExecutor: gitExecutor})
	if serviceError != nil {
		return serviceError
	}

	createIfMissing := configuration.CreateIfMissing
	if len(remoteName) == 0 {
		remoteName = defaultRemoteNameConstant
	}

	for _, repository := range repositoryRoots {
		result, changeError := service.Change(command.Context(), Options{
			RepositoryPath:  repository,
			BranchName:      branchName,
			RemoteName:      remoteName,
			CreateIfMissing: createIfMissing,
			DryRun:          dryRun,
		})
		if changeError != nil {
			return changeError
		}

		message := fmt.Sprintf(changeSuccessMessageTemplateConstant, result.RepositoryPath, result.BranchName)
		if result.BranchCreated && !dryRun {
			message += changeCreatedSuffixConstant
		}
		fmt.Fprintln(command.OutOrStdout(), message)
	}

	return nil
}

func (builder *CommandBuilder) resolveConfiguration() CommandConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultCommandConfiguration()
	}
	return builder.ConfigurationProvider().Sanitize()
}

func (builder *CommandBuilder) resolveLogger() *zap.Logger {
	if builder.LoggerProvider == nil {
		return zap.NewNop()
	}
	logger := builder.LoggerProvider()
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

func (builder *CommandBuilder) resolveBranchName(command *cobra.Command, arguments []string, configuration CommandConfiguration) (string, []string) {
	remaining := arguments
	if len(remaining) > 0 {
		branch := strings.TrimSpace(remaining[0])
		return branch, remaining[1:]
	}

	accessor := utils.NewCommandContextAccessor()
	if branchContext, exists := accessor.BranchContext(command.Context()); exists {
		trimmed := strings.TrimSpace(branchContext.Name)
		if len(trimmed) > 0 {
			return trimmed, remaining
		}
	}

	defaultBranch := strings.TrimSpace(configuration.DefaultBranch)
	if len(defaultBranch) > 0 {
		return defaultBranch, remaining
	}

	return "", remaining
}
