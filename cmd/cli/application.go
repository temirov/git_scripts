package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/cmd/cli/repos"
	workflowcmd "github.com/temirov/git_scripts/cmd/cli/workflow"
	"github.com/temirov/git_scripts/internal/audit"
	"github.com/temirov/git_scripts/internal/branches"
	"github.com/temirov/git_scripts/internal/migrate"
	"github.com/temirov/git_scripts/internal/packages"
	"github.com/temirov/git_scripts/internal/utils"
)

const (
	applicationNameConstant                            = "git-scripts"
	applicationShortDescriptionConstant                = "Command-line interface for git_scripts utilities"
	applicationLongDescriptionConstant                 = "git_scripts ships reusable helpers that integrate Git, GitHub CLI, and related tooling."
	configFileFlagNameConstant                         = "config"
	configFileFlagUsageConstant                        = "Optional path to a configuration file (YAML or JSON)."
	logLevelFlagNameConstant                           = "log-level"
	logLevelFlagUsageConstant                          = "Override the configured log level."
	logFormatFlagNameConstant                          = "log-format"
	logFormatFlagUsageConstant                         = "Override the configured log format (structured or console)."
	commonConfigurationKeyConstant                     = "common"
	commonLogLevelConfigKeyConstant                    = commonConfigurationKeyConstant + ".log_level"
	commonLogFormatConfigKeyConstant                   = commonConfigurationKeyConstant + ".log_format"
	environmentPrefixConstant                          = "GITSCRIPTS"
	configurationNameConstant                          = "config"
	configurationTypeConstant                          = "yaml"
	configurationInitializedMessageConstant            = "configuration initialized"
	configurationLogLevelFieldConstant                 = "log_level"
	configurationLogFormatFieldConstant                = "log_format"
	configurationFileFieldConstant                     = "config_file"
	configurationLoadErrorTemplateConstant             = "unable to load configuration: %w"
	loggerCreationErrorTemplateConstant                = "unable to create logger: %w"
	loggerSyncErrorTemplateConstant                    = "unable to flush logger: %w"
	rootCommandInfoMessageConstant                     = "git_scripts CLI executed"
	rootCommandDebugMessageConstant                    = "git_scripts CLI diagnostics"
	logFieldCommandNameConstant                        = "command_name"
	logFieldArgumentCountConstant                      = "argument_count"
	logFieldArgumentsConstant                          = "arguments"
	loggerNotInitializedMessageConstant                = "logger not initialized"
	defaultConfigurationSearchPathConstant             = "."
	configurationSearchPathEnvironmentVariableConstant = "GITSCRIPTS_CONFIG_SEARCH_PATH"
	auditOperationNameConstant                         = "audit"
	packagesPurgeOperationNameConstant                 = "repo-packages-purge"
	branchCleanupOperationNameConstant                 = "repo-prs-purge"
	reposRenameOperationNameConstant                   = "repo-folders-rename"
	reposRemotesOperationNameConstant                  = "repo-remote-update"
	reposProtocolOperationNameConstant                 = "repo-protocol-convert"
	workflowCommandOperationNameConstant               = "workflow"
	branchMigrateOperationNameConstant                 = "branch-migrate"
	operationDecodeErrorMessageConstant                = "unable to decode operation defaults"
	operationNameLogFieldConstant                      = "operation"
	operationErrorLogFieldConstant                     = "error"
)

// ApplicationConfiguration describes the persisted configuration for the CLI entrypoint.
type ApplicationConfiguration struct {
	Common     ApplicationCommonConfiguration      `mapstructure:"common"`
	Operations []ApplicationOperationConfiguration `mapstructure:"operations"`
}

// ApplicationCommonConfiguration stores logging configuration shared across commands.
type ApplicationCommonConfiguration struct {
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
}

// ApplicationOperationConfiguration captures reusable operation defaults from the configuration file.
type ApplicationOperationConfiguration struct {
	Name    string         `mapstructure:"operation"`
	Options map[string]any `mapstructure:"with"`
}

// OperationConfigurations stores reusable operation defaults indexed by normalized operation name.
type OperationConfigurations struct {
	entries map[string]map[string]any
}

func newOperationConfigurations(definitions []ApplicationOperationConfiguration) OperationConfigurations {
	entries := make(map[string]map[string]any)
	for index := range definitions {
		normalizedName := normalizeOperationName(definitions[index].Name)
		if len(normalizedName) == 0 {
			continue
		}

		options := make(map[string]any)
		for key, value := range definitions[index].Options {
			options[key] = value
		}

		entries[normalizedName] = options
	}

	return OperationConfigurations{entries: entries}
}

func (configurations OperationConfigurations) decode(operationName string, target any) error {
	if target == nil {
		return nil
	}

	normalizedName := normalizeOperationName(operationName)
	if len(normalizedName) == 0 {
		return nil
	}

	if configurations.entries == nil {
		return nil
	}

	options, exists := configurations.entries[normalizedName]
	if !exists || len(options) == 0 {
		return nil
	}

	decoder, decoderError := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		Result:           target,
		WeaklyTypedInput: true,
	})
	if decoderError != nil {
		return decoderError
	}

	return decoder.Decode(options)
}

func normalizeOperationName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// Application wires the Cobra root command, configuration loader, and structured logger.
type Application struct {
	rootCommand             *cobra.Command
	configurationLoader     *utils.ConfigurationLoader
	loggerFactory           *utils.LoggerFactory
	logger                  *zap.Logger
	configuration           ApplicationConfiguration
	configurationMetadata   utils.LoadedConfiguration
	configurationFilePath   string
	logLevelFlagValue       string
	logFormatFlagValue      string
	commandContextAccessor  utils.CommandContextAccessor
	operationConfigurations OperationConfigurations
}

// NewApplication assembles a fully wired CLI application instance.
func NewApplication() *Application {
	application := &Application{
		loggerFactory:          utils.NewLoggerFactory(),
		logger:                 zap.NewNop(),
		commandContextAccessor: utils.NewCommandContextAccessor(),
	}

	application.configurationLoader = utils.NewConfigurationLoader(
		configurationNameConstant,
		configurationTypeConstant,
		environmentPrefixConstant,
		application.resolveConfigurationSearchPaths(),
	)

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
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.auditCommandConfiguration,
	}
	auditCommand, auditBuildError := auditBuilder.Build()
	if auditBuildError == nil {
		cobraCommand.AddCommand(auditCommand)
	}

	branchCleanupBuilder := branches.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.branchCleanupConfiguration,
	}
	branchCleanupCommand, branchCleanupBuildError := branchCleanupBuilder.Build()
	if branchCleanupBuildError == nil {
		cobraCommand.AddCommand(branchCleanupCommand)
	}

	branchMigrationBuilder := migrate.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.branchMigrateConfiguration,
	}
	if workingDirectory, workingDirectoryError := os.Getwd(); workingDirectoryError == nil {
		branchMigrationBuilder.WorkingDirectory = workingDirectory
	}
	branchMigrationCommand, branchMigrationBuildError := branchMigrationBuilder.Build()
	if branchMigrationBuildError == nil {
		cobraCommand.AddCommand(branchMigrationCommand)
	}

	packagesBuilder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		ConfigurationProvider: application.packagesConfiguration,
	}
	repoPackagesPurgeCommand, packagesBuildError := packagesBuilder.Build()
	if packagesBuildError == nil {
		cobraCommand.AddCommand(repoPackagesPurgeCommand)
	}

	renameBuilder := repos.RenameCommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.reposRenameConfiguration,
	}
	renameCommand, renameBuildError := renameBuilder.Build()
	if renameBuildError == nil {
		cobraCommand.AddCommand(renameCommand)
	}

	remotesBuilder := repos.RemotesCommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.reposRemotesConfiguration,
	}
	remotesCommand, remotesBuildError := remotesBuilder.Build()
	if remotesBuildError == nil {
		cobraCommand.AddCommand(remotesCommand)
	}

	protocolBuilder := repos.ProtocolCommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.reposProtocolConfiguration,
	}
	protocolCommand, protocolBuildError := protocolBuilder.Build()
	if protocolBuildError == nil {
		cobraCommand.AddCommand(protocolCommand)
	}

	workflowBuilder := workflowcmd.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.workflowCommandConfiguration,
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

func (application *Application) resolveConfigurationSearchPaths() []string {
	overrideValue := strings.TrimSpace(os.Getenv(configurationSearchPathEnvironmentVariableConstant))
	if len(overrideValue) == 0 {
		return []string{defaultConfigurationSearchPathConstant}
	}

	overridePaths := strings.FieldsFunc(overrideValue, func(candidate rune) bool {
		return candidate == os.PathListSeparator
	})

	cleanedPaths := make([]string, 0, len(overridePaths))
	for _, pathCandidate := range overridePaths {
		trimmedCandidate := strings.TrimSpace(pathCandidate)
		if len(trimmedCandidate) == 0 {
			continue
		}
		cleanedPaths = append(cleanedPaths, trimmedCandidate)
	}

	if len(cleanedPaths) == 0 {
		return []string{defaultConfigurationSearchPathConstant}
	}

	return cleanedPaths
}

func (application *Application) initializeConfiguration(command *cobra.Command) error {
	defaultValues := map[string]any{
		commonLogLevelConfigKeyConstant:  string(utils.LogLevelInfo),
		commonLogFormatConfigKeyConstant: string(utils.LogFormatStructured),
	}

	loadedConfiguration, loadError := application.configurationLoader.LoadConfiguration(application.configurationFilePath, defaultValues, &application.configuration)
	if loadError != nil {
		return fmt.Errorf(configurationLoadErrorTemplateConstant, loadError)
	}

	application.configurationMetadata = loadedConfiguration
	application.operationConfigurations = newOperationConfigurations(application.configuration.Operations)

	if application.persistentFlagChanged(command, logLevelFlagNameConstant) {
		application.configuration.Common.LogLevel = application.logLevelFlagValue
	}

	if application.persistentFlagChanged(command, logFormatFlagNameConstant) {
		application.configuration.Common.LogFormat = application.logFormatFlagValue
	}

	loggerOutputs, loggerCreationError := application.loggerFactory.CreateLoggerOutputs(
		utils.LogLevel(application.configuration.Common.LogLevel),
		utils.LogFormat(application.configuration.Common.LogFormat),
	)
	if loggerCreationError != nil {
		return fmt.Errorf(loggerCreationErrorTemplateConstant, loggerCreationError)
	}

	application.logger = loggerOutputs.DiagnosticLogger

	application.logger.Info(
		configurationInitializedMessageConstant,
		zap.String(configurationLogLevelFieldConstant, application.configuration.Common.LogLevel),
		zap.String(configurationLogFormatFieldConstant, application.configuration.Common.LogFormat),
		zap.String(configurationFileFieldConstant, application.configurationMetadata.ConfigFileUsed),
	)

	if command != nil {
		updatedContext := application.commandContextAccessor.WithConfigurationFilePath(
			command.Context(),
			application.configurationMetadata.ConfigFileUsed,
		)
		command.SetContext(updatedContext)
		if rootCommand := command.Root(); rootCommand != nil {
			rootCommand.SetContext(updatedContext)
		}
	}

	return nil
}

func (application *Application) humanReadableLoggingEnabled() bool {
	logFormatValue := strings.TrimSpace(application.configuration.Common.LogFormat)
	return strings.EqualFold(logFormatValue, string(utils.LogFormatConsole))
}

func (application *Application) auditCommandConfiguration() audit.CommandConfiguration {
	configuration := audit.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(auditOperationNameConstant, &configuration)
	return configuration
}

func (application *Application) packagesConfiguration() packages.Configuration {
	configuration := packages.DefaultConfiguration()
	application.decodeOperationConfiguration(packagesPurgeOperationNameConstant, &configuration.Purge)
	return configuration
}

func (application *Application) branchCleanupConfiguration() branches.CommandConfiguration {
	configuration := branches.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(branchCleanupOperationNameConstant, &configuration)
	return configuration
}

func (application *Application) reposRenameConfiguration() repos.RenameConfiguration {
	configuration := repos.DefaultToolsConfiguration()
	application.decodeOperationConfiguration(reposRenameOperationNameConstant, &configuration.Rename)
	return configuration.Rename
}

func (application *Application) reposRemotesConfiguration() repos.RemotesConfiguration {
	configuration := repos.DefaultToolsConfiguration()
	application.decodeOperationConfiguration(reposRemotesOperationNameConstant, &configuration.Remotes)
	return configuration.Remotes
}

func (application *Application) reposProtocolConfiguration() repos.ProtocolConfiguration {
	configuration := repos.DefaultToolsConfiguration()
	application.decodeOperationConfiguration(reposProtocolOperationNameConstant, &configuration.Protocol)
	return configuration.Protocol
}

func (application *Application) workflowCommandConfiguration() workflowcmd.CommandConfiguration {
	configuration := workflowcmd.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(workflowCommandOperationNameConstant, &configuration)
	return configuration
}

func (application *Application) branchMigrateConfiguration() migrate.CommandConfiguration {
	configuration := migrate.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(branchMigrateOperationNameConstant, &configuration)
	return configuration
}

func (application *Application) decodeOperationConfiguration(operationName string, target any) {
	if decodeError := application.operationConfigurations.decode(operationName, target); decodeError != nil {
		if application.logger == nil {
			return
		}
		application.logger.Warn(
			operationDecodeErrorMessageConstant,
			zap.String(operationNameLogFieldConstant, operationName),
			zap.Error(decodeError),
		)
	}
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
	case errors.Is(syncError, syscall.EBADF):
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
