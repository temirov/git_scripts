package repos

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/temirov/git_scripts/internal/audit"
	"github.com/temirov/git_scripts/internal/repos/dependencies"
	"github.com/temirov/git_scripts/internal/repos/remotes"
	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	remotesUseConstant          = "repo-remote-update [root ...]"
	remotesShortDescription     = "Update origin URLs to match canonical GitHub repositories"
	remotesLongDescription      = "repo-remote-update adjusts origin remotes to point to canonical GitHub repositories."
	remotesDryRunFlagName       = "dry-run"
	remotesDryRunDescription    = "Preview remote updates without making changes"
	remotesAssumeYesFlagName    = "yes"
	remotesAssumeYesDescription = "Automatically confirm remote updates"
	remotesAssumeYesShorthand   = "y"
)

// RemotesCommandBuilder assembles the repo-remote-update command.
type RemotesCommandBuilder struct {
	LoggerProvider               LoggerProvider
	Discoverer                   shared.RepositoryDiscoverer
	GitExecutor                  shared.GitExecutor
	GitManager                   shared.GitRepositoryManager
	GitHubResolver               shared.GitHubMetadataResolver
	PrompterFactory              PrompterFactory
	HumanReadableLoggingProvider func() bool
	ConfigurationProvider        func() RemotesConfiguration
}

// Build constructs the repo-remote-update command.
func (builder *RemotesCommandBuilder) Build() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:   remotesUseConstant,
		Short: remotesShortDescription,
		Long:  remotesLongDescription,
		RunE:  builder.run,
	}

	command.Flags().Bool(remotesDryRunFlagName, false, remotesDryRunDescription)
	command.Flags().BoolP(remotesAssumeYesFlagName, remotesAssumeYesShorthand, false, remotesAssumeYesDescription)

	return command, nil
}

func (builder *RemotesCommandBuilder) run(command *cobra.Command, arguments []string) error {
	configuration := builder.resolveConfiguration()

	dryRun := configuration.DryRun
	if command != nil && command.Flags().Changed(remotesDryRunFlagName) {
		dryRun, _ = command.Flags().GetBool(remotesDryRunFlagName)
	}

	assumeYes := configuration.AssumeYes
	if command != nil && command.Flags().Changed(remotesAssumeYesFlagName) {
		assumeYes, _ = command.Flags().GetBool(remotesAssumeYesFlagName)
	}

	roots := determineRepositoryRoots(arguments, configuration.RepositoryRoots)

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

	inspections, inspectionError := service.DiscoverInspections(command.Context(), roots, false)
	if inspectionError != nil {
		return inspectionError
	}

	remotesDependencies := remotes.Dependencies{
		GitManager: gitManager,
		Prompter:   prompter,
		Output:     command.OutOrStdout(),
	}

	for _, inspection := range inspections {
		if len(strings.TrimSpace(inspection.CanonicalOwnerRepo)) == 0 && len(strings.TrimSpace(inspection.OriginOwnerRepo)) == 0 {
			continue
		}

		remotesOptions := remotes.Options{
			RepositoryPath:           inspection.Path,
			CurrentOriginURL:         inspection.OriginURL,
			OriginOwnerRepository:    inspection.OriginOwnerRepo,
			CanonicalOwnerRepository: inspection.CanonicalOwnerRepo,
			RemoteProtocol:           shared.RemoteProtocol(inspection.RemoteProtocol),
			DryRun:                   dryRun,
			AssumeYes:                assumeYes,
		}

		remotes.Execute(command.Context(), remotesDependencies, remotesOptions)
	}

	return nil
}

func (builder *RemotesCommandBuilder) resolveConfiguration() RemotesConfiguration {
	if builder.ConfigurationProvider == nil {
		defaults := DefaultToolsConfiguration()
		return defaults.Remotes
	}

	provided := builder.ConfigurationProvider()
	return provided.sanitize()
}
