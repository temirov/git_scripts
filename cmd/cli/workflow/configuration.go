package workflow

import (
	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

var workflowConfigurationRepositoryPathSanitizer = pathutils.NewRepositoryPathSanitizer()

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
	sanitized.Roots = workflowConfigurationRepositoryPathSanitizer.Sanitize(configuration.Roots)
	return sanitized
}
