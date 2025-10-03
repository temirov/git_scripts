package repos

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/rename"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

const (
	renameUseConstant             = "repo-folders-rename"
	renameShortDescription        = "Rename repository directories to match canonical GitHub names"
	renameLongDescription         = "repo-folders-rename normalizes repository directory names to match canonical GitHub repositories."
	renameRequireCleanFlagName    = "require-clean"
	renameRequireCleanDescription = "Require clean worktrees before applying renames"
	renameIncludeOwnerFlagName    = "owner"
	renameIncludeOwnerDescription = "Include repository owner in the target directory path"
)

// RenameCommandBuilder assembles the repo-folders-rename command.
type RenameCommandBuilder struct {
	LoggerProvider               LoggerProvider
	Discoverer                   shared.RepositoryDiscoverer
	GitExecutor                  shared.GitExecutor
	GitManager                   shared.GitRepositoryManager
	GitHubResolver               shared.GitHubMetadataResolver
	FileSystem                   shared.FileSystem
	PrompterFactory              PrompterFactory
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() RenameConfiguration
}

// Build constructs the repo-folders-rename command.
func (builder *RenameCommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   renameUseConstant,
		Short: renameShortDescription,
		Long:  renameLongDescription,
		Args:  cobra.NoArgs,
		RunE:  builder.run,
	}

	command.Flags().Bool(renameRequireCleanFlagName, false, renameRequireCleanDescription)
	command.Flags().Bool(renameIncludeOwnerFlagName, false, renameIncludeOwnerDescription)

	return command, nil
}

func (builder *RenameCommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()
	executionFlags, executionFlagsAvailable := flagutils.ResolveExecutionFlags(command)

	dryRun := configuration.DryRun
	if executionFlagsAvailable && executionFlags.DryRunSet {
		dryRun = executionFlags.DryRun
	}

	assumeYes := configuration.AssumeYes
	if executionFlagsAvailable && executionFlags.AssumeYesSet {
		assumeYes = executionFlags.AssumeYes
	}

	requireClean := configuration.RequireCleanWorktree
	if command != nil && command.Flags().Changed(renameRequireCleanFlagName) {
		requireClean, _ = command.Flags().GetBool(renameRequireCleanFlagName)
	}

	includeOwner := configuration.IncludeOwner
	if command != nil && command.Flags().Changed(renameIncludeOwnerFlagName) {
		includeOwner, _ = command.Flags().GetBool(renameIncludeOwnerFlagName)
	}

	roots, rootsError := requireRepositoryRoots(command, arguments, configuration.RepositoryRoots)
	if rootsError != nil {
		return rootsError
	}

	logger := resolveLogger(builder.LoggerProvider)
	humanReadableLogging := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadableLogging = builder.HumanReadableLoggingProvider()
	}
	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, humanReadableLogging)
	if executorError != nil {
		return executorError
	}

	gitManager, managerError := dependencies.ResolveGitRepositoryManager(builder.GitManager, gitExecutor)
	if managerError != nil {
		return managerError
	}

	githubResolver, resolverError := dependencies.ResolveGitHubResolver(builder.GitHubResolver, gitExecutor)
	if resolverError != nil {
		return resolverError
	}

	repositoryDiscoverer := dependencies.ResolveRepositoryDiscoverer(builder.Discoverer)
	fileSystem := dependencies.ResolveFileSystem(builder.FileSystem)

	prompter := resolvePrompter(builder.PrompterFactory, command)

	service := audit.NewService(repositoryDiscoverer, gitManager, gitExecutor, githubResolver, command.OutOrStdout(), command.ErrOrStderr())

	inspections, inspectionError := service.DiscoverInspections(command.Context(), roots, false, audit.InspectionDepthMinimal)
	if inspectionError != nil {
		return inspectionError
	}

	trackingPrompter := newCascadingConfirmationPrompter(prompter, assumeYes)
	renameDependencies := rename.Dependencies{
		FileSystem: fileSystem,
		GitManager: gitManager,
		Prompter:   trackingPrompter,
		Clock:      shared.SystemClock{},
		Output:     command.OutOrStdout(),
		Errors:     command.ErrOrStderr(),
	}

	directoryPlanner := rename.NewDirectoryPlanner()
	for _, inspection := range inspections {
		plan := directoryPlanner.Plan(includeOwner, inspection.FinalOwnerRepo, inspection.DesiredFolderName)
		if plan.IsNoop(inspection.Path, inspection.FolderName) {
			continue
		}
		if len(strings.TrimSpace(plan.FolderName)) == 0 {
			continue
		}

		renameOptions := rename.Options{
			RepositoryPath:          inspection.Path,
			DesiredFolderName:       plan.FolderName,
			DryRun:                  dryRun,
			RequireCleanWorktree:    requireClean,
			AssumeYes:               trackingPrompter.AssumeYes(),
			IncludeOwner:            plan.IncludeOwner,
			EnsureParentDirectories: plan.IncludeOwner,
		}

		rename.Execute(command.Context(), renameDependencies, renameOptions)
	}

	return nil
}

func (builder *RenameCommandBuilder) resolveConfiguration() RenameConfiguration {
	if builder.ConfigurationProvider == nil {
		defaults := DefaultToolsConfiguration()
		return defaults.Rename
	}

	provided := builder.ConfigurationProvider()
	return provided.sanitize()
}
