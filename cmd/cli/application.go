package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/cmd/cli/repos"
	workflowcmd "github.com/temirov/git_scripts/cmd/cli/workflow"
	"github.com/temirov/git_scripts/internal/audit"
	"github.com/temirov/git_scripts/internal/branches"
	"github.com/temirov/git_scripts/internal/migrate"
	"github.com/temirov/git_scripts/internal/packages"
	"github.com/temirov/git_scripts/internal/ui"
	"github.com/temirov/git_scripts/internal/utils"
)

const (
	applicationNameConstant                 = "git-scripts"
	applicationShortDescriptionConstant     = "Command-line interface for git_scripts utilities"
	applicationLongDescriptionConstant      = "git_scripts ships reusable helpers that integrate Git, GitHub CLI, and related tooling."
	configFileFlagNameConstant              = "config"
	configFileFlagUsageConstant             = "Optional path to a configuration file (YAML or JSON)."
	logLevelFlagNameConstant                = "log-level"
	logLevelFlagUsageConstant               = "Override the configured log level."
	logFormatFlagNameConstant               = "log-format"
	logFormatFlagUsageConstant              = "Override the configured log format (structured or console)."
	logLevelConfigKeyConstant               = "log_level"
	logFormatConfigKeyConstant              = "log_format"
	environmentPrefixConstant               = "GITSCRIPTS"
	configurationNameConstant               = "config"
	configurationTypeConstant               = "yaml"
	configurationInitializedMessageConstant = "configuration initialized"
	configurationLogLevelFieldConstant      = "log_level"
	configurationLogFormatFieldConstant     = "log_format"
	configurationFileFieldConstant          = "config_file"
	configurationLoadErrorTemplateConstant  = "unable to load configuration: %w"
	loggerCreationErrorTemplateConstant     = "unable to create logger: %w"
	loggerSyncErrorTemplateConstant         = "unable to flush logger: %w"
	rootCommandInfoMessageConstant          = "git_scripts CLI executed"
	rootCommandDebugMessageConstant         = "git_scripts CLI diagnostics"
	logFieldCommandNameConstant             = "command_name"
	logFieldArgumentCountConstant           = "argument_count"
	logFieldArgumentsConstant               = "arguments"
	loggerNotInitializedMessageConstant     = "logger not initialized"
	defaultConfigurationSearchPathConstant  = "."
)

// ApplicationConfiguration describes the persisted configuration for the CLI entrypoint.
type ApplicationConfiguration struct {
	LogLevel  string                 `mapstructure:"log_level"`
	LogFormat string                 `mapstructure:"log_format"`
	Packages  packages.Configuration `mapstructure:"packages"`
}

// Application wires the Cobra root command, configuration loader, and structured logger.
type Application struct {
	rootCommand           *cobra.Command
	configurationLoader   *utils.ConfigurationLoader
	loggerFactory         *utils.LoggerFactory
	logger                *zap.Logger
	eventLogger           *zap.Logger
	commandEventsLogger   *ui.ConsoleCommandEventLogger
	configuration         ApplicationConfiguration
	configurationMetadata utils.LoadedConfiguration
	configurationFilePath string
	logLevelFlagValue     string
	logFormatFlagValue    string
}

// NewApplication assembles a fully wired CLI application instance.
func NewApplication() *Application {
	configurationLoader := utils.NewConfigurationLoader(
		configurationNameConstant,
		configurationTypeConstant,
		environmentPrefixConstant,
		[]string{defaultConfigurationSearchPathConstant},
	)

	application := &Application{
		configurationLoader: configurationLoader,
		loggerFactory:       utils.NewLoggerFactory(),
	}

	cobraCommand := &cobra.Command{
		Use:           applicationNameConstant,
		Short:         applicationShortDescriptionConstant,
		Long:          applicationLongDescriptionConstant,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(command *cobra.Command, arguments []string) error {
			return application.initializeConfiguration(command)
		},
		RunE: func(command *cobra.Command, arguments []string) error {
			return application.runRootCommand(command, arguments)
		},
	}

	cobraCommand.SetContext(context.Background())
	cobraCommand.PersistentFlags().StringVar(&application.configurationFilePath, configFileFlagNameConstant, "", configFileFlagUsageConstant)
	cobraCommand.PersistentFlags().StringVar(&application.logLevelFlagValue, logLevelFlagNameConstant, "", logLevelFlagUsageConstant)
	cobraCommand.PersistentFlags().StringVar(&application.logFormatFlagValue, logFormatFlagNameConstant, "", logFormatFlagUsageConstant)

	auditBuilder := audit.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		CommandEventsObserver: application.commandEventsLogger,
	}
	auditCommand, auditBuildError := auditBuilder.Build()
	if auditBuildError == nil {
		cobraCommand.AddCommand(auditCommand)
	}

	branchCleanupBuilder := branches.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		CommandEventsObserver: application.commandEventsLogger,
	}
	branchCleanupCommand, branchCleanupBuildError := branchCleanupBuilder.Build()
	if branchCleanupBuildError == nil {
		cobraCommand.AddCommand(branchCleanupCommand)
	}

	branchMigrationBuilder := migrate.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		CommandEventsObserver: application.commandEventsLogger,
	}
	if workingDirectory, workingDirectoryError := os.Getwd(); workingDirectoryError == nil {
		branchMigrationBuilder.WorkingDirectory = workingDirectory
	}
	branchCommand, branchBuildError := branchMigrationBuilder.Build()
	if branchBuildError == nil {
		cobraCommand.AddCommand(branchCommand)
	}

	packagesBuilder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		ConfigurationProvider: func() packages.Configuration {
			return application.configuration.Packages
		},
	}
	packagesCommand, packagesBuildError := packagesBuilder.Build()
	if packagesBuildError == nil {
		cobraCommand.AddCommand(packagesCommand)
	}

	reposBuilder := repos.CommandGroupBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		CommandEventsObserver: application.commandEventsLogger,
	}
	reposCommand, reposBuildError := reposBuilder.Build()
	if reposBuildError == nil {
		cobraCommand.AddCommand(reposCommand)
	}

	workflowBuilder := workflowcmd.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		CommandEventsObserver: application.commandEventsLogger,
	}
	workflowCommand, workflowBuildError := workflowBuilder.Build()
	if workflowBuildError == nil {
		cobraCommand.AddCommand(workflowCommand)
	}

	application.rootCommand = cobraCommand

	return application
}

// Execute runs the configured Cobra command hierarchy and ensures logger flushing.
func (application *Application) Execute() error {
	executionError := application.rootCommand.Execute()
	if syncError := application.flushLogger(); syncError != nil {
		return fmt.Errorf(loggerSyncErrorTemplateConstant, syncError)
	}
	return executionError
}

// Execute builds a fresh application instance and executes the root command hierarchy.
func Execute() error {
	return NewApplication().Execute()
}

func (application *Application) initializeConfiguration(command *cobra.Command) error {
	defaultValues := map[string]any{
		logLevelConfigKeyConstant:  string(utils.LogLevelInfo),
		logFormatConfigKeyConstant: string(utils.LogFormatStructured),
	}
	for configurationKey, configurationValue := range packages.DefaultConfigurationValues() {
		defaultValues[configurationKey] = configurationValue
	}

	loadedConfiguration, loadError := application.configurationLoader.LoadConfiguration(application.configurationFilePath, defaultValues, &application.configuration)
	if loadError != nil {
		return fmt.Errorf(configurationLoadErrorTemplateConstant, loadError)
	}

	application.configurationMetadata = loadedConfiguration

	if application.persistentFlagChanged(command, logLevelFlagNameConstant) {
		application.configuration.LogLevel = application.logLevelFlagValue
	}

	if application.persistentFlagChanged(command, logFormatFlagNameConstant) {
		application.configuration.LogFormat = application.logFormatFlagValue
	}

	loggerOutputs, loggerCreationError := application.loggerFactory.CreateLoggerOutputs(
		utils.LogLevel(application.configuration.LogLevel),
		utils.LogFormat(application.configuration.LogFormat),
	)
	if loggerCreationError != nil {
		return fmt.Errorf(loggerCreationErrorTemplateConstant, loggerCreationError)
	}

	application.logger = loggerOutputs.DiagnosticLogger
	application.eventLogger = loggerOutputs.ConsoleLogger
	application.commandEventsLogger = ui.NewConsoleCommandEventLogger(application.eventLogger)

	application.logger.Info(
		configurationInitializedMessageConstant,
		zap.String(configurationLogLevelFieldConstant, application.configuration.LogLevel),
		zap.String(configurationLogFormatFieldConstant, application.configuration.LogFormat),
		zap.String(configurationFileFieldConstant, application.configurationMetadata.ConfigFileUsed),
	)

	return nil
}

func (application *Application) runRootCommand(command *cobra.Command, arguments []string) error {
	if application.logger == nil {
		return errors.New(loggerNotInitializedMessageConstant)
	}

	application.logger.Info(
		rootCommandInfoMessageConstant,
		zap.String(logFieldCommandNameConstant, command.Name()),
		zap.Int(logFieldArgumentCountConstant, len(arguments)),
	)

	application.logger.Debug(
		rootCommandDebugMessageConstant,
		zap.Strings(logFieldArgumentsConstant, arguments),
	)

	if len(arguments) == 0 {
		return command.Help()
	}

	return nil
}

func (application *Application) flushLogger() error {
	if syncError := application.syncLoggerInstance(application.logger); syncError != nil {
		return syncError
	}
	if syncError := application.syncLoggerInstance(application.eventLogger); syncError != nil {
		return syncError
	}
	return nil
}

func (application *Application) syncLoggerInstance(logger *zap.Logger) error {
	if logger == nil {
		return nil
	}

	syncError := logger.Sync()
	switch {
	case syncError == nil:
		return nil
	case errors.Is(syncError, syscall.ENOTSUP):
		return nil
	case errors.Is(syncError, syscall.EINVAL):
		return nil
	default:
		return syncError
	}
}

func (application *Application) persistentFlagChanged(command *cobra.Command, flagName string) bool {
	if command == nil {
		return false
	}

	flagSetsToInspect := []*pflag.FlagSet{
		command.PersistentFlags(),
		command.InheritedFlags(),
	}

	rootCommand := command.Root()
	if rootCommand != nil {
		flagSetsToInspect = append(flagSetsToInspect, rootCommand.PersistentFlags())
	}

	for _, flagSet := range flagSetsToInspect {
		if flagSet == nil {
			continue
		}

		if flagSet.Changed(flagName) {
			return true
		}
	}

	return false
}
