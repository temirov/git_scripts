package migrate

import (
	"strings"

	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

var migrateConfigurationHomeDirectoryExpander = pathutils.NewHomeExpander()

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
	sanitized.RepositoryRoots = sanitizeRoots(configuration.RepositoryRoots)
	return sanitized
}

func sanitizeRoots(raw []string) []string {
	sanitized := make([]string, 0, len(raw))
	for _, candidateRoot := range raw {
		trimmed := strings.TrimSpace(candidateRoot)
		if len(trimmed) == 0 {
			continue
		}
		expandedRoot := migrateConfigurationHomeDirectoryExpander.Expand(trimmed)
		sanitized = append(sanitized, expandedRoot)
	}
	return sanitized
}
