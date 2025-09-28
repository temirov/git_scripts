package utils

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const (
	environmentKeySeparatorOldConstant          = "."
	environmentKeySeparatorNewConstant          = "_"
	configurationReadErrorTemplateConstant      = "failed to read configuration: %w"
	configurationUnmarshalErrorTemplateConstant = "failed to parse configuration: %w"
)

// ConfigurationLoader wraps Viper to load structured configuration files and environment overrides.
type ConfigurationLoader struct {
	configurationName      string
	configurationType      string
	environmentPrefix      string
	searchPaths            []string
	environmentKeyReplacer *strings.Replacer
}

// LoadedConfiguration surfaces metadata about the resolved configuration.
type LoadedConfiguration struct {
	ConfigFileUsed string
}

// NewConfigurationLoader creates a loader that searches known paths and respects an environment prefix.
func NewConfigurationLoader(configurationName string, configurationType string, environmentPrefix string, searchPaths []string) *ConfigurationLoader {
	duplicatedSearchPaths := make([]string, len(searchPaths))
	copy(duplicatedSearchPaths, searchPaths)

	return &ConfigurationLoader{
		configurationName:      configurationName,
		configurationType:      configurationType,
		environmentPrefix:      environmentPrefix,
		searchPaths:            duplicatedSearchPaths,
		environmentKeyReplacer: strings.NewReplacer(environmentKeySeparatorOldConstant, environmentKeySeparatorNewConstant),
	}
}

// LoadConfiguration populates targetConfiguration using configuration files, defaults, and environment variables.
func (loader *ConfigurationLoader) LoadConfiguration(configurationFilePath string, defaultValues map[string]any, targetConfiguration any) (LoadedConfiguration, error) {
	viperInstance := viper.New()
	viperInstance.SetConfigName(loader.configurationName)
	viperInstance.SetConfigType(loader.configurationType)

	for _, searchPath := range loader.searchPaths {
		viperInstance.AddConfigPath(searchPath)
	}

	viperInstance.SetEnvPrefix(loader.environmentPrefix)
	if loader.environmentKeyReplacer != nil {
		viperInstance.SetEnvKeyReplacer(loader.environmentKeyReplacer)
	}
	viperInstance.AutomaticEnv()

	for defaultKey, defaultValue := range defaultValues {
		viperInstance.SetDefault(defaultKey, defaultValue)
	}

	if len(configurationFilePath) > 0 {
		viperInstance.SetConfigFile(configurationFilePath)
	}

	readError := viperInstance.ReadInConfig()
	if readError != nil {
		if _, isNotFound := readError.(viper.ConfigFileNotFoundError); !isNotFound {
			return LoadedConfiguration{}, fmt.Errorf(configurationReadErrorTemplateConstant, readError)
		}
	}

	unmarshalError := viperInstance.Unmarshal(targetConfiguration)
	if unmarshalError != nil {
		return LoadedConfiguration{}, fmt.Errorf(configurationUnmarshalErrorTemplateConstant, unmarshalError)
	}

	loadedConfiguration := LoadedConfiguration{
		ConfigFileUsed: viperInstance.ConfigFileUsed(),
	}

	return loadedConfiguration, nil
}
