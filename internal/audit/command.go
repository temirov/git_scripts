package audit

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/repos/dependencies"
	"github.com/temirov/gix/internal/utils"
)

// LoggerProvider supplies a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the audit cobra command with configurable dependencies.
type CommandBuilder struct {
	LoggerProvider               LoggerProvider
	Discoverer                   RepositoryDiscoverer
	GitExecutor                  GitExecutor
	GitManager                   GitRepositoryManager
	GitHubResolver               GitHubMetadataResolver
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() CommandConfiguration
}

// Build constructs the cobra command for repository audit workflows.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandUseConstant,
		Short: commandShortDescription,
		Long:  commandLongDescription,
		Args:  cobra.NoArgs,
		RunE:  builder.run,
	}

	command.Flags().StringSlice(flagRootNameConstant, nil, flagRootDescriptionConstant)
	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	options, optionsError := builder.parseOptions(command)
	if optionsError != nil {
		return optionsError
	}

	logger := builder.resolveLogger()
	gitExecutor, executorError := builder.resolveGitExecutor(logger)
	if executorError != nil {
		return executorError
	}

	gitManager, managerError := builder.resolveGitManager(gitExecutor)
	if managerError != nil {
		return managerError
	}

	githubClient, githubError := builder.resolveGitHubClient(gitExecutor)
	if githubError != nil {
		return githubError
	}

	discoverer := dependencies.ResolveRepositoryDiscoverer(builder.Discoverer)

	service := NewService(discoverer, gitManager, gitExecutor, githubClient, command.OutOrStdout(), command.ErrOrStderr())
	return service.Run(command.Context(), options)
}

func (builder *CommandBuilder) parseOptions(command *cobra.Command) (CommandOptions, error) {
	configuration := builder.resolveConfiguration()

	debugMode := configuration.Debug
	if command != nil {
		contextAccessor := utils.NewCommandContextAccessor()
		if logLevel, available := contextAccessor.LogLevel(command.Context()); available {
			if strings.EqualFold(logLevel, string(utils.LogLevelDebug)) {
				debugMode = true
			}
		}
	}

	roots := append([]string{}, configuration.Roots...)
	if command != nil && command.Flags().Changed(flagRootNameConstant) {
		flagRoots, _ := command.Flags().GetStringSlice(flagRootNameConstant)
		roots = auditConfigurationRepositoryPathSanitizer.Sanitize(flagRoots)
	}

	if len(roots) == 0 {
		if command != nil {
			_ = command.Help()
		}
		return CommandOptions{}, errors.New(missingRootsErrorMessageConstant)
	}

	options := CommandOptions{
		Roots:           roots,
		DebugOutput:     debugMode,
		InspectionDepth: InspectionDepthFull,
	}

	return options, nil
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

func (builder *CommandBuilder) resolveGitExecutor(logger *zap.Logger) (GitExecutor, error) {
	humanReadableLogging := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadableLogging = builder.HumanReadableLoggingProvider()
	}
	return dependencies.ResolveGitExecutor(builder.GitExecutor, logger, humanReadableLogging)
}

func (builder *CommandBuilder) resolveGitManager(executor GitExecutor) (GitRepositoryManager, error) {
	return dependencies.ResolveGitRepositoryManager(builder.GitManager, executor)
}

func (builder *CommandBuilder) resolveGitHubClient(executor GitExecutor) (GitHubMetadataResolver, error) {
	return dependencies.ResolveGitHubResolver(builder.GitHubResolver, executor)
}

func (builder *CommandBuilder) resolveConfiguration() CommandConfiguration {
	if builder.ConfigurationProvider == nil {
		return DefaultCommandConfiguration()
	}
	provided := builder.ConfigurationProvider()
	return provided.Sanitize()
}
