package audit

import "strings"

const (
	configurationRootsKeyConstant = "roots"
	configurationDebugKeyConstant = "debug"
)

// CommandConfiguration captures persistent settings for the audit command.
type CommandConfiguration struct {
	Roots []string `mapstructure:"roots"`
	Debug bool     `mapstructure:"debug"`
}

// DefaultCommandConfiguration returns baseline configuration values for the audit command.
func DefaultCommandConfiguration() CommandConfiguration {
	return CommandConfiguration{
		Roots: []string{defaultRootPathConstant},
		Debug: false,
	}
}

// DefaultConfigurationValues exposes Viper-ready defaults for audit configuration keys.
func DefaultConfigurationValues(rootKey string) map[string]any {
	defaults := DefaultCommandConfiguration()

	return map[string]any{
		rootKey + "." + configurationRootsKeyConstant: defaults.Roots,
		rootKey + "." + configurationDebugKeyConstant: defaults.Debug,
	}
}

// sanitize trims whitespace and applies defaults to unset configuration values.
func (configuration CommandConfiguration) sanitize() CommandConfiguration {
	sanitized := configuration

	sanitized.Roots = sanitizeRoots(configuration.Roots)
	if len(sanitized.Roots) == 0 {
		sanitized.Roots = append([]string{}, defaultRootPathConstant)
	}

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
