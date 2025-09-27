package workflow

import "strings"

// CommandConfiguration captures configuration values for workflow.
type CommandConfiguration struct {
	Roots     []string `mapstructure:"roots"`
	DryRun    bool     `mapstructure:"dry_run"`
	AssumeYes bool     `mapstructure:"assume_yes"`
}

// DefaultCommandConfiguration provides default workflow command settings for workflow.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		Roots:     []string{defaultWorkflowRootConstant},
		DryRun:    false,
		AssumeYes: false,
	}
}

// sanitize normalizes configuration values.
func (configuration CommandConfiguration) sanitize() CommandConfiguration {
	sanitized := configuration
	sanitized.Roots = sanitizeRoots(configuration.Roots)
	if len(sanitized.Roots) == 0 {
		sanitized.Roots = append([]string{}, defaultWorkflowRootConstant)
	}
	return sanitized
}

func sanitizeRoots(raw []string) []string {
	trimmed := make([]string, 0, len(raw))
	for _, candidate := range raw {
		value := strings.TrimSpace(candidate)
		if len(value) == 0 {
			continue
		}
		trimmed = append(trimmed, value)
	}
	return trimmed
}
