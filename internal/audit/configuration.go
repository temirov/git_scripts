package audit

import "strings"

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

// sanitize trims whitespace and applies defaults to unset configuration values.
func (configuration CommandConfiguration) sanitize() CommandConfiguration {
	sanitized := configuration

	sanitized.Roots = sanitizeRoots(configuration.Roots)

	return sanitized
}

func sanitizeRoots(raw []string) []string {
	sanitized := make([]string, 0, len(raw))
	for index := range raw {
		trimmed := strings.TrimSpace(raw[index])
		if len(trimmed) == 0 {
			continue
		}
		sanitized = append(sanitized, trimmed)
	}
	return sanitized
}
