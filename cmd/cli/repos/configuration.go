package repos

import "strings"

const (
	remotesConfigurationKeyConstant      = "remotes"
	protocolConfigurationKeyConstant     = "protocol"
	configurationDryRunKeyConstant       = "dry_run"
	configurationAssumeYesKeyConstant    = "assume_yes"
	configurationRootsKeyConstant        = "roots"
	protocolConfigurationFromKeyConstant = "from"
	protocolConfigurationToKeyConstant   = "to"
)

// ToolsConfiguration captures repository command configuration sections.
type ToolsConfiguration struct {
	Remotes  RemotesConfiguration  `mapstructure:"remotes"`
	Protocol ProtocolConfiguration `mapstructure:"protocol"`
}

// RemotesConfiguration describes configuration values for repo-remote-update.
type RemotesConfiguration struct {
	DryRun          bool     `mapstructure:"dry_run"`
	AssumeYes       bool     `mapstructure:"assume_yes"`
	RepositoryRoots []string `mapstructure:"roots"`
}

// ProtocolConfiguration describes configuration values for protocol-convert.
type ProtocolConfiguration struct {
	DryRun          bool     `mapstructure:"dry_run"`
	AssumeYes       bool     `mapstructure:"assume_yes"`
	RepositoryRoots []string `mapstructure:"roots"`
	FromProtocol    string   `mapstructure:"from"`
	ToProtocol      string   `mapstructure:"to"`
}

// DefaultToolsConfiguration returns baseline configuration values for repository commands.
func DefaultToolsConfiguration() ToolsConfiguration {
	return ToolsConfiguration{
		Remotes: RemotesConfiguration{
			DryRun:          false,
			AssumeYes:       false,
			RepositoryRoots: []string{defaultRepositoryRootConstant},
		},
		Protocol: ProtocolConfiguration{
			DryRun:          false,
			AssumeYes:       false,
			RepositoryRoots: []string{defaultRepositoryRootConstant},
			FromProtocol:    "",
			ToProtocol:      "",
		},
	}
}

// DefaultConfigurationValues produces Viper defaults for repository commands.
func DefaultConfigurationValues(rootKey string) map[string]any {
	defaults := DefaultToolsConfiguration()
	return map[string]any{
		rootKey + "." + remotesConfigurationKeyConstant + "." + configurationDryRunKeyConstant:        defaults.Remotes.DryRun,
		rootKey + "." + remotesConfigurationKeyConstant + "." + configurationAssumeYesKeyConstant:     defaults.Remotes.AssumeYes,
		rootKey + "." + remotesConfigurationKeyConstant + "." + configurationRootsKeyConstant:         defaults.Remotes.RepositoryRoots,
		rootKey + "." + protocolConfigurationKeyConstant + "." + configurationDryRunKeyConstant:       defaults.Protocol.DryRun,
		rootKey + "." + protocolConfigurationKeyConstant + "." + configurationAssumeYesKeyConstant:    defaults.Protocol.AssumeYes,
		rootKey + "." + protocolConfigurationKeyConstant + "." + configurationRootsKeyConstant:        defaults.Protocol.RepositoryRoots,
		rootKey + "." + protocolConfigurationKeyConstant + "." + protocolConfigurationFromKeyConstant: defaults.Protocol.FromProtocol,
		rootKey + "." + protocolConfigurationKeyConstant + "." + protocolConfigurationToKeyConstant:   defaults.Protocol.ToProtocol,
	}
}

// sanitize normalizes repository configuration values.
func (configuration RemotesConfiguration) sanitize() RemotesConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = trimRoots(configuration.RepositoryRoots)
	if len(sanitized.RepositoryRoots) == 0 {
		sanitized.RepositoryRoots = append([]string{}, defaultRepositoryRootConstant)
	}
	return sanitized
}

// sanitize normalizes protocol configuration values.
func (configuration ProtocolConfiguration) sanitize() ProtocolConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = trimRoots(configuration.RepositoryRoots)
	if len(sanitized.RepositoryRoots) == 0 {
		sanitized.RepositoryRoots = append([]string{}, defaultRepositoryRootConstant)
	}
	sanitized.FromProtocol = strings.TrimSpace(configuration.FromProtocol)
	sanitized.ToProtocol = strings.TrimSpace(configuration.ToProtocol)
	return sanitized
}
