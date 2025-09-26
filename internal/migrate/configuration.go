package migrate

import "strings"

const (
	migrateConfigurationDebugKeyConstant = "debug"
	migrateConfigurationRootsKeyConstant = "roots"
	defaultRepositoryRootConstant        = "."
)

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

// DefaultConfigurationValues exposes configuration defaults for integration with Viper.
func DefaultConfigurationValues(rootKey string) map[string]any {
	defaults := DefaultCommandConfiguration()

	return map[string]any{
		rootKey + "." + migrateConfigurationDebugKeyConstant: defaults.EnableDebugLogging,
		rootKey + "." + migrateConfigurationRootsKeyConstant: defaults.RepositoryRoots,
	}
}

// sanitize trims configured values and removes empty entries.
func (configuration CommandConfiguration) sanitize() CommandConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = sanitizeRoots(configuration.RepositoryRoots)
	return sanitized
}

func sanitizeRoots(raw []string) []string {
	sanitized := make([]string, 0, len(raw))
	for _, candidate := range raw {
		trimmed := strings.TrimSpace(candidate)
		if len(trimmed) == 0 {
			continue
		}
		sanitized = append(sanitized, trimmed)
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}
