package migrate

import (
	pathutils "github.com/temirov/gix/internal/utils/path"
)

var migrateConfigurationRepositoryPathSanitizer = pathutils.NewRepositoryPathSanitizer()

// CommandConfiguration captures persisted configuration for branch migration.
type CommandConfiguration struct {
	EnableDebugLogging bool     `mapstructure:"debug"`
	RepositoryRoots    []string `mapstructure:"roots"`
}

// DefaultCommandConfiguration returns baseline configuration values for branch migration.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		EnableDebugLogging: false,
		RepositoryRoots:    nil,
	}
}

// Sanitize trims configured values and removes empty entries.
func (configuration CommandConfiguration) Sanitize() CommandConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = migrateConfigurationRepositoryPathSanitizer.Sanitize(configuration.RepositoryRoots)
	return sanitized
}
