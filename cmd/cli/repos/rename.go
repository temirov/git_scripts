package repos

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/rename"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	renameUseConstant             = "repo-folders-rename [root ...]"
	renameShortDescription        = "Rename repository directories to match canonical GitHub names"
	renameLongDescription         = "repo-folders-rename normalizes repository directory names to match canonical GitHub repositories."
	renameDryRunFlagName          = "dry-run"
	renameDryRunFlagDescription   = "Preview rename actions without making changes"
	renameAssumeYesFlagName       = "yes"
	renameAssumeYesFlagShorthand  = "y"
	renameAssumeYesDescription    = "Automatically confirm rename prompts"
	renameRequireCleanFlagName    = "require-clean"
	renameRequireCleanDescription = "Require clean worktrees before applying renames"
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
		RunE:  builder.run,
	}

	command.Flags().Bool(renameDryRunFlagName, false, renameDryRunFlagDescription)
	command.Flags().BoolP(renameAssumeYesFlagName, renameAssumeYesFlagShorthand, false, renameAssumeYesDescription)
	command.Flags().Bool(renameRequireCleanFlagName, false, renameRequireCleanDescription)

	return command, nil
}

func (builder *RenameCommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()

	dryRun := configuration.DryRun
	if command != nil && command.Flags().Changed(renameDryRunFlagName) {
		dryRun, _ = command.Flags().GetBool(renameDryRunFlagName)
	}

	assumeYes := configuration.AssumeYes
	if command != nil && command.Flags().Changed(renameAssumeYesFlagName) {
		assumeYes, _ = command.Flags().GetBool(renameAssumeYesFlagName)
	}

	requireClean := configuration.RequireCleanWorktree
	if command != nil && command.Flags().Changed(renameRequireCleanFlagName) {
		requireClean, _ = command.Flags().GetBool(renameRequireCleanFlagName)
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

	inspections, inspectionError := service.DiscoverInspections(command.Context(), roots, false)
	if inspectionError != nil {
		return inspectionError
	}

	renameDependencies := rename.Dependencies{
		FileSystem: fileSystem,
		GitManager: gitManager,
		Prompter:   prompter,
		Clock:      shared.SystemClock{},
		Output:     command.OutOrStdout(),
		Errors:     command.ErrOrStderr(),
	}

	for _, inspection := range inspections {
		if len(strings.TrimSpace(inspection.DesiredFolderName)) == 0 {
			continue
		}
		if inspection.DesiredFolderName == inspection.FolderName {
			continue
		}

		renameOptions := rename.Options{
			RepositoryPath:       inspection.Path,
			DesiredFolderName:    inspection.DesiredFolderName,
			DryRun:               dryRun,
			RequireCleanWorktree: requireClean,
			AssumeYes:            assumeYes,
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
