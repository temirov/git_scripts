package repos

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/git_scripts/internal/audit"
	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/repos/dependencies"
	conversion "github.com/temirov/git_scripts/internal/repos/protocol"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	protocolUseConstant          = "convert-remote-protocol [root ...]"
	protocolShortDescription     = "Convert repository origin remotes between protocols"
	protocolLongDescription      = "convert-remote-protocol updates origin remotes to use the requested Git protocol."
	protocolDryRunFlagName       = "dry-run"
	protocolDryRunDescription    = "Preview protocol conversions without making changes"
	protocolAssumeYesFlagName    = "yes"
	protocolAssumeYesShorthand   = "y"
	protocolAssumeYesDescription = "Automatically confirm protocol conversions"
	protocolFromFlagName         = "from"
	protocolFromDescription      = "Current protocol to convert from (git|ssh|https)"
	protocolToFlagName           = "to"
	protocolToDescription        = "Protocol to convert to (git|ssh|https)"
	protocolErrorMissingPair     = "specify both --from and --to"
	protocolErrorSamePair        = "--from and --to cannot be the same protocol"
	protocolErrorInvalidValue    = "unsupported protocol value: %s"
)

// ProtocolCommandBuilder assembles the convert-remote-protocol command.
type ProtocolCommandBuilder struct {
	LoggerProvider        LoggerProvider
	Discoverer            shared.RepositoryDiscoverer
	GitExecutor           shared.GitExecutor
	GitManager            shared.GitRepositoryManager
	GitHubResolver        shared.GitHubMetadataResolver
	PrompterFactory       PrompterFactory
	CommandEventsObserver execshell.CommandEventObserver
}

// Build constructs the convert-remote-protocol command.
func (builder *ProtocolCommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   protocolUseConstant,
		Short: protocolShortDescription,
		Long:  protocolLongDescription,
		RunE:  builder.run,
	}

	command.Flags().Bool(protocolDryRunFlagName, false, protocolDryRunDescription)
	command.Flags().BoolP(protocolAssumeYesFlagName, protocolAssumeYesShorthand, false, protocolAssumeYesDescription)
	command.Flags().String(protocolFromFlagName, "", protocolFromDescription)
	command.Flags().String(protocolToFlagName, "", protocolToDescription)

	return command, nil
}

func (builder *ProtocolCommandBuilder) run(command *cobra.Command, arguments []string) error {
	dryRun, _ := command.Flags().GetBool(protocolDryRunFlagName)
	assumeYes, _ := command.Flags().GetBool(protocolAssumeYesFlagName)
	fromValue, _ := command.Flags().GetString(protocolFromFlagName)
	toValue, _ := command.Flags().GetString(protocolToFlagName)

	if len(strings.TrimSpace(fromValue)) == 0 || len(strings.TrimSpace(toValue)) == 0 {
		return errors.New(protocolErrorMissingPair)
	}

	fromProtocol, fromError := parseProtocolValue(fromValue)
	if fromError != nil {
		return fromError
	}

	toProtocol, toError := parseProtocolValue(toValue)
	if toError != nil {
		return toError
	}

	if fromProtocol == toProtocol {
		return errors.New(protocolErrorSamePair)
	}

	roots := determineRepositoryRoots(arguments)

	logger := resolveLogger(builder.LoggerProvider)
	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, builder.CommandEventsObserver)
	if executorError != nil {
		return executorError
	}

	gitManager, managerError := dependencies.ResolveGitRepositoryManager(builder.GitManager, gitExecutor)
	if managerError != nil {
		return managerError
	}

	githubResolver, resolverError := dependencies.ResolveGitHubResolver(builder.GitHubResolver, gitExecutor)
	if resolverError != nil {
		return resolverError
	}

	repositoryDiscoverer := dependencies.ResolveRepositoryDiscoverer(builder.Discoverer)
	prompter := resolvePrompter(builder.PrompterFactory, command)

	service := audit.NewService(repositoryDiscoverer, gitManager, gitExecutor, githubResolver, command.OutOrStdout(), command.ErrOrStderr())

	inspections, inspectionError := service.DiscoverInspections(command.Context(), roots, false)
	if inspectionError != nil {
		return inspectionError
	}

	protocolDependencies := conversion.Dependencies{
		GitManager: gitManager,
		Prompter:   prompter,
		Output:     command.OutOrStdout(),
		Errors:     command.ErrOrStderr(),
	}

	for _, inspection := range inspections {
		if shared.RemoteProtocol(inspection.RemoteProtocol) != fromProtocol {
			continue
		}

		conversionOptions := conversion.Options{
			RepositoryPath:           inspection.Path,
			OriginOwnerRepository:    inspection.OriginOwnerRepo,
			CanonicalOwnerRepository: inspection.CanonicalOwnerRepo,
			CurrentProtocol:          fromProtocol,
			TargetProtocol:           toProtocol,
			DryRun:                   dryRun,
			AssumeYes:                assumeYes,
		}

		conversion.Execute(command.Context(), protocolDependencies, conversionOptions)
	}

	return nil
}

func parseProtocolValue(value string) (shared.RemoteProtocol, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case string(shared.RemoteProtocolGit):
		return shared.RemoteProtocolGit, nil
	case string(shared.RemoteProtocolSSH):
		return shared.RemoteProtocolSSH, nil
	case string(shared.RemoteProtocolHTTPS):
		return shared.RemoteProtocolHTTPS, nil
	default:
		return "", fmt.Errorf(protocolErrorInvalidValue, value)
	}
}
