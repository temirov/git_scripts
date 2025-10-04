package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/githubcli"
	"github.com/temirov/gix/internal/gitrepo"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/shared"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
	"github.com/temirov/gix/internal/workflow"
)

const (
	commandUseConstant                        = "workflow [workflow]"
	commandShortDescriptionConstant           = "Run a workflow configuration file"
	commandLongDescriptionConstant            = "workflow executes operations defined in a YAML or JSON configuration file across discovered repositories."
	requireCleanFlagNameConstant              = "require-clean"
	requireCleanFlagDescriptionConstant       = "Require clean worktrees for rename operations"
	configurationPathRequiredMessageConstant  = "workflow configuration path required; provide a positional argument or --config flag"
	loadConfigurationErrorTemplateConstant    = "unable to load workflow configuration: %w"
	buildOperationsErrorTemplateConstant      = "unable to build workflow operations: %w"
	gitRepositoryManagerErrorTemplateConstant = "unable to construct repository manager: %w"
	gitHubClientErrorTemplateConstant         = "unable to construct GitHub client: %w"
)

// CommandBuilder assembles the workflow command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	Discoverer                   shared.RepositoryDiscoverer
	GitExecutor                  shared.GitExecutor
	FileSystem                   shared.FileSystem
	PrompterFactory              PrompterFactory
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the workflow command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandUseConstant,
		Short: commandShortDescriptionConstant,
		Long:  commandLongDescriptionConstant,
		RunE:  builder.run,
	}

	flagutils.AddToggleFlag(command.Flags(), nil, requireCleanFlagNameConstant, "", false, requireCleanFlagDescriptionConstant)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	executionFlags, executionFlagsAvailable := flagutils.ResolveExecutionFlags(command)
	contextAccessor := utils.NewCommandContextAccessor()

	configurationPathCandidate := ""
	remainingArguments := []string{}
	if len(arguments) > 0 {
		configurationPathCandidate = strings.TrimSpace(arguments[0])
		if len(arguments) > 1 {
			remainingArguments = append(remainingArguments, arguments[1:]...)
		}
	} else {
		configurationPathFromContext, configurationPathAvailable := contextAccessor.ConfigurationFilePath(command.Context())
		if configurationPathAvailable {
			configurationPathCandidate = strings.TrimSpace(configurationPathFromContext)
		}
	}

	if len(configurationPathCandidate) == 0 {
		if helpError := displayCommandHelp(command); helpError != nil {
			return helpError
		}
		return errors.New(configurationPathRequiredMessageConstant)
	}

	configurationPath := configurationPathCandidate
	workflowConfiguration, configurationError := workflow.LoadConfiguration(configurationPath)
	if configurationError != nil {
		return fmt.Errorf(loadConfigurationErrorTemplateConstant, configurationError)
	}

	operations, operationsError := workflow.BuildOperations(workflowConfiguration)
	if operationsError != nil {
		return fmt.Errorf(buildOperationsErrorTemplateConstant, operationsError)
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

	repositoryManager, managerError := gitrepo.NewRepositoryManager(gitExecutor)
	if managerError != nil {
		return fmt.Errorf(gitRepositoryManagerErrorTemplateConstant, managerError)
	}

	gitHubClient, clientError := githubcli.NewClient(gitExecutor)
	if clientError != nil {
		return fmt.Errorf(gitHubClientErrorTemplateConstant, clientError)
	}

	repositoryDiscoverer := dependencies.ResolveRepositoryDiscoverer(builder.Discoverer)
	fileSystem := dependencies.ResolveFileSystem(builder.FileSystem)
	prompter := resolvePrompter(builder.PrompterFactory, command)

	workflowDependencies := workflow.Dependencies{
		Logger:               logger,
		RepositoryDiscoverer: repositoryDiscoverer,
		GitExecutor:          gitExecutor,
		RepositoryManager:    repositoryManager,
		GitHubClient:         gitHubClient,
		FileSystem:           fileSystem,
		Prompter:             prompter,
		Output:               utils.NewFlushingWriter(command.OutOrStdout()),
		Errors:               utils.NewFlushingWriter(command.ErrOrStderr()),
	}

	executor := workflow.NewExecutor(operations, workflowDependencies)

	commandConfiguration := builder.resolveConfiguration()

	requireCleanDefault := commandConfiguration.RequireClean
	if command != nil {
		requireCleanFlagValue, requireCleanFlagChanged, requireCleanFlagError := flagutils.BoolFlag(command, requireCleanFlagNameConstant)
		if requireCleanFlagError != nil && !errors.Is(requireCleanFlagError, flagutils.ErrFlagNotDefined) {
			return requireCleanFlagError
		}
		if requireCleanFlagChanged {
			requireCleanDefault = requireCleanFlagValue
		}
	}

	workflow.ApplyDefaults(operations, workflow.OperationDefaults{RequireClean: requireCleanDefault})

	roots, rootsError := rootutils.Resolve(command, remainingArguments, commandConfiguration.Roots)
	if rootsError != nil {
		return rootsError
	}

	dryRun := commandConfiguration.DryRun
	if executionFlagsAvailable && executionFlags.DryRunSet {
		dryRun = executionFlags.DryRun
	}

	assumeYes := commandConfiguration.AssumeYes
	if executionFlagsAvailable && executionFlags.AssumeYesSet {
		assumeYes = executionFlags.AssumeYes
	}

	runtimeOptions := workflow.RuntimeOptions{DryRun: dryRun, AssumeYes: assumeYes}

	return executor.Execute(command.Context(), roots, runtimeOptions)
}

func (builder *CommandBuilder) resolveConfiguration() CommandConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultCommandConfiguration()
	}

	provided := builder.ConfigurationProvider()
	return provided.Sanitize()
}
