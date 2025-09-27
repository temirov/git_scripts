package audit

import (
	"strings"

	pathutils "github.com/temirov/git_scripts/internal/utils/path"
)

var auditConfigurationHomeDirectoryExpander = pathutils.NewHomeExpander()

// CommandConfiguration captures persistent settings for the audit command.
type CommandConfiguration struct {
	Roots []string `mapstructure:"roots"`
	Debug bool     `mapstructure:"debug"`
}

// DefaultCommandConfiguration returns baseline configuration values for the audit command.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		Roots: nil,
		Debug: false,
	}
}

// Sanitize trims whitespace and applies defaults to unset configuration values.
func (configuration CommandConfiguration) Sanitize() CommandConfiguration {
	sanitized := configuration

	sanitized.Roots = sanitizeRoots(configuration.Roots)

	return sanitized
}

func sanitizeRoots(raw []string) []string {
	sanitized := make([]string, 0, len(raw))
	for rawRootIndex := range raw {
		trimmed := strings.TrimSpace(raw[rawRootIndex])
		if len(trimmed) == 0 {
			continue
		}
		expandedRoot := auditConfigurationHomeDirectoryExpander.Expand(trimmed)
		sanitized = append(sanitized, expandedRoot)
	}
	return sanitized
}
