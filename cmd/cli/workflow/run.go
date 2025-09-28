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
	"github.com/temirov/gix/internal/workflow"
)

const (
	commandUseConstant                        = "workflow [workflow]"
	commandShortDescriptionConstant           = "Run a workflow configuration file"
	commandLongDescriptionConstant            = "workflow executes operations defined in a YAML or JSON configuration file across discovered repositories."
	rootsFlagNameConstant                     = "roots"
	rootsFlagDescriptionConstant              = "Root directories containing repositories"
	dryRunFlagNameConstant                    = "dry-run"
	dryRunFlagDescriptionConstant             = "Preview workflow operations without making changes"
	assumeYesFlagNameConstant                 = "yes"
	assumeYesFlagShorthandConstant            = "y"
	assumeYesFlagDescriptionConstant          = "Automatically confirm prompts"
	configurationPathRequiredMessageConstant  = "workflow configuration path required; provide a positional argument or --config flag"
	loadConfigurationErrorTemplateConstant    = "unable to load workflow configuration: %w"
	buildOperationsErrorTemplateConstant      = "unable to build workflow operations: %w"
	gitRepositoryManagerErrorTemplateConstant = "unable to construct repository manager: %w"
	gitHubClientErrorTemplateConstant         = "unable to construct GitHub client: %w"
	missingRootsErrorMessageConstant          = "workflow roots required; specify --roots flag or configuration"
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

	command.Flags().StringSlice(rootsFlagNameConstant, nil, rootsFlagDescriptionConstant)
	command.Flags().Bool(dryRunFlagNameConstant, false, dryRunFlagDescriptionConstant)
	command.Flags().BoolP(assumeYesFlagNameConstant, assumeYesFlagShorthandConstant, false, assumeYesFlagDescriptionConstant)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	contextAccessor := utils.NewCommandContextAccessor()

	configurationPathCandidate := ""
	if len(arguments) > 0 {
		configurationPathCandidate = strings.TrimSpace(arguments[0])
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

	rootValues, _ := command.Flags().GetStringSlice(rootsFlagNameConstant)
	preferFlagRoots := command != nil && command.Flags().Changed(rootsFlagNameConstant)
	roots := DetermineRoots(rootValues, commandConfiguration.Roots, preferFlagRoots)
	if len(roots) == 0 {
		if helpError := displayCommandHelp(command); helpError != nil {
			return helpError
		}
		return errors.New(missingRootsErrorMessageConstant)
	}

	dryRun := commandConfiguration.DryRun
	if command != nil && command.Flags().Changed(dryRunFlagNameConstant) {
		dryRun, _ = command.Flags().GetBool(dryRunFlagNameConstant)
	}

	assumeYes := commandConfiguration.AssumeYes
	if command != nil && command.Flags().Changed(assumeYesFlagNameConstant) {
		assumeYes, _ = command.Flags().GetBool(assumeYesFlagNameConstant)
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
