package branches

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/repos/discovery"
)

const (
	commandUseConstant                          = "pr-cleanup [root ...]"
	commandShortDescriptionConstant             = "Remove remote and local branches for closed pull requests"
	commandLongDescriptionConstant              = "pr-cleanup removes remote and local Git branches whose pull requests are already closed."
	flagRemoteNameConstant                      = "remote"
	flagRemoteDescriptionConstant               = "Name of the remote containing pull request branches"
	flagLimitNameConstant                       = "limit"
	flagLimitDescriptionConstant                = "Maximum number of closed pull requests to examine"
	flagDryRunNameConstant                      = "dry-run"
	flagDryRunDescriptionConstant               = "Preview deletions without making changes"
	repositoryDiscoveryErrorTemplateConstant    = "repository discovery failed: %w"
	defaultRepositoryRootConstant               = "."
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

// CommandBuilder assembles the Cobra command for branch cleanup.
type CommandBuilder struct {
	LoggerProvider        LoggerProvider
	Executor              CommandExecutor
	WorkingDirectory      string
	RepositoryDiscoverer  RepositoryDiscoverer
	CommandEventsObserver execshell.CommandEventObserver
}

// Build constructs the pr-cleanup command.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandUseConstant,
		Short: commandShortDescriptionConstant,
		Long:  commandLongDescriptionConstant,
		RunE:  builder.run,
	}

	command.Flags().String(flagRemoteNameConstant, defaultRemoteNameConstant, flagRemoteDescriptionConstant)
	command.Flags().Int(flagLimitNameConstant, defaultPullRequestLimitConstant, flagLimitDescriptionConstant)
	command.Flags().Bool(flagDryRunNameConstant, false, flagDryRunDescriptionConstant)

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
	remoteNameValue, _ := command.Flags().GetString(flagRemoteNameConstant)
	trimmedRemoteName := strings.TrimSpace(remoteNameValue)
	if len(trimmedRemoteName) == 0 {
		trimmedRemoteName = defaultRemoteNameConstant
	}

	limitValue, _ := command.Flags().GetInt(flagLimitNameConstant)
	if limitValue == 0 {
		limitValue = defaultPullRequestLimitConstant
	}

	dryRunValue, _ := command.Flags().GetBool(flagDryRunNameConstant)

	cleanupOptions := CleanupOptions{
		RemoteName:       trimmedRemoteName,
		PullRequestLimit: limitValue,
		DryRun:           dryRunValue,
	}

	repositoryRoots := builder.determineRepositoryRoots(arguments)

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
	shellExecutor, creationError := execshell.NewShellExecutor(logger, commandRunner, builder.CommandEventsObserver)
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

func (builder *CommandBuilder) determineRepositoryRoots(arguments []string) []string {
	repositoryRoots := make([]string, 0, len(arguments))
	for argumentIndex := range arguments {
		trimmedRoot := strings.TrimSpace(arguments[argumentIndex])
		if len(trimmedRoot) == 0 {
			continue
		}
		repositoryRoots = append(repositoryRoots, trimmedRoot)
	}

	if len(repositoryRoots) > 0 {
		return repositoryRoots
	}

	trimmedWorkingDirectory := strings.TrimSpace(builder.WorkingDirectory)
	if len(trimmedWorkingDirectory) > 0 {
		return []string{trimmedWorkingDirectory}
	}

	return []string{defaultRepositoryRootConstant}
}
