package branches

import "strings"

const (
	defaultRepositoryRootConstant = "."
)

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
		RemoteName:       defaultRemoteNameConstant,
		PullRequestLimit: defaultPullRequestLimitConstant,
		DryRun:           false,
		RepositoryRoots:  []string{defaultRepositoryRootConstant},
	}
}

// sanitize populates zero values with defaults and trims whitespace.
func (configuration CommandConfiguration) sanitize() CommandConfiguration {
	sanitized := configuration

	if len(strings.TrimSpace(configuration.RemoteName)) == 0 {
		sanitized.RemoteName = defaultRemoteNameConstant
	}

	if configuration.PullRequestLimit <= 0 {
		sanitized.PullRequestLimit = defaultPullRequestLimitConstant
	}

	sanitized.RepositoryRoots = sanitizeRoots(configuration.RepositoryRoots)
	if len(sanitized.RepositoryRoots) == 0 {
		sanitized.RepositoryRoots = append([]string{}, defaultRepositoryRootConstant)
	}

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
