package packages

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/ghcr"
)

const (
	packagesPurgeCommandUseConstant              = "repo-packages-purge"
	packagesPurgeCommandShortDescriptionConstant = "Delete untagged GHCR versions"
	packagesPurgeCommandLongDescriptionConstant  = "repo-packages-purge removes untagged container versions from GitHub Container Registry."
	unexpectedArgumentsErrorMessageConstant      = "repo-packages-purge does not accept positional arguments"
	commandExecutionErrorTemplateConstant        = "repo-packages-purge failed: %w"
	ownerFlagNameConstant                        = "owner"
	ownerFlagDescriptionConstant                 = "GitHub user or organization that owns the package"
	packageFlagNameConstant                      = "package"
	packageFlagDescriptionConstant               = "Container package name in GHCR"
	ownerTypeFlagNameConstant                    = "owner-type"
	ownerTypeFlagDescriptionConstant             = "Owner type: user or org"
	tokenSourceFlagNameConstant                  = "token-source"
	tokenSourceFlagDescriptionConstant           = "Token source (env:NAME or file:/path)"
	dryRunFlagNameConstant                       = "dry-run"
	dryRunFlagDescriptionConstant                = "Preview deletions without modifying GHCR"
	ownerTypeParseErrorTemplateConstant          = "invalid owner type: %w"
	tokenSourceParseErrorTemplateConstant        = "invalid token source: %w"
)

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// ConfigurationProvider returns the current packages configuration.
type ConfigurationProvider func() Configuration

// PurgeServiceResolver creates purge executors for the command.
type PurgeServiceResolver interface {
	Resolve(logger *zap.Logger) (PurgeExecutor, error)
}

// CommandBuilder assembles the repo-packages-purge command.
type CommandBuilder struct {
	LoggerProvider        LoggerProvider
	ConfigurationProvider ConfigurationProvider
	ServiceResolver       PurgeServiceResolver
	HTTPClient            ghcr.HTTPClient
	EnvironmentLookup     EnvironmentLookup
	FileReader            FileReader
	TokenResolver         TokenResolver
}

// Build constructs the repo-packages-purge command with purge functionality.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	purgeCommand := &cobra.Command{
		Use:   packagesPurgeCommandUseConstant,
		Short: packagesPurgeCommandShortDescriptionConstant,
		Long:  packagesPurgeCommandLongDescriptionConstant,
		RunE:  builder.runPurge,
	}

	purgeCommand.Flags().String(ownerFlagNameConstant, "", ownerFlagDescriptionConstant)
	purgeCommand.Flags().String(packageFlagNameConstant, "", packageFlagDescriptionConstant)
	purgeCommand.Flags().String(ownerTypeFlagNameConstant, "", ownerTypeFlagDescriptionConstant)
	purgeCommand.Flags().String(tokenSourceFlagNameConstant, "", tokenSourceFlagDescriptionConstant)
	purgeCommand.Flags().Bool(dryRunFlagNameConstant, false, dryRunFlagDescriptionConstant)

	return purgeCommand, nil
}

func (builder *CommandBuilder) runPurge(command *cobra.Command, arguments []string) error {
	if len(arguments) > 0 {
		return errors.New(unexpectedArgumentsErrorMessageConstant)
	}

	purgeOptions, optionsError := builder.parsePurgeOptions(command)
	if optionsError != nil {
		return optionsError
	}

	logger := builder.resolveLogger()
	purgeService, serviceError := builder.resolvePurgeService(logger)
	if serviceError != nil {
		return serviceError
	}

	_, executionError := purgeService.Execute(command.Context(), purgeOptions)
	if executionError != nil {
		return fmt.Errorf(commandExecutionErrorTemplateConstant, executionError)
	}

	return nil
}

func (builder *CommandBuilder) parsePurgeOptions(command *cobra.Command) (PurgeOptions, error) {
	configuration := builder.resolveConfiguration()

	ownerFlagValue, ownerFlagError := command.Flags().GetString(ownerFlagNameConstant)
	if ownerFlagError != nil {
		return PurgeOptions{}, ownerFlagError
	}
	ownerValue := selectStringValue(ownerFlagValue, configuration.Purge.Owner)

	packageFlagValue, packageFlagError := command.Flags().GetString(packageFlagNameConstant)
	if packageFlagError != nil {
		return PurgeOptions{}, packageFlagError
	}
	packageValue := selectStringValue(packageFlagValue, configuration.Purge.PackageName)

	ownerTypeFlagValue, ownerTypeFlagError := command.Flags().GetString(ownerTypeFlagNameConstant)
	if ownerTypeFlagError != nil {
		return PurgeOptions{}, ownerTypeFlagError
	}
	ownerTypeValue := selectStringValue(ownerTypeFlagValue, configuration.Purge.OwnerType)
	parsedOwnerType, ownerTypeParseError := ghcr.ParseOwnerType(ownerTypeValue)
	if ownerTypeParseError != nil {
		return PurgeOptions{}, fmt.Errorf(ownerTypeParseErrorTemplateConstant, ownerTypeParseError)
	}

	tokenSourceFlagValue, tokenSourceFlagError := command.Flags().GetString(tokenSourceFlagNameConstant)
	if tokenSourceFlagError != nil {
		return PurgeOptions{}, tokenSourceFlagError
	}
	tokenSourceValue := selectStringValue(tokenSourceFlagValue, configuration.Purge.TokenSource)
	if len(strings.TrimSpace(tokenSourceValue)) == 0 {
		tokenSourceValue = defaultTokenSourceValueConstant
	}
	parsedTokenSource, tokenParseError := ParseTokenSource(tokenSourceValue)
	if tokenParseError != nil {
		return PurgeOptions{}, fmt.Errorf(tokenSourceParseErrorTemplateConstant, tokenParseError)
	}

	dryRunValue := configuration.Purge.DryRun
	if command.Flags().Changed(dryRunFlagNameConstant) {
		flagDryRunValue, dryRunFlagError := command.Flags().GetBool(dryRunFlagNameConstant)
		if dryRunFlagError != nil {
			return PurgeOptions{}, dryRunFlagError
		}
		dryRunValue = flagDryRunValue
	}

	purgeOptions := PurgeOptions{
		Owner:       ownerValue,
		PackageName: packageValue,
		OwnerType:   parsedOwnerType,
		TokenSource: parsedTokenSource,
		DryRun:      dryRunValue,
	}

	return purgeOptions, nil
}

func (builder *CommandBuilder) resolveLogger() *zap.Logger {
	if builder.LoggerProvider == nil {
		return zap.NewNop()
	}

	logger := builder.LoggerProvider()
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

func (builder *CommandBuilder) resolveConfiguration() Configuration {
	configuration := DefaultConfiguration()
	if builder.ConfigurationProvider != nil {
		configuration = builder.ConfigurationProvider()
	}

	if len(strings.TrimSpace(configuration.Purge.TokenSource)) == 0 {
		configuration.Purge.TokenSource = defaultTokenSourceValueConstant
	}

	return configuration
}

func (builder *CommandBuilder) resolvePurgeService(logger *zap.Logger) (PurgeExecutor, error) {
	if builder.ServiceResolver != nil {
		return builder.ServiceResolver.Resolve(logger)
	}

	defaultResolver := &DefaultPurgeServiceResolver{
		HTTPClient:        builder.HTTPClient,
		EnvironmentLookup: builder.EnvironmentLookup,
		FileReader:        builder.FileReader,
		TokenResolver:     builder.TokenResolver,
	}

	return defaultResolver.Resolve(logger)
}

func selectStringValue(flagValue string, configurationValue string) string {
	trimmedFlagValue := strings.TrimSpace(flagValue)
	if len(trimmedFlagValue) > 0 {
		return trimmedFlagValue
	}

	return strings.TrimSpace(configurationValue)
}
