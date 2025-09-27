package utils

import "context"

const (
	configurationFilePathContextKeyConstant = commandContextKey("configurationFilePath")
)

type commandContextKey string

// CommandContextAccessor manages values stored in command execution contexts.
type CommandContextAccessor struct{}

// NewCommandContextAccessor constructs a CommandContextAccessor instance.
func NewCommandContextAccessor() CommandContextAccessor {
	return CommandContextAccessor{}
}

// WithConfigurationFilePath attaches the configuration file path to the provided context.
func (accessor CommandContextAccessor) WithConfigurationFilePath(parentContext context.Context, configurationFilePath string) context.Context {
	if parentContext == nil {
		parentContext = context.Background()
	}
	return context.WithValue(parentContext, configurationFilePathContextKeyConstant, configurationFilePath)
}

// ConfigurationFilePath extracts the configuration file path from the provided context.
func (accessor CommandContextAccessor) ConfigurationFilePath(executionContext context.Context) (string, bool) {
	if executionContext == nil {
		return "", false
	}
	configurationFilePath, configurationFilePathAvailable := executionContext.Value(configurationFilePathContextKeyConstant).(string)
	if !configurationFilePathAvailable {
		return "", false
	}
	return configurationFilePath, true
}
