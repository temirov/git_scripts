package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

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
	logLevelConfigKeyConstant               = "log_level"
	environmentPrefixConstant               = "GITSCRIPTS"
	configurationNameConstant               = "config"
	configurationTypeConstant               = "yaml"
	configurationInitializedMessageConstant = "configuration initialized"
	configurationLogLevelFieldConstant      = "log_level"
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
	errorOutputTemplateConstant             = "%v\n"
)

// ApplicationConfiguration describes the persisted configuration for the CLI entrypoint.
type ApplicationConfiguration struct {
	LogLevel string `mapstructure:"log_level"`
}

// CLIApplication wires the Cobra root command, configuration loader, and structured logger.
type CLIApplication struct {
	rootCommand           *cobra.Command
	configurationLoader   *utils.ConfigurationLoader
	loggerFactory         *utils.LoggerFactory
	logger                *zap.Logger
	configuration         ApplicationConfiguration
	configurationMetadata utils.LoadedConfiguration
	configurationFilePath string
	logLevelFlagValue     string
}

func main() {
	cliApplication := newCLIApplication()
	if executionError := cliApplication.Execute(); executionError != nil {
		fmt.Fprintf(os.Stderr, errorOutputTemplateConstant, executionError)
		os.Exit(1)
	}
}

func newCLIApplication() *CLIApplication {
	configurationLoader := utils.NewConfigurationLoader(
		configurationNameConstant,
		configurationTypeConstant,
		environmentPrefixConstant,
		[]string{defaultConfigurationSearchPathConstant},
	)

	cliApplication := &CLIApplication{
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
			return cliApplication.initializeConfiguration(command)
		},
		RunE: func(command *cobra.Command, arguments []string) error {
			return cliApplication.runRootCommand(command, arguments)
		},
	}

	cobraCommand.SetContext(context.Background())
	cobraCommand.PersistentFlags().StringVar(&cliApplication.configurationFilePath, configFileFlagNameConstant, "", configFileFlagUsageConstant)
	cobraCommand.PersistentFlags().StringVar(&cliApplication.logLevelFlagValue, logLevelFlagNameConstant, "", logLevelFlagUsageConstant)

	cliApplication.rootCommand = cobraCommand

	return cliApplication
}

// Execute runs the configured Cobra command hierarchy and ensures logger flushing.
func (application *CLIApplication) Execute() error {
	executionError := application.rootCommand.Execute()
	if syncError := application.flushLogger(); syncError != nil {
		return fmt.Errorf(loggerSyncErrorTemplateConstant, syncError)
	}
	return executionError
}

func (application *CLIApplication) initializeConfiguration(command *cobra.Command) error {
	defaultValues := map[string]any{
		logLevelConfigKeyConstant: string(utils.LogLevelInfo),
	}

	loadedConfiguration, loadError := application.configurationLoader.LoadConfiguration(application.configurationFilePath, defaultValues, &application.configuration)
	if loadError != nil {
		return fmt.Errorf(configurationLoadErrorTemplateConstant, loadError)
	}

	application.configurationMetadata = loadedConfiguration

	if command.PersistentFlags().Changed(logLevelFlagNameConstant) {
		application.configuration.LogLevel = application.logLevelFlagValue
	}

	logger, loggerCreationError := application.loggerFactory.CreateLogger(utils.LogLevel(application.configuration.LogLevel))
	if loggerCreationError != nil {
		return fmt.Errorf(loggerCreationErrorTemplateConstant, loggerCreationError)
	}

	application.logger = logger

	application.logger.Info(
		configurationInitializedMessageConstant,
		zap.String(configurationLogLevelFieldConstant, application.configuration.LogLevel),
		zap.String(configurationFileFieldConstant, application.configurationMetadata.ConfigFileUsed),
	)

	return nil
}

func (application *CLIApplication) runRootCommand(command *cobra.Command, arguments []string) error {
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

	return nil
}

func (application *CLIApplication) flushLogger() error {
	if application.logger == nil {
		return nil
	}

	syncError := application.logger.Sync()
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
