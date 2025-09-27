package branches

import "strings"

// CommandConfiguration captures configuration values for the branch cleanup command.
type CommandConfiguration struct {
	RemoteName       string   `mapstructure:"remote"`
	PullRequestLimit int      `mapstructure:"limit"`
	DryRun           bool     `mapstructure:"dry_run"`
	RepositoryRoots  []string `mapstructure:"roots"`
}

// DefaultCommandConfiguration provides baseline configuration values for branch cleanup.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		RemoteName:       "",
		PullRequestLimit: 0,
		DryRun:           false,
		RepositoryRoots:  nil,
	}
}

// sanitize trims configuration values without applying implicit defaults.
func (configuration CommandConfiguration) sanitize() CommandConfiguration {
	sanitized := configuration

	sanitized.RemoteName = strings.TrimSpace(configuration.RemoteName)
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
	return sanitized
}
