package repos

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/rename"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

const (
	renameUseConstant                 = "repo-folders-rename"
	renameShortDescription            = "Rename repository directories to match canonical GitHub names"
	renameLongDescription             = "repo-folders-rename normalizes repository directory names to match canonical GitHub repositories."
	renameRequireCleanFlagName        = "require-clean"
	renameRequireCleanDescription     = "Require clean worktrees before applying renames"
	renameIncludeOwnerFlagName        = "owner"
	renameIncludeOwnerDescription     = "Include repository owner in the target directory path"
	pathSeparatorForwardSlashConstant = "/"
	parentDirectoryTokenConstant      = ".."
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

	flagutils.AddToggleFlag(command.Flags(), nil, renameRequireCleanFlagName, "", false, renameRequireCleanDescription)
	flagutils.AddToggleFlag(command.Flags(), nil, renameIncludeOwnerFlagName, "", false, renameIncludeOwnerDescription)

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
	if command != nil {
		requireCleanFlagValue, requireCleanFlagChanged, requireCleanFlagError := flagutils.BoolFlag(command, renameRequireCleanFlagName)
		if requireCleanFlagError != nil && !errors.Is(requireCleanFlagError, flagutils.ErrFlagNotDefined) {
			return requireCleanFlagError
		}
		if requireCleanFlagChanged {
			requireClean = requireCleanFlagValue
		}
	}

	includeOwner := configuration.IncludeOwner
	if command != nil {
		includeOwnerFlagValue, includeOwnerFlagChanged, includeOwnerFlagError := flagutils.BoolFlag(command, renameIncludeOwnerFlagName)
		if includeOwnerFlagError != nil && !errors.Is(includeOwnerFlagError, flagutils.ErrFlagNotDefined) {
			return includeOwnerFlagError
		}
		if includeOwnerFlagChanged {
			includeOwner = includeOwnerFlagValue
		}
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

	inspections, inspectionError := service.DiscoverInspections(command.Context(), roots, false, false, audit.InspectionDepthMinimal)
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
	renameRequests := prepareRenameRequests(inspections, directoryPlanner, includeOwner, dryRun, requireClean)
	relaxedRepositories := determineCleanRelaxations(command.Context(), gitManager, renameRequests, requireClean)
	for _, request := range renameRequests {
		options := request.options
		if relaxedRepositories[options.RepositoryPath] {
			options.RequireCleanWorktree = false
		}
		options.AssumeYes = trackingPrompter.AssumeYes()
		rename.Execute(command.Context(), renameDependencies, options)
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

type renameRequest struct {
	options   rename.Options
	pathDepth int
}

func prepareRenameRequests(
	inspections []audit.RepositoryInspection,
	directoryPlanner rename.DirectoryPlanner,
	includeOwner bool,
	dryRun bool,
	requireClean bool,
) []renameRequest {
	renameRequests := make([]renameRequest, 0, len(inspections))

	for _, inspection := range inspections {
		plan := directoryPlanner.Plan(includeOwner, inspection.FinalOwnerRepo, inspection.DesiredFolderName)
		desiredFolderName := plan.FolderName
		if plan.IsNoop(inspection.Path, inspection.FolderName) {
			desiredFolderName = filepath.Base(inspection.Path)
		}
		trimmedFolderName := strings.TrimSpace(desiredFolderName)
		if len(trimmedFolderName) == 0 {
			continue
		}

		request := renameRequest{
			options: rename.Options{
				RepositoryPath:          inspection.Path,
				DesiredFolderName:       trimmedFolderName,
				DryRun:                  dryRun,
				RequireCleanWorktree:    requireClean,
				IncludeOwner:            plan.IncludeOwner,
				EnsureParentDirectories: plan.IncludeOwner,
			},
			pathDepth: calculatePathDepth(inspection.Path),
		}

		renameRequests = append(renameRequests, request)
	}

	sort.SliceStable(renameRequests, func(firstIndex int, secondIndex int) bool {
		firstRequest := renameRequests[firstIndex]
		secondRequest := renameRequests[secondIndex]
		if firstRequest.pathDepth == secondRequest.pathDepth {
			return firstRequest.options.RepositoryPath < secondRequest.options.RepositoryPath
		}
		return firstRequest.pathDepth > secondRequest.pathDepth
	})

	return renameRequests
}

func calculatePathDepth(path string) int {
	cleanedPath := filepath.Clean(path)
	if len(cleanedPath) == 0 || cleanedPath == "." {
		return 0
	}
	normalizedPath := filepath.ToSlash(cleanedPath)
	return strings.Count(normalizedPath, pathSeparatorForwardSlashConstant)
}

func determineCleanRelaxations(
	executionContext context.Context,
	gitManager shared.GitRepositoryManager,
	renameRequests []renameRequest,
	requireClean bool,
) map[string]bool {
	relaxed := make(map[string]bool)
	if !requireClean || gitManager == nil {
		return relaxed
	}

	ancestorRepositories := identifyAncestorRepositories(renameRequests)
	for repositoryPath := range ancestorRepositories {
		clean, cleanError := gitManager.CheckCleanWorktree(executionContext, repositoryPath)
		if cleanError != nil {
			continue
		}
		if clean {
			relaxed[repositoryPath] = true
		}
	}
	return relaxed
}

func identifyAncestorRepositories(renameRequests []renameRequest) map[string]struct{} {
	ancestors := make(map[string]struct{})
	for firstIndex := range renameRequests {
		firstPath := renameRequests[firstIndex].options.RepositoryPath
		for secondIndex := range renameRequests {
			if firstIndex == secondIndex {
				continue
			}
			secondPath := renameRequests[secondIndex].options.RepositoryPath
			if isAncestorPath(firstPath, secondPath) {
				ancestors[firstPath] = struct{}{}
			}
		}
	}
	return ancestors
}

func isAncestorPath(potentialAncestor string, potentialDescendant string) bool {
	relativePath, relativeError := filepath.Rel(potentialAncestor, potentialDescendant)
	if relativeError != nil {
		return false
	}
	if relativePath == "." {
		return false
	}
	return !strings.HasPrefix(relativePath, parentDirectoryTokenConstant)
}
