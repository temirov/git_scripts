package repos

import (
	"strings"

	rootutils "github.com/temirov/gix/internal/utils/roots"
)

// ToolsConfiguration captures repository command configuration sections.
type ToolsConfiguration struct {
	Remotes  RemotesConfiguration  `mapstructure:"remotes"`
	Protocol ProtocolConfiguration `mapstructure:"protocol"`
	Rename   RenameConfiguration   `mapstructure:"rename"`
	Remove   RemoveConfiguration   `mapstructure:"remove"`
}

// RemotesConfiguration describes configuration values for repo-remote-update.
type RemotesConfiguration struct {
	DryRun          bool     `mapstructure:"dry_run"`
	AssumeYes       bool     `mapstructure:"assume_yes"`
	Owner           string   `mapstructure:"owner"`
	RepositoryRoots []string `mapstructure:"roots"`
}

// ProtocolConfiguration describes configuration values for repo-protocol-convert.
type ProtocolConfiguration struct {
	DryRun          bool     `mapstructure:"dry_run"`
	AssumeYes       bool     `mapstructure:"assume_yes"`
	RepositoryRoots []string `mapstructure:"roots"`
	FromProtocol    string   `mapstructure:"from"`
	ToProtocol      string   `mapstructure:"to"`
}

// RenameConfiguration describes configuration values for repo-folders-rename.
type RenameConfiguration struct {
	DryRun               bool     `mapstructure:"dry_run"`
	AssumeYes            bool     `mapstructure:"assume_yes"`
	RequireCleanWorktree bool     `mapstructure:"require_clean"`
	RepositoryRoots      []string `mapstructure:"roots"`
	IncludeOwner         bool     `mapstructure:"include_owner"`
}

// RemoveConfiguration describes configuration values for repo history removal.
type RemoveConfiguration struct {
	DryRun          bool     `mapstructure:"dry_run"`
	AssumeYes       bool     `mapstructure:"assume_yes"`
	RepositoryRoots []string `mapstructure:"roots"`
	Remote          string   `mapstructure:"remote"`
	Push            bool     `mapstructure:"push"`
	Restore         bool     `mapstructure:"restore"`
	PushMissing     bool     `mapstructure:"push_missing"`
}

// DefaultToolsConfiguration returns baseline configuration values for repository commands.
func DefaultToolsConfiguration() ToolsConfiguration {
	return ToolsConfiguration{
		Remotes: RemotesConfiguration{
			DryRun:          false,
			AssumeYes:       false,
			Owner:           "",
			RepositoryRoots: nil,
		},
		Protocol: ProtocolConfiguration{
			DryRun:          false,
			AssumeYes:       false,
			RepositoryRoots: nil,
			FromProtocol:    "",
			ToProtocol:      "",
		},
		Rename: RenameConfiguration{
			DryRun:               false,
			AssumeYes:            false,
			RequireCleanWorktree: false,
			RepositoryRoots:      nil,
			IncludeOwner:         false,
		},
		Remove: RemoveConfiguration{
			DryRun:          false,
			AssumeYes:       false,
			RepositoryRoots: nil,
			Remote:          "",
			Push:            true,
			Restore:         true,
			PushMissing:     false,
		},
	}
}

// sanitize normalizes repository configuration values.
func (configuration RemotesConfiguration) sanitize() RemotesConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = rootutils.SanitizeConfigured(configuration.RepositoryRoots)
	sanitized.Owner = strings.TrimSpace(configuration.Owner)
	return sanitized
}

// sanitize normalizes protocol configuration values.
func (configuration ProtocolConfiguration) sanitize() ProtocolConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = rootutils.SanitizeConfigured(configuration.RepositoryRoots)
	sanitized.FromProtocol = strings.TrimSpace(configuration.FromProtocol)
	sanitized.ToProtocol = strings.TrimSpace(configuration.ToProtocol)
	return sanitized
}

// sanitize normalizes rename configuration values.
func (configuration RenameConfiguration) sanitize() RenameConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = rootutils.SanitizeConfigured(configuration.RepositoryRoots)
	return sanitized
}

// sanitize normalizes remove configuration values.
func (configuration RemoveConfiguration) sanitize() RemoveConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = rootutils.SanitizeConfigured(configuration.RepositoryRoots)
	sanitized.Remote = strings.TrimSpace(configuration.Remote)
	return sanitized
}

// Sanitize normalizes remove configuration values.
func (configuration RemoveConfiguration) Sanitize() RemoveConfiguration {
	return configuration.sanitize()
}
