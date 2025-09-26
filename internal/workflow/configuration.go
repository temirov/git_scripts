package workflow

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configurationStepsFieldNameConstant               = "steps"
	configurationOperationFieldNameConstant           = "operation"
	configurationOptionsFieldNameConstant             = "with"
	configurationLoadErrorTemplateConstant            = "failed to load workflow configuration: %w"
	configurationParseErrorTemplateConstant           = "failed to parse workflow configuration: %w"
	configurationPathRequiredMessageConstant          = "workflow configuration path must be provided"
	configurationEmptyStepsMessageConstant            = "workflow configuration must define at least one step"
	configurationOperationMissingMessageConstant      = "workflow step missing operation name"
	configurationToolNameRequiredMessageConstant      = "workflow tool names must be non-empty"
	configurationDuplicateToolNameMessageConstant     = "workflow configuration defines duplicate tool names"
	configurationToolOperationMissingTemplateConstant = "workflow tool %s missing operation name"
)

// OperationType identifies supported workflow operations.
type OperationType string

// Supported workflow operations.
const (
	OperationTypeProtocolConversion OperationType = OperationType("convert-protocol")
	OperationTypeCanonicalRemote    OperationType = OperationType("update-canonical-remote")
	OperationTypeRenameDirectories  OperationType = OperationType("rename-directories")
	OperationTypeBranchMigration    OperationType = OperationType("migrate-branch")
	OperationTypeAuditReport        OperationType = OperationType("audit-report")
)

// Configuration describes the ordered workflow steps and reusable tool definitions loaded from YAML or JSON.
type Configuration struct {
	Tools map[string]ToolConfiguration `yaml:"tools" json:"tools"`
	Steps []StepConfiguration          `yaml:"steps" json:"steps"`
}

// StepConfiguration associates an operation type with declarative options.
type StepConfiguration struct {
	Operation OperationType  `yaml:"operation" json:"operation"`
	Options   map[string]any `yaml:"with" json:"with"`
}

// ToolConfiguration describes reusable workflow options for a specific operation type.
type ToolConfiguration struct {
	Operation OperationType  `yaml:"operation" json:"operation"`
	Options   map[string]any `yaml:"with" json:"with"`
}

// LoadConfiguration reads the workflow definition from disk and performs basic validation.
func LoadConfiguration(filePath string) (Configuration, error) {
	trimmedPath := strings.TrimSpace(filePath)
	if len(trimmedPath) == 0 {
		return Configuration{}, errors.New(configurationPathRequiredMessageConstant)
	}

	contentBytes, readError := os.ReadFile(trimmedPath)
	if readError != nil {
		return Configuration{}, fmt.Errorf(configurationLoadErrorTemplateConstant, readError)
	}

	var configuration Configuration
	if unmarshalError := yaml.Unmarshal(contentBytes, &configuration); unmarshalError != nil {
		return Configuration{}, fmt.Errorf(configurationParseErrorTemplateConstant, unmarshalError)
	}

	normalizedTools, toolsError := normalizeTools(configuration.Tools)
	if toolsError != nil {
		return Configuration{}, toolsError
	}
	configuration.Tools = normalizedTools

	if len(configuration.Steps) == 0 {
		return Configuration{}, errors.New(configurationEmptyStepsMessageConstant)
	}

	for stepIndex := range configuration.Steps {
		if len(strings.TrimSpace(string(configuration.Steps[stepIndex].Operation))) == 0 {
			return Configuration{}, errors.New(configurationOperationMissingMessageConstant)
		}
	}

	return configuration, nil
}

func normalizeTools(raw map[string]ToolConfiguration) (map[string]ToolConfiguration, error) {
	if raw == nil {
		return nil, nil
	}

	normalized := make(map[string]ToolConfiguration, len(raw))
	for originalName, tool := range raw {
		trimmedName := strings.TrimSpace(originalName)
		if len(trimmedName) == 0 {
			return nil, errors.New(configurationToolNameRequiredMessageConstant)
		}
		if _, exists := normalized[trimmedName]; exists {
			return nil, errors.New(configurationDuplicateToolNameMessageConstant)
		}
		if len(strings.TrimSpace(string(tool.Operation))) == 0 {
			return nil, fmt.Errorf(configurationToolOperationMissingTemplateConstant, trimmedName)
		}
		normalized[trimmedName] = tool
	}

	return normalized, nil
}
