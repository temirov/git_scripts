package audit

import (
	"errors"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/repos/dependencies"
)

// LoggerProvider supplies a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// CommandBuilder assembles the audit cobra command with configurable dependencies.
type CommandBuilder struct {
	LoggerProvider        LoggerProvider
	Discoverer            RepositoryDiscoverer
	GitExecutor           GitExecutor
	GitManager            GitRepositoryManager
	GitHubResolver        GitHubMetadataResolver
	CommandEventsObserver execshell.CommandEventObserver
}

// Build constructs the cobra command for repository audit workflows.
func (builder *CommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   commandNameConstant,
		Short: commandShortDescription,
		Long:  commandLongDescription,
		RunE:  builder.run,
	}

	command.Flags().Bool(flagAuditName, false, flagAuditDescription)
	command.Flags().Bool(flagDebugName, false, flagDebugDescription)

	return command, nil
}

func (builder *CommandBuilder) run(command *cobra.Command, arguments []string) error {
	options, optionsError := builder.parseOptions(command, arguments)
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

func (builder *CommandBuilder) parseOptions(command *cobra.Command, arguments []string) (CommandOptions, error) {
	auditFlag, _ := command.Flags().GetBool(flagAuditName)
	debugFlag, _ := command.Flags().GetBool(flagDebugName)

	if !auditFlag {
		if helpError := builder.displayCommandHelp(command); helpError != nil {
			return CommandOptions{}, helpError
		}
		return CommandOptions{}, errors.New(errorMissingOperation)
	}

	roots := append([]string{}, arguments...)

	options := CommandOptions{
		Roots:       roots,
		AuditReport: auditFlag,
		DebugOutput: debugFlag,
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
	return dependencies.ResolveGitExecutor(builder.GitExecutor, logger, builder.CommandEventsObserver)
}

func (builder *CommandBuilder) resolveGitManager(executor GitExecutor) (GitRepositoryManager, error) {
	return dependencies.ResolveGitRepositoryManager(builder.GitManager, executor)
}

func (builder *CommandBuilder) resolveGitHubClient(executor GitExecutor) (GitHubMetadataResolver, error) {
	return dependencies.ResolveGitHubResolver(builder.GitHubResolver, executor)
}

func (builder *CommandBuilder) displayCommandHelp(command *cobra.Command) error {
	if command == nil {
		return nil
	}
	return command.Help()
}
