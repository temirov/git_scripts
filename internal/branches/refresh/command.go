package refresh

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/shared"
	"github.com/temirov/gix/internal/utils"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	commandUseConstant                      = "branch-refresh"
	commandShortDescriptionConstant         = "Fetch, checkout, and pull a branch"
	commandLongDescriptionConstant          = "branch-refresh synchronizes a repository branch by fetching updates, checking out the branch, and pulling the latest changes."
	stashFlagNameConstant                   = "stash"
	stashFlagDescriptionConstant            = "Stash local changes before refreshing the branch"
	commitFlagNameConstant                  = "commit"
	commitFlagDescriptionConstant           = "Commit local changes before refreshing the branch"
	missingBranchNameMessageConstant        = "branch name is required; supply --branch"
	conflictingRecoveryFlagsMessageConstant = "use at most one of --stash or --commit"
	refreshSuccessMessageTemplateConstant   = "REFRESHED: %s (%s)\n"
)

// LoggerProvider yields a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the branch-refresh command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	GitExecutor                  shared.GitExecutor
	GitRepositoryManager         shared.GitRepositoryManager
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the branch-refresh command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandUseConstant,
		Short: commandShortDescriptionConstant,
		Long:  commandLongDescriptionConstant,
		Args:  cobra.NoArgs,
		RunE:  builder.run,
	}

	command.Flags().Bool(stashFlagNameConstant, false, stashFlagDescriptionConstant)
	command.Flags().Bool(commitFlagNameConstant, false, commitFlagDescriptionConstant)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()

	branchName := strings.TrimSpace(configuration.BranchName)
	contextAccessor := utils.NewCommandContextAccessor()
	if branchContext, exists := contextAccessor.BranchContext(command.Context()); exists {
		if len(strings.TrimSpace(branchContext.Name)) > 0 {
			branchName = branchContext.Name
		}
	}
	if len(branchName) == 0 {
		if command != nil {
			_ = command.Help()
		}
		return errors.New(missingBranchNameMessageConstant)
	}

	stashRequested, stashFlagError := command.Flags().GetBool(stashFlagNameConstant)
	if stashFlagError != nil {
		return stashFlagError
	}
	commitRequested, commitFlagError := command.Flags().GetBool(commitFlagNameConstant)
	if commitFlagError != nil {
		return commitFlagError
	}
	if stashRequested && commitRequested {
		return errors.New(conflictingRecoveryFlagsMessageConstant)
	}

	repositoryRoots, rootsError := rootutils.Resolve(command, arguments, configuration.RepositoryRoots)
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

	repositoryManager, managerError := dependencies.ResolveGitRepositoryManager(builder.GitRepositoryManager, gitExecutor)
	if managerError != nil {
		return managerError
	}

	service, serviceCreationError := NewService(Dependencies{GitExecutor: gitExecutor, RepositoryManager: repositoryManager})
	if serviceCreationError != nil {
		return serviceCreationError
	}

	for _, repositoryPath := range repositoryRoots {
		_, refreshError := service.Refresh(command.Context(), Options{
			RepositoryPath: repositoryPath,
			BranchName:     branchName,
			RequireClean:   true,
			StashChanges:   stashRequested,
			CommitChanges:  commitRequested,
		})
		if refreshError != nil {
			return refreshError
		}
		fmt.Fprintf(command.OutOrStdout(), refreshSuccessMessageTemplateConstant, repositoryPath, branchName)
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
