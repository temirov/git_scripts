package audit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
)

// LoggerProvider supplies a zap logger for command execution.
type LoggerProvider func() *zap.Logger

// PrompterFactory creates confirmation prompters for commands.
type PrompterFactory func(command *cobra.Command) ConfirmationPrompter

// CommandBuilder assembles the audit cobra command with configurable dependencies.
type CommandBuilder struct {
	LoggerProvider  LoggerProvider
	Discoverer      RepositoryDiscoverer
	GitExecutor     GitExecutor
	GitManager      GitRepositoryManager
	GitHubResolver  GitHubMetadataResolver
	FileSystem      FileSystem
	PrompterFactory PrompterFactory
	Clock           Clock
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
	command.Flags().Bool(flagRenameName, false, flagRenameDescription)
	command.Flags().Bool(flagUpdateRemoteName, false, flagUpdateRemoteDescription)
	command.Flags().String(flagProtocolFromName, "", flagProtocolFromDescription)
	command.Flags().String(flagProtocolToName, "", flagProtocolToDescription)
	command.Flags().Bool(flagDryRunName, false, flagDryRunDescription)
	command.Flags().BoolP(flagAssumeYesName, flagAssumeYesShorthand, false, flagAssumeYesDescription)
	command.Flags().Bool(flagRequireCleanName, false, flagRequireCleanDescription)
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

	discoverer := builder.Discoverer
	if discoverer == nil {
		discoverer = NewFilesystemRepositoryDiscoverer()
	}

	fileSystem := builder.FileSystem
	if fileSystem == nil {
		fileSystem = OSFileSystem{}
	}

	prompter := builder.resolvePrompter(command)

	service := NewService(discoverer, gitManager, gitExecutor, githubClient, fileSystem, prompter, command.OutOrStdout(), command.ErrOrStderr(), builder.Clock)
	return service.Run(command.Context(), options)
}

func (builder *CommandBuilder) parseOptions(command *cobra.Command, arguments []string) (CommandOptions, error) {
	auditFlag, _ := command.Flags().GetBool(flagAuditName)
	renameFlag, _ := command.Flags().GetBool(flagRenameName)
	updateRemoteFlag, _ := command.Flags().GetBool(flagUpdateRemoteName)
	protocolFromValue, _ := command.Flags().GetString(flagProtocolFromName)
	protocolToValue, _ := command.Flags().GetString(flagProtocolToName)
	dryRunFlag, _ := command.Flags().GetBool(flagDryRunName)
	assumeYesFlag, _ := command.Flags().GetBool(flagAssumeYesName)
	requireCleanFlag, _ := command.Flags().GetBool(flagRequireCleanName)
	debugFlag, _ := command.Flags().GetBool(flagDebugName)

	if !auditFlag && !renameFlag && !updateRemoteFlag && len(strings.TrimSpace(protocolFromValue)) == 0 && len(strings.TrimSpace(protocolToValue)) == 0 {
		return CommandOptions{}, errors.New(errorMissingOperation)
	}

	protocolFrom, parseFromError := parseProtocol(protocolFromValue)
	if parseFromError != nil {
		return CommandOptions{}, parseFromError
	}

	protocolTo, parseToError := parseProtocol(protocolToValue)
	if parseToError != nil {
		return CommandOptions{}, parseToError
	}

	if (len(strings.TrimSpace(protocolFromValue)) > 0) != (len(strings.TrimSpace(protocolToValue)) > 0) {
		return CommandOptions{}, errors.New(errorProtocolPairIncomplete)
	}

	if len(strings.TrimSpace(protocolFromValue)) > 0 && protocolFrom == protocolTo {
		return CommandOptions{}, errors.New(errorProtocolPairSame)
	}

	roots := append([]string{}, arguments...)

	options := CommandOptions{
		Roots:                roots,
		AuditReport:          auditFlag,
		RenameRepositories:   renameFlag,
		UpdateRemotes:        updateRemoteFlag,
		ProtocolFrom:         protocolFrom,
		ProtocolTo:           protocolTo,
		DryRun:               dryRunFlag,
		AssumeYes:            assumeYesFlag,
		RequireCleanWorktree: requireCleanFlag,
		DebugOutput:          debugFlag,
		Clock:                builder.Clock,
	}

	if options.Clock == nil {
		options.Clock = SystemClock{}
	}

	return options, nil
}

func parseProtocol(value string) (RemoteProtocolType, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "":
		return RemoteProtocolType(""), nil
	case string(RemoteProtocolGit):
		return RemoteProtocolGit, nil
	case string(RemoteProtocolSSH):
		return RemoteProtocolSSH, nil
	case string(RemoteProtocolHTTPS):
		return RemoteProtocolHTTPS, nil
	default:
		return RemoteProtocolType(""), fmt.Errorf(errorInvalidProtocolValue)
	}
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
	if builder.GitExecutor != nil {
		return builder.GitExecutor, nil
	}
	runner := execshell.NewOSCommandRunner()
	executor, creationError := execshell.NewShellExecutor(logger, runner)
	if creationError != nil {
		return nil, creationError
	}
	return executor, nil
}

func (builder *CommandBuilder) resolveGitManager(executor GitExecutor) (GitRepositoryManager, error) {
	if builder.GitManager != nil {
		return builder.GitManager, nil
	}
	manager, creationError := gitrepo.NewRepositoryManager(executor)
	if creationError != nil {
		return nil, creationError
	}
	return manager, nil
}

func (builder *CommandBuilder) resolveGitHubClient(executor GitExecutor) (GitHubMetadataResolver, error) {
	if builder.GitHubResolver != nil {
		return builder.GitHubResolver, nil
	}
	client, creationError := githubcli.NewClient(executor)
	if creationError != nil {
		return nil, creationError
	}
	return client, nil
}

func (builder *CommandBuilder) resolvePrompter(command *cobra.Command) ConfirmationPrompter {
	if builder.PrompterFactory != nil {
		return builder.PrompterFactory(command)
	}
	return NewIOConfirmationPrompter(command.InOrStdin(), command.OutOrStdout())
}
