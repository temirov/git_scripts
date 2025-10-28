package repos

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	"github.com/temirov/gix/internal/workflow"
)

const (
	removeUseConstant              = "repo-history-remove"
	removeShortDescription         = "Rewrite repository history to remove selected paths"
	removeLongDescription          = "repo-history-remove purges the specified paths from repository history using git-filter-repo, optionally force-pushing updates and restoring upstream tracking."
	removeRemoteFlagName           = "remote"
	removeRemoteFlagDescription    = "Remote to use when pushing after purge (auto-detected when omitted)"
	removePushFlagName             = "push"
	removePushFlagDescription      = "Force push rewritten history to the configured remote"
	removeRestoreFlagName          = "restore"
	removeRestoreFlagDescription   = "Restore upstream tracking for local branches after purge"
	removePushMissingFlagName      = "push-missing"
	removePushMissingDescription   = "Create missing remote branches when restoring upstreams"
	removeMissingPathsErrorMessage = "history purge requires at least one path argument"
)

// RemoveCommandBuilder assembles the repo-history-remove command.
type RemoveCommandBuilder struct {
	LoggerProvider               LoggerProvider
	Discoverer                   shared.RepositoryDiscoverer
	GitExecutor                  shared.GitExecutor
	GitManager                   shared.GitRepositoryManager
	FileSystem                   shared.FileSystem
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() RemoveConfiguration
	TaskRunnerFactory            func(workflow.Dependencies) TaskRunnerExecutor
}

// Build constructs the repo-history-remove command.
func (builder *RemoveCommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   removeUseConstant,
		Short: removeShortDescription,
		Long:  removeLongDescription,
		RunE:  builder.run,
	}

	command.Flags().String(removeRemoteFlagName, "", removeRemoteFlagDescription)
	flagutils.AddToggleFlag(command.Flags(), nil, removePushFlagName, "", true, removePushFlagDescription)
	flagutils.AddToggleFlag(command.Flags(), nil, removeRestoreFlagName, "", true, removeRestoreFlagDescription)
	flagutils.AddToggleFlag(command.Flags(), nil, removePushMissingFlagName, "", false, removePushMissingDescription)

	return command, nil
}

func (builder *RemoveCommandBuilder) run(command *cobra.Command, arguments []string) error {
	if len(arguments) == 0 {
		if command != nil {
			_ = command.Help()
		}
		return errors.New(removeMissingPathsErrorMessage)
	}

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

	remoteName := configuration.Remote
	if command != nil && command.Flags().Changed(removeRemoteFlagName) {
		flagValue, flagError := command.Flags().GetString(removeRemoteFlagName)
		if flagError != nil {
			return flagError
		}
		remoteName = strings.TrimSpace(flagValue)
	}

	pushEnabled := configuration.Push
	if command != nil {
		flagValue, flagChanged, flagError := flagutils.BoolFlag(command, removePushFlagName)
		if flagError != nil && !errors.Is(flagError, flagutils.ErrFlagNotDefined) {
			return flagError
		}
		if flagChanged {
			pushEnabled = flagValue
		}
	}

	restoreEnabled := configuration.Restore
	if command != nil {
		flagValue, flagChanged, flagError := flagutils.BoolFlag(command, removeRestoreFlagName)
		if flagError != nil && !errors.Is(flagError, flagutils.ErrFlagNotDefined) {
			return flagError
		}
		if flagChanged {
			restoreEnabled = flagValue
		}
	}

	pushMissing := configuration.PushMissing
	if command != nil {
		flagValue, flagChanged, flagError := flagutils.BoolFlag(command, removePushMissingFlagName)
		if flagError != nil && !errors.Is(flagError, flagutils.ErrFlagNotDefined) {
			return flagError
		}
		if flagChanged {
			pushMissing = flagValue
		}
	}

	roots, rootsError := requireRepositoryRoots(command, nil, configuration.RepositoryRoots)
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

	resolvedManager := gitManager
	repositoryManager := (*gitrepo.RepositoryManager)(nil)
	if concreteManager, ok := resolvedManager.(*gitrepo.RepositoryManager); ok {
		repositoryManager = concreteManager
	} else {
		constructedManager, constructedManagerError := gitrepo.NewRepositoryManager(gitExecutor)
		if constructedManagerError != nil {
			return constructedManagerError
		}
		repositoryManager = constructedManager
	}

	fileSystem := dependencies.ResolveFileSystem(builder.FileSystem)
	repositoryDiscoverer := dependencies.ResolveRepositoryDiscoverer(builder.Discoverer)

	githubClient, githubClientError := githubcli.NewClient(gitExecutor)
	if githubClientError != nil {
		return githubClientError
	}

	taskDependencies := workflow.Dependencies{
		Logger:               logger,
		RepositoryDiscoverer: repositoryDiscoverer,
		GitExecutor:          gitExecutor,
		RepositoryManager:    repositoryManager,
		GitHubClient:         githubClient,
		FileSystem:           fileSystem,
		Output:               command.OutOrStdout(),
		Errors:               command.ErrOrStderr(),
	}

	taskRunner := ResolveTaskRunner(builder.TaskRunnerFactory, taskDependencies)

	normalizedPaths := make([]string, 0, len(arguments))
	for _, pathArgument := range arguments {
		trimmed := strings.TrimSpace(pathArgument)
		if len(trimmed) == 0 {
			continue
		}
		normalized := strings.TrimPrefix(trimmed, "./")
		if len(normalized) == 0 {
			continue
		}
		normalizedPaths = append(normalizedPaths, normalized)
	}
	if len(normalizedPaths) == 0 {
		return errors.New(removeMissingPathsErrorMessage)
	}

	actionOptions := map[string]any{
		"paths":        normalizedPaths,
		"remote":       remoteName,
		"push":         pushEnabled,
		"restore":      restoreEnabled,
		"push_missing": pushMissing,
	}

	taskDefinition := workflow.TaskDefinition{
		Name:        "Remove repository history paths",
		EnsureClean: true,
		Actions: []workflow.TaskActionDefinition{
			{Type: "repo.history.purge", Options: actionOptions},
		},
	}

	runtimeOptions := workflow.RuntimeOptions{
		DryRun:    dryRun,
		AssumeYes: assumeYes,
	}

	return taskRunner.Run(command.Context(), roots, []workflow.TaskDefinition{taskDefinition}, runtimeOptions)
}

func (builder *RemoveCommandBuilder) resolveConfiguration() RemoveConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultToolsConfiguration().Remove
	}

	return builder.ConfigurationProvider().sanitize()
}
