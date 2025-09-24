package workflow

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
	"github.com/temirov/git_scripts/internal/repos/dependencies"
	"github.com/temirov/git_scripts/internal/repos/shared"
	"github.com/temirov/git_scripts/internal/workflow"
)

const (
	groupUseConstant                          = "workflow"
	groupShortDescriptionConstant             = "Execute declarative repository workflows"
	groupLongDescriptionConstant              = "workflow runs ordered operations described in a configuration file."
	runUseConstant                            = "run [workflow]"
	runShortDescriptionConstant               = "Run a workflow configuration file"
	runLongDescriptionConstant                = "workflow run executes operations defined in a YAML or JSON configuration file across discovered repositories."
	rootsFlagNameConstant                     = "roots"
	rootsFlagDescriptionConstant              = "Root directories containing repositories"
	dryRunFlagNameConstant                    = "dry-run"
	dryRunFlagDescriptionConstant             = "Preview workflow operations without making changes"
	assumeYesFlagNameConstant                 = "yes"
	assumeYesFlagShorthandConstant            = "y"
	assumeYesFlagDescriptionConstant          = "Automatically confirm prompts"
	configurationPathRequiredMessageConstant  = "workflow configuration path required"
	loadConfigurationErrorTemplateConstant    = "unable to load workflow configuration: %w"
	buildOperationsErrorTemplateConstant      = "unable to build workflow operations: %w"
	gitRepositoryManagerErrorTemplateConstant = "unable to construct repository manager: %w"
	gitHubClientErrorTemplateConstant         = "unable to construct GitHub client: %w"
)

// CommandBuilder assembles the workflow command hierarchy.
type CommandBuilder struct {
	LoggerProvider  LoggerProvider
	Discoverer      shared.RepositoryDiscoverer
	GitExecutor     shared.GitExecutor
	FileSystem      shared.FileSystem
	PrompterFactory PrompterFactory
}

// Build constructs the workflow command group.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	groupCommand := &cobra.Command{
		Use:   groupUseConstant,
		Short: groupShortDescriptionConstant,
		Long:  groupLongDescriptionConstant,
	}

	runCommand := &cobra.Command{
		Use:   runUseConstant,
		Short: runShortDescriptionConstant,
		Long:  runLongDescriptionConstant,
		RunE:  builder.run,
	}

	runCommand.Flags().StringSlice(rootsFlagNameConstant, nil, rootsFlagDescriptionConstant)
	runCommand.Flags().Bool(dryRunFlagNameConstant, false, dryRunFlagDescriptionConstant)
	runCommand.Flags().BoolP(assumeYesFlagNameConstant, assumeYesFlagShorthandConstant, false, assumeYesFlagDescriptionConstant)

	groupCommand.AddCommand(runCommand)

	return groupCommand, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	if len(arguments) == 0 {
		if helpError := displayCommandHelp(command); helpError != nil {
			return helpError
		}
		return errors.New(configurationPathRequiredMessageConstant)
	}

	configurationPath := arguments[0]
	configuration, configurationError := workflow.LoadConfiguration(configurationPath)
	if configurationError != nil {
		return fmt.Errorf(loadConfigurationErrorTemplateConstant, configurationError)
	}

	operations, operationsError := workflow.BuildOperations(configuration)
	if operationsError != nil {
		return fmt.Errorf(buildOperationsErrorTemplateConstant, operationsError)
	}

	logger := resolveLogger(builder.LoggerProvider)
	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger)
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
		Output:               command.OutOrStdout(),
		Errors:               command.ErrOrStderr(),
	}

	executor := workflow.NewExecutor(operations, workflowDependencies)

	rootValues, _ := command.Flags().GetStringSlice(rootsFlagNameConstant)
	dryRun, _ := command.Flags().GetBool(dryRunFlagNameConstant)
	assumeYes, _ := command.Flags().GetBool(assumeYesFlagNameConstant)

	runtimeOptions := workflow.RuntimeOptions{DryRun: dryRun, AssumeYes: assumeYes}
	roots := determineRoots(rootValues)

	return executor.Execute(command.Context(), roots, runtimeOptions)
}
