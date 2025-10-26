package release

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/releases"
	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	commandUseName          = "release"
	commandUsageTemplate    = commandUseName + " <tag>"
	commandExampleTemplate  = "gix repo release v1.2.3 --roots ~/Development"
	commandShortDescription = "Create and push an annotated release tag"
	commandLongDescription  = "release annotates the provided tag (default message 'Release <tag>') and pushes it to the configured remote for each repository root. Provide the tag as the first argument before any optional repository roots or flags."
	messageFlagName         = "message"
	messageFlagUsage        = "Override the tag message"
	missingTagErrorMessage  = "tag name is required"
	releaseSuccessTemplate  = "RELEASED: %s -> %s"
)

// CommandBuilder assembles the release command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	GitExecutor                  shared.GitExecutor
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the repo release command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:     commandUsageTemplate,
		Short:   commandShortDescription,
		Long:    commandLongDescription,
		Example: commandExampleTemplate,
		Args:    cobra.ArbitraryArgs,
		RunE:    builder.run,
	}

	command.Flags().String(messageFlagName, "", messageFlagUsage)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()

	executionFlags, executionFlagsAvailable := flagutils.ResolveExecutionFlags(command)
	dryRun := false
	if executionFlagsAvailable && executionFlags.DryRunSet {
		dryRun = executionFlags.DryRun
	}

	if len(arguments) == 0 {
		if command != nil {
			_ = command.Help()
		}
		return errors.New(missingTagErrorMessage)
	}

	tagName := strings.TrimSpace(arguments[0])
	if len(tagName) == 0 {
		if command != nil {
			_ = command.Help()
		}
		return errors.New(missingTagErrorMessage)
	}
	additionalArgs := arguments[1:]

	messageValue := configuration.Message
	if command != nil {
		if flagValue, flagError := command.Flags().GetString(messageFlagName); flagError == nil && command.Flags().Changed(messageFlagName) {
			messageValue = strings.TrimSpace(flagValue)
		}
	}

	remoteName := configuration.RemoteName
	if executionFlagsAvailable && executionFlags.RemoteSet {
		override := strings.TrimSpace(executionFlags.Remote)
		if len(override) > 0 {
			remoteName = override
		}
	}

	repositoryRoots, rootsError := rootutils.Resolve(command, additionalArgs, configuration.RepositoryRoots)
	if rootsError != nil {
		return rootsError
	}

	logger := resolveLogger(builder.LoggerProvider)
	humanReadable := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadable = builder.HumanReadableLoggingProvider()
	}

	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, humanReadable)
	if executorError != nil {
		return executorError
	}

	service, serviceError := releases.NewService(releases.ServiceDependencies{GitExecutor: gitExecutor})
	if serviceError != nil {
		return serviceError
	}

	for _, repository := range repositoryRoots {
		result, releaseError := service.Release(command.Context(), releases.Options{
			RepositoryPath: repository,
			TagName:        tagName,
			Message:        messageValue,
			RemoteName:     remoteName,
			DryRun:         dryRun,
		})
		if releaseError != nil {
			return releaseError
		}

		fmt.Fprintln(command.OutOrStdout(), fmt.Sprintf(releaseSuccessTemplate, result.RepositoryPath, result.TagName))
	}

	return nil
}

func (builder *CommandBuilder) resolveConfiguration() CommandConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultCommandConfiguration()
	}
	return builder.ConfigurationProvider().Sanitize()
}
