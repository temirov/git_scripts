package migrate

import (
	"strings"

	pathutils "github.com/temirov/gix/internal/utils/path"
)

var migrateConfigurationRepositoryPathSanitizer = pathutils.NewRepositoryPathSanitizer()

// CommandConfiguration captures persisted configuration for branch migration.
type CommandConfiguration struct {
	EnableDebugLogging bool     `mapstructure:"debug"`
	RepositoryRoots    []string `mapstructure:"roots"`
	SourceBranch       string   `mapstructure:"from"`
	TargetBranch       string   `mapstructure:"to"`
}

// DefaultCommandConfiguration returns baseline configuration values for branch migration.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		EnableDebugLogging: false,
		RepositoryRoots:    nil,
		SourceBranch:       string(BranchMain),
		TargetBranch:       string(BranchMaster),
	}
}

// Sanitize trims configured values and removes empty entries.
func (configuration CommandConfiguration) Sanitize() CommandConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = migrateConfigurationRepositoryPathSanitizer.Sanitize(configuration.RepositoryRoots)
	sanitized.SourceBranch = strings.TrimSpace(configuration.SourceBranch)
	if len(sanitized.SourceBranch) == 0 {
		sanitized.SourceBranch = string(BranchMain)
	}
	sanitized.TargetBranch = strings.TrimSpace(configuration.TargetBranch)
	if len(sanitized.TargetBranch) == 0 {
		sanitized.TargetBranch = string(BranchMaster)
	}
	return sanitized
}
