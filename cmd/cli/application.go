package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/temirov/gix/cmd/cli/repos"
	workflowcmd "github.com/temirov/gix/cmd/cli/workflow"
	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/branches"
	branchrefresh "github.com/temirov/gix/internal/branches/refresh"
	"github.com/temirov/gix/internal/migrate"
	"github.com/temirov/gix/internal/packages"
	reposdeps "github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/prompt"
	"github.com/temirov/gix/internal/repos/shared"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	"github.com/temirov/gix/internal/version"
)

const (
	applicationNameConstant                                          = "gix"
	applicationShortDescriptionConstant                              = "Command-line interface for gix utilities"
	applicationLongDescriptionConstant                               = "gix ships reusable helpers that integrate Git, GitHub CLI, and related tooling."
	configFileFlagNameConstant                                       = "config"
	configFileFlagUsageConstant                                      = "Optional path to a configuration file (YAML or JSON)."
	logLevelFlagNameConstant                                         = "log-level"
	logLevelFlagUsageConstant                                        = "Override the configured log level."
	logFormatFlagNameConstant                                        = "log-format"
	logFormatFlagUsageConstant                                       = "Override the configured log format (structured or console)."
	configurationInitializationFlagNameConstant                      = "init"
	configurationInitializationFlagUsageConstant                     = "Write the embedded default configuration to the selected scope (local or user)."
	configurationInitializationDefaultScopeConstant                  = "local"
	configurationInitializationForceFlagNameConstant                 = "force"
	configurationInitializationForceFlagUsageConstant                = "Overwrite an existing configuration file when initializing."
	configurationInitializationScopeLocalConstant                    = "local"
	configurationInitializationScopeUserConstant                     = "user"
	configurationInitializationUnsupportedScopeTemplateConstant      = "unsupported initialization scope %q"
	configurationInitializationWorkingDirectoryErrorTemplateConstant = "unable to determine working directory: %w"
	configurationInitializationWorkingDirectoryEmptyErrorConstant    = "working directory is empty"
	configurationInitializationHomeDirectoryErrorTemplateConstant    = "unable to determine user home directory: %w"
	configurationInitializationHomeDirectoryEmptyErrorConstant       = "user home directory is empty"
	configurationInitializationContentUnavailableErrorConstant       = "embedded configuration content is unavailable"
	configurationInitializationDirectoryErrorTemplateConstant        = "unable to ensure configuration directory %s: %w"
	configurationInitializationExistingFileTemplateConstant          = "configuration file already exists at %s (use --force to overwrite)"
	configurationInitializationExistingDirectoryTemplateConstant     = "configuration path %s is a directory"
	configurationInitializationDirectoryConflictTemplateConstant     = "configuration directory path %s is not a directory"
	configurationInitializationWriteErrorTemplateConstant            = "unable to write configuration file %s: %w"
	configurationInitializationSuccessMessageConstant                = "configuration file created"
	commonConfigurationKeyConstant                                   = "common"
	commonLogLevelConfigKeyConstant                                  = commonConfigurationKeyConstant + ".log_level"
	commonLogFormatConfigKeyConstant                                 = commonConfigurationKeyConstant + ".log_format"
	commonDryRunConfigKeyConstant                                    = commonConfigurationKeyConstant + ".dry_run"
	commonAssumeYesConfigKeyConstant                                 = commonConfigurationKeyConstant + ".assume_yes"
	commonRequireCleanConfigKeyConstant                              = commonConfigurationKeyConstant + ".require_clean"
	environmentPrefixConstant                                        = "GIX"
	configurationNameConstant                                        = "config"
	configurationTypeConstant                                        = "yaml"
	configurationFileNameConstant                                    = configurationNameConstant + "." + configurationTypeConstant
	configurationDirectoryPermissionConstant                         = 0o755
	configurationFilePermissionConstant                              = 0o600
	configurationInitializedMessageConstant                          = "configuration initialized"
	configurationLogLevelFieldConstant                               = "log_level"
	configurationLogFormatFieldConstant                              = "log_format"
	configurationFileFieldConstant                                   = "config_file"
	xdgConfigHomeEnvironmentVariableConstant                         = "XDG_CONFIG_HOME"
	configurationLoadErrorTemplateConstant                           = "unable to load configuration: %w"
	loggerCreationErrorTemplateConstant                              = "unable to create logger: %w"
	loggerSyncErrorTemplateConstant                                  = "unable to flush logger: %w"
	configurationInitializedConsoleTemplateConstant                  = "%s | log level=%s | log format=%s | config file=%s"
	rootCommandInfoMessageConstant                                   = "gix CLI executed"
	rootCommandDebugMessageConstant                                  = "gix CLI diagnostics"
	logFieldCommandNameConstant                                      = "command_name"
	logFieldArgumentCountConstant                                    = "argument_count"
	logFieldArgumentsConstant                                        = "arguments"
	loggerNotInitializedMessageConstant                              = "logger not initialized"
	defaultConfigurationSearchPathConstant                           = "."
	userConfigurationDirectoryNameConstant                           = ".gix"
	configurationSearchPathEnvironmentVariableConstant               = "GIX_CONFIG_SEARCH_PATH"
	auditOperationNameConstant                                       = "audit"
	packagesPurgeOperationNameConstant                               = "repo-packages-purge"
	branchCleanupOperationNameConstant                               = "repo-prs-purge"
	reposRenameOperationNameConstant                                 = "repo-folders-rename"
	reposRemotesOperationNameConstant                                = "repo-remote-update"
	reposProtocolOperationNameConstant                               = "repo-protocol-convert"
	workflowCommandOperationNameConstant                             = "workflow"
	branchRefreshOperationNameConstant                               = "branch-refresh"
	branchMigrateOperationNameConstant                               = "branch-migrate"
	auditCommandAliasConstant                                        = "a"
	workflowCommandAliasConstant                                     = "w"
	repoNamespaceUseNameConstant                                     = "repo"
	repoNamespaceAliasConstant                                       = "r"
	repoNamespaceShortDescriptionConstant                            = "Repository maintenance commands"
	repoFolderNamespaceUseNameConstant                               = "folder"
	repoFolderNamespaceAliasConstant                                 = "folders"
	repoFolderNamespaceShortDescriptionConstant                      = "Repository directory commands"
	renameCommandUseNameConstant                                     = "rename"
	repoRemoteNamespaceUseNameConstant                               = "remote"
	repoRemoteNamespaceShortDescriptionConstant                      = "Repository remote commands"
	updateRemoteCanonicalUseNameConstant                             = "update-to-canonical"
	updateRemoteCanonicalAliasConstant                               = "canonical"
	updateProtocolCommandUseNameConstant                             = "update-protocol"
	updateProtocolAliasConstant                                      = "convert"
	repoPullRequestsNamespaceUseNameConstant                         = "prs"
	repoPullRequestsNamespaceShortDescriptionConstant                = "Pull request cleanup commands"
	prsDeleteCommandUseNameConstant                                  = "delete"
	prsDeleteCommandAliasConstant                                    = "purge"
	repoPackagesNamespaceUseNameConstant                             = "packages"
	repoPackagesNamespaceShortDescriptionConstant                    = "GitHub Packages maintenance commands"
	packagesDeleteCommandUseNameConstant                             = "delete"
	packagesDeleteCommandAliasConstant                               = "prune"
	branchNamespaceUseNameConstant                                   = "branch"
	branchNamespaceAliasConstant                                     = "b"
	branchNamespaceShortDescriptionConstant                          = "Branch management commands"
	migrateCommandUseNameConstant                                    = "migrate"
	refreshCommandUseNameConstant                                    = "refresh"
	repoPullRequestsDeleteCompositeKeyConstant                       = repoPullRequestsNamespaceUseNameConstant + "/" + prsDeleteCommandUseNameConstant
	repoPackagesDeleteCompositeKeyConstant                           = repoPackagesNamespaceUseNameConstant + "/" + packagesDeleteCommandUseNameConstant
	renameNestedLongDescriptionConstant                              = "repo folder rename normalizes repository directory names to match canonical GitHub repositories."
	updateRemoteCanonicalLongDescriptionConstant                     = "repo remote update-to-canonical adjusts origin remotes to match canonical GitHub repositories."
	updateProtocolLongDescriptionConstant                            = "repo remote update-protocol converts origin URLs to a desired protocol."
	prsDeleteLongDescriptionConstant                                 = "repo prs delete removes remote and local Git branches whose pull requests are already closed."
	packagesDeleteLongDescriptionConstant                            = "repo packages delete removes untagged container versions from GitHub Packages."
	branchMigrateNestedLongDescriptionConstant                       = "branch migrate switches repository defaults from main to master with safety gates."
	branchRefreshNestedLongDescriptionConstant                       = "branch refresh synchronizes repository branches by fetching, checking out, and pulling updates."
	versionFlagNameConstant                                          = "version"
	versionFlagUsageConstant                                         = "Print the application version and exit"
	versionOutputTemplateConstant                                    = "gix version: %s\n"
	operationDecodeErrorMessageConstant                              = "unable to decode operation defaults"
	operationNameLogFieldConstant                                    = "operation"
	operationErrorLogFieldConstant                                   = "error"
	duplicateOperationConfigurationTemplateConstant                  = "duplicate configuration for operation %q"
	missingOperationConfigurationTemplateConstant                    = "missing configuration for operation %q"
	missingOperationConfigurationSkippedMessageConstant              = "operation configuration missing; continuing without defaults"
	unknownCommandNamePlaceholderConstant                            = "unknown"
	dryRunOptionKeyConstant                                          = "dry_run"
	assumeYesOptionKeyConstant                                       = "assume_yes"
	requireCleanOptionKeyConstant                                    = "require_clean"
	branchFlagNameConstant                                           = "branch"
	branchFlagUsageConstant                                          = "Branch name for command context"
)

var commandOperationRequirements = map[string][]string{
	auditOperationNameConstant:                 {auditOperationNameConstant},
	branchCleanupOperationNameConstant:         {branchCleanupOperationNameConstant},
	branchMigrateOperationNameConstant:         {branchMigrateOperationNameConstant},
	branchRefreshOperationNameConstant:         {branchRefreshOperationNameConstant},
	migrateCommandUseNameConstant:              {branchMigrateOperationNameConstant},
	packagesPurgeOperationNameConstant:         {packagesPurgeOperationNameConstant},
	repoPackagesDeleteCompositeKeyConstant:     {packagesPurgeOperationNameConstant},
	repoPullRequestsDeleteCompositeKeyConstant: {branchCleanupOperationNameConstant},
	refreshCommandUseNameConstant:              {branchRefreshOperationNameConstant},
	renameCommandUseNameConstant:               {reposRenameOperationNameConstant},
	reposProtocolOperationNameConstant:         {reposProtocolOperationNameConstant},
	reposRemotesOperationNameConstant:          {reposRemotesOperationNameConstant},
	reposRenameOperationNameConstant:           {reposRenameOperationNameConstant},
	updateProtocolCommandUseNameConstant:       {reposProtocolOperationNameConstant},
	updateRemoteCanonicalUseNameConstant:       {reposRemotesOperationNameConstant},
	workflowCommandOperationNameConstant:       {workflowCommandOperationNameConstant},
}

var requiredOperationConfigurationNames = collectRequiredOperationConfigurationNames()

type loggerOutputsFactory interface {
	CreateLoggerOutputs(utils.LogLevel, utils.LogFormat) (utils.LoggerOutputs, error)
}

// DuplicateOperationConfigurationError indicates that the configuration file defines the same operation multiple times.
type DuplicateOperationConfigurationError struct {
	OperationName string
}

// Error implements the error interface.
func (errorDetails DuplicateOperationConfigurationError) Error() string {
	return fmt.Sprintf(duplicateOperationConfigurationTemplateConstant, errorDetails.OperationName)
}

// MissingOperationConfigurationError indicates that a referenced operation configuration is absent.
type MissingOperationConfigurationError struct {
	OperationName string
}

// Error implements the error interface.
func (errorDetails MissingOperationConfigurationError) Error() string {
	return fmt.Sprintf(missingOperationConfigurationTemplateConstant, errorDetails.OperationName)
}

// ApplicationConfiguration describes the persisted configuration for the CLI entrypoint.
type ApplicationConfiguration struct {
	Common     ApplicationCommonConfiguration      `mapstructure:"common"`
	Operations []ApplicationOperationConfiguration `mapstructure:"operations"`
}

// ApplicationCommonConfiguration stores logging and execution defaults shared across commands.
type ApplicationCommonConfiguration struct {
	LogLevel     string `mapstructure:"log_level"`
	LogFormat    string `mapstructure:"log_format"`
	DryRun       bool   `mapstructure:"dry_run"`
	AssumeYes    bool   `mapstructure:"assume_yes"`
	RequireClean bool   `mapstructure:"require_clean"`
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

type configurationInitializationPlan struct {
	DirectoryPath string
	FilePath      string
}

func newOperationConfigurations(definitions []ApplicationOperationConfiguration) (OperationConfigurations, error) {
	entries := make(map[string]map[string]any)
	seenOperations := make(map[string]struct{})
	for definitionIndex := range definitions {
		normalizedName := normalizeOperationName(definitions[definitionIndex].Name)
		if len(normalizedName) == 0 {
			continue
		}

		if _, exists := seenOperations[normalizedName]; exists {
			return OperationConfigurations{}, DuplicateOperationConfigurationError{OperationName: normalizedName}
		}
		seenOperations[normalizedName] = struct{}{}

		options := make(map[string]any)
		for optionKey, optionValue := range definitions[definitionIndex].Options {
			options[optionKey] = optionValue
		}

		entries[normalizedName] = options
	}

	return OperationConfigurations{entries: entries}, nil
}

// Lookup returns the configuration options for the provided operation name or an error if the configuration is absent.
func (configurations OperationConfigurations) Lookup(operationName string) (map[string]any, error) {
	normalizedName := normalizeOperationName(operationName)
	if len(normalizedName) == 0 {
		return nil, MissingOperationConfigurationError{OperationName: operationName}
	}

	if configurations.entries == nil {
		return nil, MissingOperationConfigurationError{OperationName: normalizedName}
	}

	options, exists := configurations.entries[normalizedName]
	if !exists {
		return nil, MissingOperationConfigurationError{OperationName: normalizedName}
	}

	duplicatedOptions := make(map[string]any, len(options))
	for optionKey, optionValue := range options {
		duplicatedOptions[optionKey] = optionValue
	}

	return duplicatedOptions, nil
}

func (configurations OperationConfigurations) decode(operationName string, target any) error {
	if target == nil {
		return nil
	}

	options, lookupError := configurations.Lookup(operationName)
	if lookupError != nil {
		return lookupError
	}

	if len(options) == 0 {
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
	rootCommand                       *cobra.Command
	configurationLoader               *utils.ConfigurationLoader
	loggerFactory                     loggerOutputsFactory
	logger                            *zap.Logger
	consoleLogger                     *zap.Logger
	configuration                     ApplicationConfiguration
	configurationMetadata             utils.LoadedConfiguration
	configurationFilePath             string
	logLevelFlagValue                 string
	logFormatFlagValue                string
	commandContextAccessor            utils.CommandContextAccessor
	operationConfigurations           OperationConfigurations
	rootFlagValues                    *flagutils.RootFlagValues
	branchFlagValues                  *flagutils.BranchFlagValues
	configurationInitializationScope  string
	configurationInitializationForced bool
	versionFlag                       bool
	versionResolver                   func(context.Context) string
	exitFunction                      func(int)
}

// NewApplication assembles a fully wired CLI application instance.
func NewApplication() *Application {
	application := &Application{
		loggerFactory:          utils.NewLoggerFactory(),
		logger:                 zap.NewNop(),
		consoleLogger:          zap.NewNop(),
		commandContextAccessor: utils.NewCommandContextAccessor(),
	}
	application.versionResolver = application.resolveVersion
	application.exitFunction = os.Exit

	application.configurationLoader = utils.NewConfigurationLoader(
		configurationNameConstant,
		configurationTypeConstant,
		environmentPrefixConstant,
		application.resolveConfigurationSearchPaths(),
	)

	embeddedConfigurationData, embeddedConfigurationType := EmbeddedDefaultConfiguration()
	application.configurationLoader.SetEmbeddedConfiguration(embeddedConfigurationData, embeddedConfigurationType)

	cobraCommand := &cobra.Command{
		Use:           applicationNameConstant,
		Short:         applicationShortDescriptionConstant,
		Long:          applicationLongDescriptionConstant,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(command *cobra.Command, arguments []string) error {
			if initializationError := application.initializeConfiguration(command); initializationError != nil {
				return initializationError
			}

			versionRequested := application.versionFlag
			if command != nil {
				if flagValue, flagChanged, flagError := flagutils.BoolFlag(command, versionFlagNameConstant); flagError == nil && flagChanged {
					versionRequested = flagValue
				}
			}

			if versionRequested {
				versionString := application.versionResolver(command.Context())
				fmt.Printf(versionOutputTemplateConstant, versionString)
				application.exitFunction(0)
			}

			return nil
		},
		RunE: func(command *cobra.Command, arguments []string) error {
			return application.runRootCommand(command, arguments)
		},
	}

	cobraCommand.SetContext(context.Background())
	cobraCommand.PersistentFlags().StringVar(&application.configurationFilePath, configFileFlagNameConstant, "", configFileFlagUsageConstant)
	cobraCommand.PersistentFlags().StringVar(&application.logLevelFlagValue, logLevelFlagNameConstant, "", logLevelFlagUsageConstant)
	cobraCommand.PersistentFlags().StringVar(&application.logFormatFlagValue, logFormatFlagNameConstant, "", logFormatFlagUsageConstant)
	cobraCommand.PersistentFlags().StringVar(
		&application.configurationInitializationScope,
		configurationInitializationFlagNameConstant,
		configurationInitializationDefaultScopeConstant,
		configurationInitializationFlagUsageConstant,
	)
	initializationFlag := cobraCommand.PersistentFlags().Lookup(configurationInitializationFlagNameConstant)
	if initializationFlag != nil {
		initializationFlag.NoOptDefVal = configurationInitializationDefaultScopeConstant
	}
	cobraCommand.PersistentFlags().BoolVar(
		&application.configurationInitializationForced,
		configurationInitializationForceFlagNameConstant,
		false,
		configurationInitializationForceFlagUsageConstant,
	)

	application.rootFlagValues = flagutils.BindRootFlags(
		cobraCommand,
		flagutils.RootFlagValues{},
		flagutils.RootFlagDefinition{Name: flagutils.DefaultRootFlagName, Usage: flagutils.DefaultRootFlagUsage, Enabled: true, Persistent: true},
	)

	flagutils.BindExecutionFlags(
		cobraCommand,
		flagutils.ExecutionDefaults{},
		flagutils.ExecutionFlagDefinitions{
			DryRun:    flagutils.ExecutionFlagDefinition{Name: flagutils.DryRunFlagName, Usage: flagutils.DryRunFlagUsage, Enabled: true},
			AssumeYes: flagutils.ExecutionFlagDefinition{Name: flagutils.AssumeYesFlagName, Usage: flagutils.AssumeYesFlagUsage, Shorthand: flagutils.AssumeYesFlagShorthand, Enabled: true},
		},
	)

	cobraCommand.PersistentFlags().String(flagutils.RemoteFlagName, "", flagutils.RemoteFlagUsage)

	application.branchFlagValues = flagutils.BindBranchFlags(
		cobraCommand,
		flagutils.BranchFlagValues{},
		flagutils.BranchFlagDefinition{Name: branchFlagNameConstant, Usage: branchFlagUsageConstant, Enabled: true},
	)

	cobraCommand.PersistentFlags().BoolVar(&application.versionFlag, versionFlagNameConstant, false, versionFlagUsageConstant)

	auditBuilder := audit.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.auditCommandConfiguration,
	}
	auditCommand, auditBuildError := auditBuilder.Build()
	if auditBuildError == nil {
		auditCommand.Aliases = appendUnique(auditCommand.Aliases, auditCommandAliasConstant)
		cobraCommand.AddCommand(auditCommand)
	}

	branchCleanupBuilder := branches.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.branchCleanupConfiguration,
		PrompterFactory: func(command *cobra.Command) shared.ConfirmationPrompter {
			if command == nil {
				return nil
			}
			return prompt.NewIOConfirmationPrompter(command.InOrStdin(), command.OutOrStdout())
		},
	}

	branchRefreshBuilder := branchrefresh.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.branchRefreshConfiguration,
	}

	branchMigrationBuilder := migrate.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.branchMigrateConfiguration,
	}

	packagesBuilder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		ConfigurationProvider: application.packagesConfiguration,
	}

	renameBuilder := repos.RenameCommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.reposRenameConfiguration,
	}

	remotesBuilder := repos.RemotesCommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.reposRemotesConfiguration,
	}

	protocolBuilder := repos.ProtocolCommandBuilder{
		LoggerProvider: func() *zap.Logger {
			return application.logger
		},
		HumanReadableLoggingProvider: application.humanReadableLoggingEnabled,
		ConfigurationProvider:        application.reposProtocolConfiguration,
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
		workflowCommand.Aliases = appendUnique(workflowCommand.Aliases, workflowCommandAliasConstant)
		cobraCommand.AddCommand(workflowCommand)
	}

	repoNamespaceCommand := newNamespaceCommand(repoNamespaceUseNameConstant, repoNamespaceShortDescriptionConstant, repoNamespaceAliasConstant)

	repoFolderCommand := newNamespaceCommand(repoFolderNamespaceUseNameConstant, repoFolderNamespaceShortDescriptionConstant, repoFolderNamespaceAliasConstant)
	if renameNestedCommand, nestedRenameError := renameBuilder.Build(); nestedRenameError == nil {
		configureCommandMetadata(renameNestedCommand, renameCommandUseNameConstant, renameNestedCommand.Short, renameNestedLongDescriptionConstant)
		repoFolderCommand.AddCommand(renameNestedCommand)
	}
	if len(repoFolderCommand.Commands()) > 0 {
		repoNamespaceCommand.AddCommand(repoFolderCommand)
	}

	repoRemoteCommand := newNamespaceCommand(repoRemoteNamespaceUseNameConstant, repoRemoteNamespaceShortDescriptionConstant)
	if canonicalRemoteCommand, canonicalRemoteError := remotesBuilder.Build(); canonicalRemoteError == nil {
		configureCommandMetadata(canonicalRemoteCommand, updateRemoteCanonicalUseNameConstant, canonicalRemoteCommand.Short, updateRemoteCanonicalLongDescriptionConstant, updateRemoteCanonicalAliasConstant)
		repoRemoteCommand.AddCommand(canonicalRemoteCommand)
	}
	if protocolConversionCommand, protocolConversionError := protocolBuilder.Build(); protocolConversionError == nil {
		configureCommandMetadata(protocolConversionCommand, updateProtocolCommandUseNameConstant, protocolConversionCommand.Short, updateProtocolLongDescriptionConstant, updateProtocolAliasConstant)
		repoRemoteCommand.AddCommand(protocolConversionCommand)
	}
	if len(repoRemoteCommand.Commands()) > 0 {
		repoNamespaceCommand.AddCommand(repoRemoteCommand)
	}

	repoPullRequestsCommand := newNamespaceCommand(repoPullRequestsNamespaceUseNameConstant, repoPullRequestsNamespaceShortDescriptionConstant)
	if pullRequestCleanupCommand, pullRequestCleanupError := branchCleanupBuilder.Build(); pullRequestCleanupError == nil {
		configureCommandMetadata(pullRequestCleanupCommand, prsDeleteCommandUseNameConstant, pullRequestCleanupCommand.Short, prsDeleteLongDescriptionConstant, prsDeleteCommandAliasConstant)
		repoPullRequestsCommand.AddCommand(pullRequestCleanupCommand)
	}
	if len(repoPullRequestsCommand.Commands()) > 0 {
		repoNamespaceCommand.AddCommand(repoPullRequestsCommand)
	}

	repoPackagesCommand := newNamespaceCommand(repoPackagesNamespaceUseNameConstant, repoPackagesNamespaceShortDescriptionConstant)
	if packagesCleanupCommand, packagesCleanupError := packagesBuilder.Build(); packagesCleanupError == nil {
		configureCommandMetadata(packagesCleanupCommand, packagesDeleteCommandUseNameConstant, packagesCleanupCommand.Short, packagesDeleteLongDescriptionConstant, packagesDeleteCommandAliasConstant)
		repoPackagesCommand.AddCommand(packagesCleanupCommand)
	}
	if len(repoPackagesCommand.Commands()) > 0 {
		repoNamespaceCommand.AddCommand(repoPackagesCommand)
	}

	if len(repoNamespaceCommand.Commands()) > 0 {
		cobraCommand.AddCommand(repoNamespaceCommand)
	}

	branchNamespaceCommand := newNamespaceCommand(branchNamespaceUseNameConstant, branchNamespaceShortDescriptionConstant, branchNamespaceAliasConstant)
	if branchMigrateNestedCommand, branchMigrateNestedError := branchMigrationBuilder.Build(); branchMigrateNestedError == nil {
		configureCommandMetadata(branchMigrateNestedCommand, migrateCommandUseNameConstant, branchMigrateNestedCommand.Short, branchMigrateNestedLongDescriptionConstant)
		branchNamespaceCommand.AddCommand(branchMigrateNestedCommand)
	}
	if branchRefreshNestedCommand, branchRefreshNestedError := branchRefreshBuilder.Build(); branchRefreshNestedError == nil {
		configureCommandMetadata(branchRefreshNestedCommand, refreshCommandUseNameConstant, branchRefreshNestedCommand.Short, branchRefreshNestedLongDescriptionConstant)
		branchNamespaceCommand.AddCommand(branchRefreshNestedCommand)
	}
	if len(branchNamespaceCommand.Commands()) > 0 {
		cobraCommand.AddCommand(branchNamespaceCommand)
	}

	application.rootCommand = cobraCommand

	return application
}

// Execute runs the configured Cobra command hierarchy and ensures logger flushing.
func (application *Application) Execute() error {
	application.rootCommand.SetArgs(flagutils.NormalizeToggleArguments(os.Args[1:]))

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
		defaultSearchPaths := []string{defaultConfigurationSearchPathConstant}
		userConfigurationDirectoryPaths := application.resolveUserConfigurationDirectoryPaths()
		if len(userConfigurationDirectoryPaths) > 0 {
			defaultSearchPaths = append(defaultSearchPaths, userConfigurationDirectoryPaths...)
		}

		return defaultSearchPaths
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

func (application *Application) resolveUserConfigurationDirectoryPaths() []string {
	userConfigurationDirectoryPaths := make([]string, 0, 3)

	appendConfigurationDirectory := func(baseDirectoryPath string) {
		trimmedBaseDirectoryPath := strings.TrimSpace(baseDirectoryPath)
		if len(trimmedBaseDirectoryPath) == 0 {
			return
		}

		candidateDirectoryPath := filepath.Join(trimmedBaseDirectoryPath, userConfigurationDirectoryNameConstant)
		for _, existingDirectoryPath := range userConfigurationDirectoryPaths {
			if existingDirectoryPath == candidateDirectoryPath {
				return
			}
		}

		userConfigurationDirectoryPaths = append(userConfigurationDirectoryPaths, candidateDirectoryPath)
	}

	appendConfigurationDirectory(os.Getenv(xdgConfigHomeEnvironmentVariableConstant))

	userConfigurationBaseDirectoryPath, userConfigurationDirectoryError := os.UserConfigDir()
	if userConfigurationDirectoryError == nil {
		appendConfigurationDirectory(userConfigurationBaseDirectoryPath)
	}

	userHomeDirectoryPath, userHomeDirectoryError := os.UserHomeDir()
	if userHomeDirectoryError == nil {
		appendConfigurationDirectory(userHomeDirectoryPath)
	}

	return userConfigurationDirectoryPaths
}

func (application *Application) initializeConfiguration(command *cobra.Command) error {
	defaultValues := map[string]any{
		commonLogLevelConfigKeyConstant:     string(utils.LogLevelInfo),
		commonLogFormatConfigKeyConstant:    string(utils.LogFormatStructured),
		commonDryRunConfigKeyConstant:       false,
		commonAssumeYesConfigKeyConstant:    false,
		commonRequireCleanConfigKeyConstant: false,
	}

	loadedConfiguration, loadError := application.configurationLoader.LoadConfiguration(application.configurationFilePath, defaultValues, &application.configuration)
	if loadError != nil {
		return fmt.Errorf(configurationLoadErrorTemplateConstant, loadError)
	}

	application.configurationMetadata = loadedConfiguration

	operationConfigurations, configurationBuildError := newOperationConfigurations(application.configuration.Operations)
	if configurationBuildError != nil {
		return configurationBuildError
	}
	application.operationConfigurations = operationConfigurations

	if validationError := application.validateOperationConfigurations(command); validationError != nil {
		return validationError
	}

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
	if application.logger == nil {
		application.logger = zap.NewNop()
	}

	application.consoleLogger = loggerOutputs.ConsoleLogger
	if application.consoleLogger == nil {
		application.consoleLogger = zap.NewNop()
	}

	application.logConfigurationInitialization()

	if command != nil {
		updatedContext := application.commandContextAccessor.WithConfigurationFilePath(
			command.Context(),
			application.configurationMetadata.ConfigFileUsed,
		)

		executionFlags := application.collectExecutionFlags(command)
		updatedContext = application.commandContextAccessor.WithExecutionFlags(updatedContext, executionFlags)
		updatedContext = application.commandContextAccessor.WithLogLevel(updatedContext, application.configuration.Common.LogLevel)

		branchContext := utils.BranchContext{RequireClean: true}
		if application.branchFlagValues != nil {
			branchContext.Name = application.branchFlagValues.Name
		}
		updatedContext = application.commandContextAccessor.WithBranchContext(updatedContext, branchContext)

		command.SetContext(updatedContext)
		if rootCommand := command.Root(); rootCommand != nil {
			rootCommand.SetContext(updatedContext)
		}
	}

	return nil
}

// InitializeForCommand prepares application state for the provided command name without executing command logic.
func (application *Application) InitializeForCommand(commandUse string) error {
	command := &cobra.Command{Use: commandUse}
	return application.initializeConfiguration(command)
}

func (application *Application) humanReadableLoggingEnabled() bool {
	logFormatValue := strings.TrimSpace(application.configuration.Common.LogFormat)
	return strings.EqualFold(logFormatValue, string(utils.LogFormatConsole))
}

func (application *Application) logConfigurationInitialization() {
	if application.humanReadableLoggingEnabled() {
		bannerMessage := fmt.Sprintf(
			configurationInitializedConsoleTemplateConstant,
			configurationInitializedMessageConstant,
			application.configuration.Common.LogLevel,
			application.configuration.Common.LogFormat,
			application.configurationMetadata.ConfigFileUsed,
		)
		application.consoleLogger.Info(bannerMessage)
		return
	}

	application.logger.Info(
		configurationInitializedMessageConstant,
		zap.String(configurationLogLevelFieldConstant, application.configuration.Common.LogLevel),
		zap.String(configurationLogFormatFieldConstant, application.configuration.Common.LogFormat),
		zap.String(configurationFileFieldConstant, application.configurationMetadata.ConfigFileUsed),
	)
}

func (application *Application) collectExecutionFlags(command *cobra.Command) utils.ExecutionFlags {
	executionFlags := utils.ExecutionFlags{}
	if command == nil {
		return executionFlags
	}

	if dryRunValue, dryRunChanged, dryRunError := flagutils.BoolFlag(command, flagutils.DryRunFlagName); dryRunError == nil {
		executionFlags.DryRun = dryRunValue
		executionFlags.DryRunSet = dryRunChanged
	}

	if assumeYesValue, assumeYesChanged, assumeYesError := flagutils.BoolFlag(command, flagutils.AssumeYesFlagName); assumeYesError == nil {
		executionFlags.AssumeYes = assumeYesValue
		executionFlags.AssumeYesSet = assumeYesChanged
	}

	if remoteValue, remoteChanged, remoteError := flagutils.StringFlag(command, flagutils.RemoteFlagName); remoteError == nil {
		trimmedRemote := strings.TrimSpace(remoteValue)
		executionFlags.Remote = trimmedRemote
		executionFlags.RemoteSet = remoteChanged && len(trimmedRemote) > 0
	}

	return executionFlags
}

func (application *Application) auditCommandConfiguration() audit.CommandConfiguration {
	var configuration audit.CommandConfiguration
	application.decodeOperationConfiguration(auditOperationNameConstant, &configuration)
	if strings.EqualFold(application.configuration.Common.LogLevel, string(utils.LogLevelDebug)) {
		configuration.Debug = true
	}
	return configuration
}

func (application *Application) packagesConfiguration() packages.Configuration {
	configuration := packages.DefaultConfiguration()
	application.decodeOperationConfiguration(packagesPurgeOperationNameConstant, &configuration.Purge)

	options, optionsExist := application.lookupOperationOptions(packagesPurgeOperationNameConstant)
	if !optionsExist || !optionExists(options, dryRunOptionKeyConstant) {
		configuration.Purge.DryRun = application.configuration.Common.DryRun
	}
	return configuration
}

func (application *Application) branchCleanupConfiguration() branches.CommandConfiguration {
	configuration := branches.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(branchCleanupOperationNameConstant, &configuration)

	options, optionsExist := application.lookupOperationOptions(branchCleanupOperationNameConstant)
	if !optionsExist || !optionExists(options, dryRunOptionKeyConstant) {
		configuration.DryRun = application.configuration.Common.DryRun
	}
	if !optionsExist || !optionExists(options, assumeYesOptionKeyConstant) {
		configuration.AssumeYes = application.configuration.Common.AssumeYes
	}

	return configuration
}

func (application *Application) branchRefreshConfiguration() branchrefresh.CommandConfiguration {
	configuration := branchrefresh.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(branchRefreshOperationNameConstant, &configuration)
	return configuration.Sanitize()
}

func (application *Application) resolveVersion(executionContext context.Context) string {
	dependencies := version.Dependencies{}
	gitExecutor, executorError := reposdeps.ResolveGitExecutor(nil, application.logger, application.humanReadableLoggingEnabled())
	if executorError == nil {
		dependencies.GitExecutor = gitExecutor
	}

	resolved := version.Detect(executionContext, dependencies)
	trimmed := strings.TrimSpace(resolved)
	if len(trimmed) == 0 {
		return resolved
	}
	return trimmed
}

func (application *Application) reposRenameConfiguration() repos.RenameConfiguration {
	configuration := repos.DefaultToolsConfiguration().Rename
	application.decodeOperationConfiguration(reposRenameOperationNameConstant, &configuration)

	options, optionsExist := application.lookupOperationOptions(reposRenameOperationNameConstant)
	if !optionsExist || !optionExists(options, dryRunOptionKeyConstant) {
		configuration.DryRun = application.configuration.Common.DryRun
	}
	if !optionsExist || !optionExists(options, assumeYesOptionKeyConstant) {
		configuration.AssumeYes = application.configuration.Common.AssumeYes
	}
	if !optionsExist || !optionExists(options, requireCleanOptionKeyConstant) {
		configuration.RequireCleanWorktree = application.configuration.Common.RequireClean
	}

	return configuration
}

func (application *Application) reposRemotesConfiguration() repos.RemotesConfiguration {
	configuration := repos.DefaultToolsConfiguration().Remotes
	application.decodeOperationConfiguration(reposRemotesOperationNameConstant, &configuration)

	options, optionsExist := application.lookupOperationOptions(reposRemotesOperationNameConstant)
	if !optionsExist || !optionExists(options, dryRunOptionKeyConstant) {
		configuration.DryRun = application.configuration.Common.DryRun
	}
	if !optionsExist || !optionExists(options, assumeYesOptionKeyConstant) {
		configuration.AssumeYes = application.configuration.Common.AssumeYes
	}

	return configuration
}

func (application *Application) reposProtocolConfiguration() repos.ProtocolConfiguration {
	configuration := repos.DefaultToolsConfiguration().Protocol
	application.decodeOperationConfiguration(reposProtocolOperationNameConstant, &configuration)

	options, optionsExist := application.lookupOperationOptions(reposProtocolOperationNameConstant)
	if !optionsExist || !optionExists(options, dryRunOptionKeyConstant) {
		configuration.DryRun = application.configuration.Common.DryRun
	}
	if !optionsExist || !optionExists(options, assumeYesOptionKeyConstant) {
		configuration.AssumeYes = application.configuration.Common.AssumeYes
	}

	return configuration
}

func (application *Application) workflowCommandConfiguration() workflowcmd.CommandConfiguration {
	configuration := workflowcmd.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(workflowCommandOperationNameConstant, &configuration)

	options, optionsExist := application.lookupOperationOptions(workflowCommandOperationNameConstant)
	if !optionsExist || !optionExists(options, dryRunOptionKeyConstant) {
		configuration.DryRun = application.configuration.Common.DryRun
	}
	if !optionsExist || !optionExists(options, assumeYesOptionKeyConstant) {
		configuration.AssumeYes = application.configuration.Common.AssumeYes
	}
	if !optionsExist || !optionExists(options, requireCleanOptionKeyConstant) {
		configuration.RequireClean = application.configuration.Common.RequireClean
	}

	return configuration
}

func (application *Application) branchMigrateConfiguration() migrate.CommandConfiguration {
	configuration := migrate.DefaultCommandConfiguration()
	application.decodeOperationConfiguration(branchMigrateOperationNameConstant, &configuration)
	if strings.EqualFold(application.configuration.Common.LogLevel, string(utils.LogLevelDebug)) {
		configuration.EnableDebugLogging = true
	}
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

func (application *Application) lookupOperationOptions(operationName string) (map[string]any, bool) {
	options, lookupError := application.operationConfigurations.Lookup(operationName)
	if lookupError != nil {
		return nil, false
	}
	return options, true
}

func optionExists(options map[string]any, optionKey string) bool {
	if len(options) == 0 {
		return false
	}

	normalizedOptionKey := strings.ToLower(strings.TrimSpace(optionKey))
	for candidateKey := range options {
		if strings.ToLower(strings.TrimSpace(candidateKey)) == normalizedOptionKey {
			return true
		}
	}

	return false
}

func (application *Application) operationOptionExists(operationName string, optionKey string) bool {
	options, exists := application.lookupOperationOptions(operationName)
	if !exists {
		return false
	}

	return optionExists(options, optionKey)
}

func (application *Application) validateOperationConfigurations(command *cobra.Command) error {
	if len(application.configuration.Operations) == 0 {
		return nil
	}

	requiredOperations := application.operationsRequiredForCommand(command)
	if len(requiredOperations) == 0 {
		return nil
	}

	for operationIndex := range requiredOperations {
		operationName := requiredOperations[operationIndex]
		_, lookupError := application.operationConfigurations.Lookup(operationName)
		if lookupError == nil {
			continue
		}

		var missingConfigurationError MissingOperationConfigurationError
		if errors.As(lookupError, &missingConfigurationError) && command != nil {
			commandName := strings.TrimSpace(command.Name())
			if len(commandName) == 0 && command.HasParent() {
				parentCommand := command.Parent()
				commandName = strings.TrimSpace(parentCommand.Name())
			}

			application.logMissingOperationConfiguration(commandName, operationName)
			continue
		}

		return lookupError
	}

	return nil
}

func (application *Application) logMissingOperationConfiguration(commandName string, operationName string) {
	if application.logger == nil {
		return
	}

	normalizedCommandName := strings.TrimSpace(commandName)
	if len(normalizedCommandName) == 0 {
		normalizedCommandName = unknownCommandNamePlaceholderConstant
	}

	application.logger.Info(
		missingOperationConfigurationSkippedMessageConstant,
		zap.String(logFieldCommandNameConstant, normalizedCommandName),
		zap.String(operationNameLogFieldConstant, operationName),
	)
}

func (application *Application) operationsRequiredForCommand(command *cobra.Command) []string {
	if command == nil {
		return requiredOperationConfigurationNames
	}

	commandName := strings.TrimSpace(command.Name())
	if len(commandName) == 0 {
		return requiredOperationConfigurationNames
	}

	if requiredOperations, exists := commandOperationRequirements[commandName]; exists {
		return requiredOperations
	}

	if parentCommand := command.Parent(); parentCommand != nil {
		parentName := strings.TrimSpace(parentCommand.Name())
		if len(parentName) > 0 {
			compositeKey := parentName + "/" + commandName
			if requiredOperations, exists := commandOperationRequirements[compositeKey]; exists {
				return requiredOperations
			}
		}
		return application.operationsRequiredForCommand(parentCommand)
	}

	return nil
}

func appendUnique(values []string, candidates ...string) []string {
	result := values
	for _, candidate := range candidates {
		trimmedCandidate := strings.TrimSpace(candidate)
		if len(trimmedCandidate) == 0 {
			continue
		}
		duplicate := false
		for _, existing := range result {
			if existing == trimmedCandidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, trimmedCandidate)
		}
	}
	return result
}

func configureCommandMetadata(command *cobra.Command, use string, shortDescription string, longDescription string, aliases ...string) {
	if command == nil {
		return
	}

	useValue := strings.TrimSpace(use)
	if len(useValue) > 0 {
		command.Use = useValue
	}

	shortValue := strings.TrimSpace(shortDescription)
	if len(shortValue) > 0 {
		command.Short = shortValue
	}

	longValue := strings.TrimSpace(longDescription)
	if len(longValue) > 0 {
		command.Long = longValue
	}

	command.Aliases = appendUnique(command.Aliases, aliases...)
}

func newNamespaceCommand(use string, shortDescription string, aliases ...string) *cobra.Command {
	useValue := strings.TrimSpace(use)
	if len(useValue) == 0 {
		useValue = repoNamespaceUseNameConstant
	}

	shortValue := strings.TrimSpace(shortDescription)
	if len(shortValue) == 0 {
		shortValue = applicationShortDescriptionConstant
	}

	command := &cobra.Command{
		Use:           useValue,
		Short:         shortValue,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	command.Aliases = appendUnique(command.Aliases, aliases...)

	return command
}

func collectRequiredOperationConfigurationNames() []string {
	uniqueNames := make(map[string]struct{})
	for _, operationNames := range commandOperationRequirements {
		for _, operationName := range operationNames {
			uniqueNames[operationName] = struct{}{}
		}
	}

	orderedNames := make([]string, 0, len(uniqueNames))
	for operationName := range uniqueNames {
		orderedNames = append(orderedNames, operationName)
	}

	sort.Strings(orderedNames)

	return orderedNames
}

func (application *Application) handleConfigurationInitialization(command *cobra.Command) (bool, error) {
	if !application.configurationInitializationRequested(command) {
		return false, nil
	}

	initializationScope := strings.TrimSpace(application.configurationInitializationScope)
	if len(initializationScope) == 0 {
		initializationScope = configurationInitializationDefaultScopeConstant
	}

	initializationPlan, planError := application.resolveConfigurationInitializationPlan(initializationScope)
	if planError != nil {
		return true, planError
	}

	configurationContent, _ := EmbeddedDefaultConfiguration()
	if len(configurationContent) == 0 {
		return true, errors.New(configurationInitializationContentUnavailableErrorConstant)
	}

	if writeError := application.writeConfigurationFile(initializationPlan, configurationContent); writeError != nil {
		return true, writeError
	}

	application.logger.Info(
		configurationInitializationSuccessMessageConstant,
		zap.String(configurationFileFieldConstant, initializationPlan.FilePath),
	)

	return true, nil
}

func (application *Application) configurationInitializationRequested(command *cobra.Command) bool {
	return application.persistentFlagChanged(command, configurationInitializationFlagNameConstant)
}

func (application *Application) resolveConfigurationInitializationPlan(initializationScope string) (configurationInitializationPlan, error) {
	normalizedScope := strings.ToLower(strings.TrimSpace(initializationScope))
	switch normalizedScope {
	case "", configurationInitializationScopeLocalConstant:
		workingDirectoryPath, workingDirectoryError := os.Getwd()
		if workingDirectoryError != nil {
			return configurationInitializationPlan{}, fmt.Errorf(configurationInitializationWorkingDirectoryErrorTemplateConstant, workingDirectoryError)
		}

		trimmedWorkingDirectoryPath := strings.TrimSpace(workingDirectoryPath)
		if len(trimmedWorkingDirectoryPath) == 0 {
			return configurationInitializationPlan{}, fmt.Errorf(
				configurationInitializationWorkingDirectoryErrorTemplateConstant,
				errors.New(configurationInitializationWorkingDirectoryEmptyErrorConstant),
			)
		}

		return configurationInitializationPlan{
			DirectoryPath: trimmedWorkingDirectoryPath,
			FilePath:      filepath.Join(trimmedWorkingDirectoryPath, configurationFileNameConstant),
		}, nil
	case configurationInitializationScopeUserConstant:
		userHomeDirectoryPath, userHomeDirectoryError := os.UserHomeDir()
		if userHomeDirectoryError != nil {
			return configurationInitializationPlan{}, fmt.Errorf(configurationInitializationHomeDirectoryErrorTemplateConstant, userHomeDirectoryError)
		}

		trimmedHomeDirectoryPath := strings.TrimSpace(userHomeDirectoryPath)
		if len(trimmedHomeDirectoryPath) == 0 {
			return configurationInitializationPlan{}, fmt.Errorf(
				configurationInitializationHomeDirectoryErrorTemplateConstant,
				errors.New(configurationInitializationHomeDirectoryEmptyErrorConstant),
			)
		}

		configurationDirectoryPath := filepath.Join(trimmedHomeDirectoryPath, userConfigurationDirectoryNameConstant)

		return configurationInitializationPlan{
			DirectoryPath: configurationDirectoryPath,
			FilePath:      filepath.Join(configurationDirectoryPath, configurationFileNameConstant),
		}, nil
	default:
		trimmedScope := strings.TrimSpace(initializationScope)
		if len(trimmedScope) == 0 {
			trimmedScope = initializationScope
		}
		return configurationInitializationPlan{}, fmt.Errorf(configurationInitializationUnsupportedScopeTemplateConstant, trimmedScope)
	}
}

func (application *Application) writeConfigurationFile(initializationPlan configurationInitializationPlan, configurationContent []byte) error {
	if len(configurationContent) == 0 {
		return errors.New(configurationInitializationContentUnavailableErrorConstant)
	}

	directoryPath := strings.TrimSpace(initializationPlan.DirectoryPath)
	if len(directoryPath) == 0 {
		return fmt.Errorf(
			configurationInitializationDirectoryErrorTemplateConstant,
			initializationPlan.DirectoryPath,
			errors.New(configurationInitializationWorkingDirectoryEmptyErrorConstant),
		)
	}

	directoryInfo, directoryStatError := os.Stat(directoryPath)
	switch {
	case directoryStatError == nil:
		if !directoryInfo.IsDir() {
			return fmt.Errorf(configurationInitializationDirectoryConflictTemplateConstant, directoryPath)
		}
	case errors.Is(directoryStatError, os.ErrNotExist):
		if createError := os.MkdirAll(directoryPath, configurationDirectoryPermissionConstant); createError != nil {
			return fmt.Errorf(configurationInitializationDirectoryErrorTemplateConstant, directoryPath, createError)
		}
	default:
		return fmt.Errorf(configurationInitializationDirectoryErrorTemplateConstant, directoryPath, directoryStatError)
	}

	fileInfo, fileStatError := os.Stat(initializationPlan.FilePath)
	switch {
	case fileStatError == nil:
		if fileInfo.IsDir() {
			return fmt.Errorf(configurationInitializationExistingDirectoryTemplateConstant, initializationPlan.FilePath)
		}
		if !application.configurationInitializationForced {
			return fmt.Errorf(configurationInitializationExistingFileTemplateConstant, initializationPlan.FilePath)
		}
	case errors.Is(fileStatError, os.ErrNotExist):
	default:
		return fmt.Errorf(configurationInitializationWriteErrorTemplateConstant, initializationPlan.FilePath, fileStatError)
	}

	writeError := os.WriteFile(initializationPlan.FilePath, configurationContent, configurationFilePermissionConstant)
	if writeError != nil {
		return fmt.Errorf(configurationInitializationWriteErrorTemplateConstant, initializationPlan.FilePath, writeError)
	}

	return nil
}

func (application *Application) runRootCommand(command *cobra.Command, arguments []string) error {
	if application.logger == nil {
		return errors.New(loggerNotInitializedMessageConstant)
	}

	initializationHandled, initializationError := application.handleConfigurationInitialization(command)
	if initializationError != nil {
		return initializationError
	}
	if initializationHandled {
		return nil
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

	if syncError := application.syncLoggerInstance(application.consoleLogger); syncError != nil {
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
	case errors.Is(syncError, syscall.ENOTTY):
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
