package workflow

import (
	"strings"

	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

var workflowConfigurationHomeDirectoryExpander = pathutils.NewHomeExpander()

// CommandConfiguration captures configuration values for workflow.
type CommandConfiguration struct {
	Roots     []string `mapstructure:"roots"`
	DryRun    bool     `mapstructure:"dry_run"`
	AssumeYes bool     `mapstructure:"assume_yes"`
}

// DefaultCommandConfiguration provides default workflow command settings for workflow.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		DryRun:    false,
		AssumeYes: false,
	}
}

// Sanitize normalizes configuration values.
func (configuration CommandConfiguration) Sanitize() CommandConfiguration {
	sanitized := configuration
	sanitized.Roots = sanitizeRoots(configuration.Roots)
	return sanitized
}

func sanitizeRoots(raw []string) []string {
	trimmed := make([]string, 0, len(raw))
	for _, rawRoot := range raw {
		trimmedRoot := strings.TrimSpace(rawRoot)
		if len(trimmedRoot) == 0 {
			continue
		}
		expandedRoot := workflowConfigurationHomeDirectoryExpander.Expand(trimmedRoot)
		trimmed = append(trimmed, expandedRoot)
	}
	return trimmed
}
