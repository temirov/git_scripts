package branches

import (
	"strings"

	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

var branchConfigurationHomeDirectoryExpander = pathutils.NewHomeExpander()

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

// Sanitize trims configuration values without applying implicit defaults.
func (configuration CommandConfiguration) Sanitize() CommandConfiguration {
	sanitized := configuration

	sanitized.RemoteName = strings.TrimSpace(configuration.RemoteName)
	sanitized.RepositoryRoots = sanitizeRoots(configuration.RepositoryRoots)

	return sanitized
}

func sanitizeRoots(raw []string) []string {
	sanitized := make([]string, 0, len(raw))
	for _, rootCandidate := range raw {
		trimmed := strings.TrimSpace(rootCandidate)
		if len(trimmed) == 0 {
			continue
		}
		expandedRoot := branchConfigurationHomeDirectoryExpander.Expand(trimmed)
		sanitized = append(sanitized, expandedRoot)
	}
	return sanitized
}
