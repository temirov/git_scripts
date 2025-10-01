package branches

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/repos/discovery"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	commandUseConstant                          = "repo-prs-purge"
	commandShortDescriptionConstant             = "Remove remote and local branches for closed pull requests"
	commandLongDescriptionConstant              = "repo-prs-purge removes remote and local Git branches whose pull requests are already closed."
	flagRemoteDescriptionConstant               = "Name of the remote containing pull request branches"
	flagLimitNameConstant                       = "limit"
	flagLimitDescriptionConstant                = "Maximum number of closed pull requests to examine"
	invalidRemoteNameErrorMessageConstant       = "remote name must not be empty or whitespace"
	invalidPullRequestLimitErrorMessageConstant = "limit must be greater than zero"
	repositoryDiscoveryErrorTemplateConstant    = "repository discovery failed: %w"
	logMessageRepositoryDiscoveryFailedConstant = "Repository discovery failed"
	logMessageRepositoryCleanupFailedConstant   = "Repository cleanup failed"
	logFieldRepositoryRootsConstant             = "roots"
	logFieldRepositoryPathConstant              = "repository"
)

// RepositoryDiscoverer locates Git repositories beneath the provided roots.
type RepositoryDiscoverer interface {
	DiscoverRepositories(roots []string) ([]string, error)
}

// LoggerProvider supplies a zap logger instance.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the repo-prs-purge Cobra command.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	Executor                     CommandExecutor
	WorkingDirectory             string
	RepositoryDiscoverer         RepositoryDiscoverer
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the repo-prs-purge command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandUseConstant,
		Short: commandShortDescriptionConstant,
		Long:  commandLongDescriptionConstant,
		RunE:  builder.run,
	}

	command.Flags().Int(flagLimitNameConstant, defaultPullRequestLimitConstant, flagLimitDescriptionConstant)
	flagutils.EnsureRemoteFlag(command, defaultRemoteNameConstant, flagRemoteDescriptionConstant)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	options, optionsError := builder.parseOptions(command, arguments)
	if optionsError != nil {
		return optionsError
	}

	logger := builder.resolveLogger()
	executor, executorError := builder.resolveExecutor(logger)
	if executorError != nil {
		return executorError
	}

	repositoryDiscoverer := builder.resolveRepositoryDiscoverer()
	repositories, discoveryError := repositoryDiscoverer.DiscoverRepositories(options.RepositoryRoots)
	if discoveryError != nil {
		logger.Error(logMessageRepositoryDiscoveryFailedConstant,
			zap.Strings(logFieldRepositoryRootsConstant, options.RepositoryRoots),
			zap.Error(discoveryError),
		)
		return fmt.Errorf(repositoryDiscoveryErrorTemplateConstant, discoveryError)
	}

	service, serviceError := NewService(logger, executor)
	if serviceError != nil {
		return serviceError
	}

	for repositoryIndex := range repositories {
		repositoryPath := repositories[repositoryIndex]
		repositoryOptions := options.CleanupOptions
		repositoryOptions.WorkingDirectory = repositoryPath

		cleanupError := service.Cleanup(command.Context(), repositoryOptions)
		if cleanupError != nil {
			logger.Warn(logMessageRepositoryCleanupFailedConstant,
				zap.String(logFieldRepositoryPathConstant, repositoryPath),
				zap.Error(cleanupError),
			)

			if errors.Is(cleanupError, context.Canceled) || errors.Is(cleanupError, context.DeadlineExceeded) {
				return cleanupError
			}
		}
	}

	return nil
}

type commandOptions struct {
	CleanupOptions  CleanupOptions
	RepositoryRoots []string
}

func (builder *CommandBuilder) parseOptions(command *cobra.Command, arguments []string) (commandOptions, error) {
	configuration := builder.resolveConfiguration()
	executionFlags, executionFlagsAvailable := flagutils.ResolveExecutionFlags(command)

	trimmedRemoteName := strings.TrimSpace(configuration.RemoteName)
	if executionFlagsAvailable && executionFlags.RemoteSet {
		overrideRemote := strings.TrimSpace(executionFlags.Remote)
		if len(overrideRemote) == 0 {
			if command != nil {
				_ = command.Help()
			}
			return commandOptions{}, errors.New(invalidRemoteNameErrorMessageConstant)
		}
		trimmedRemoteName = overrideRemote
	}
	if len(trimmedRemoteName) == 0 && builder.ConfigurationProvider == nil {
		trimmedRemoteName = defaultRemoteNameConstant
	}
	if len(trimmedRemoteName) == 0 {
		if command != nil {
			_ = command.Help()
		}
		return commandOptions{}, errors.New(invalidRemoteNameErrorMessageConstant)
	}

	limitValue := configuration.PullRequestLimit
	if command != nil {
		flagLimitValue, _ := command.Flags().GetInt(flagLimitNameConstant)
		if command.Flags().Changed(flagLimitNameConstant) {
			limitValue = flagLimitValue
		} else if limitValue == 0 && builder.ConfigurationProvider == nil {
			limitValue = flagLimitValue
		}
	}
	if limitValue <= 0 {
		if command != nil {
			_ = command.Help()
		}
		return commandOptions{}, errors.New(invalidPullRequestLimitErrorMessageConstant)
	}

	dryRunValue := configuration.DryRun
	if executionFlagsAvailable && executionFlags.DryRunSet {
		dryRunValue = executionFlags.DryRun
	}

	cleanupOptions := CleanupOptions{
		RemoteName:       trimmedRemoteName,
		PullRequestLimit: limitValue,
		DryRun:           dryRunValue,
	}

	repositoryRoots, rootsError := rootutils.Resolve(command, arguments, configuration.RepositoryRoots)
	if rootsError != nil {
		return commandOptions{}, rootsError
	}

	return commandOptions{CleanupOptions: cleanupOptions, RepositoryRoots: repositoryRoots}, nil
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

func (builder *CommandBuilder) resolveExecutor(logger *zap.Logger) (CommandExecutor, error) {
	if builder.Executor != nil {
		return builder.Executor, nil
	}

	commandRunner := execshell.NewOSCommandRunner()
	humanReadableLogging := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadableLogging = builder.HumanReadableLoggingProvider()
	}
	shellExecutor, creationError := execshell.NewShellExecutor(logger, commandRunner, humanReadableLogging)
	if creationError != nil {
		return nil, creationError
	}

	return shellExecutor, nil
}

func (builder *CommandBuilder) resolveRepositoryDiscoverer() RepositoryDiscoverer {
	if builder.RepositoryDiscoverer != nil {
		return builder.RepositoryDiscoverer
	}

	return discovery.NewFilesystemRepositoryDiscoverer()
}

func (builder *CommandBuilder) resolveConfiguration() CommandConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultCommandConfiguration()
	}

	provided := builder.ConfigurationProvider()
	return provided.Sanitize()
}
