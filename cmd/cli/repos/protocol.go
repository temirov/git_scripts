package repos

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/repos/dependencies"
	conversion "github.com/temirov/gix/internal/repos/protocol"
	"github.com/temirov/gix/internal/repos/shared"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

const (
	protocolUseConstant         = "repo-protocol-convert [root ...]"
	protocolShortDescription    = "Convert repository origin URLs between git/ssh/https"
	protocolLongDescription     = "repo-protocol-convert converts origin URLs to a desired protocol."
	protocolFromFlagName        = "from"
	protocolFromFlagDescription = "Current protocol to convert from (git, ssh, https)"
	protocolToFlagName          = "to"
	protocolToFlagDescription   = "Target protocol to convert to (git, ssh, https)"
	protocolErrorMissingPair    = "specify both --from and --to"
	protocolErrorSamePair       = "--from and --to must differ"
	protocolErrorInvalidValue   = "invalid protocol value: %s"
)

// ProtocolCommandBuilder assembles the repo-protocol-convert command.
type ProtocolCommandBuilder struct {
	LoggerProvider               LoggerProvider
	Discoverer                   shared.RepositoryDiscoverer
	GitExecutor                  shared.GitExecutor
	GitManager                   shared.GitRepositoryManager
	GitHubResolver               shared.GitHubMetadataResolver
	PrompterFactory              PrompterFactory
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() ProtocolConfiguration
}

// Build constructs the repo-protocol-convert command.
func (builder *ProtocolCommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   protocolUseConstant,
		Short: protocolShortDescription,
		Long:  protocolLongDescription,
		RunE:  builder.run,
	}

	command.Flags().String(protocolFromFlagName, "", protocolFromFlagDescription)
	command.Flags().String(protocolToFlagName, "", protocolToFlagDescription)

	return command, nil
}

func (builder *ProtocolCommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()
	executionFlags, executionFlagsAvailable := flagutils.ResolveExecutionFlags(command)

	dryRun := configuration.DryRun
	if executionFlagsAvailable && executionFlags.DryRunSet {
		dryRun = executionFlags.DryRun
	}

	assumeYes := configuration.AssumeYes
	if executionFlagsAvailable && executionFlags.AssumeYesSet {
		assumeYes = executionFlags.AssumeYes
	}

	fromValue := configuration.FromProtocol
	if command != nil && command.Flags().Changed(protocolFromFlagName) {
		fromValue, _ = command.Flags().GetString(protocolFromFlagName)
	}

	toValue := configuration.ToProtocol
	if command != nil && command.Flags().Changed(protocolToFlagName) {
		toValue, _ = command.Flags().GetString(protocolToFlagName)
	}

	if len(strings.TrimSpace(fromValue)) == 0 || len(strings.TrimSpace(toValue)) == 0 {
		if helpError := displayCommandHelp(command); helpError != nil {
			return helpError
		}
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

	roots, rootsError := requireRepositoryRoots(command, arguments, configuration.RepositoryRoots)
	if rootsError != nil {
		return rootsError
	}

	logger := resolveLogger(builder.LoggerProvider)
	humanReadableLogging := false
	if builder.HumanReadableLoggingProvider != nil {
		humanReadableLogging = builder.HumanReadableLoggingProvider()
	}
	gitExecutor, executorError := dependencies.ResolveGitExecutor(builder.GitExecutor, logger, humanReadableLogging)
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

	inspections, inspectionError := service.DiscoverInspections(command.Context(), roots, false, audit.InspectionDepthMinimal)
	if inspectionError != nil {
		return inspectionError
	}

	trackingPrompter := newCascadingConfirmationPrompter(prompter, assumeYes)
	protocolDependencies := conversion.Dependencies{
		GitManager: gitManager,
		Prompter:   trackingPrompter,
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
			AssumeYes:                trackingPrompter.AssumeYes(),
		}

		conversion.Execute(command.Context(), protocolDependencies, conversionOptions)
	}

	return nil
}

func (builder *ProtocolCommandBuilder) resolveConfiguration() ProtocolConfiguration {
	if builder.ConfigurationProvider == nil {
		defaults := DefaultToolsConfiguration()
		return defaults.Protocol
	}

	provided := builder.ConfigurationProvider()
	return provided.sanitize()
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
