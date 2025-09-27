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
	configurationWorkflowTypeErrorMessageConstant     = "workflow block must be either a mapping or a sequence"
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
	Tools []NamedToolConfiguration `yaml:"tools" json:"tools"`
	Steps []StepConfiguration      `yaml:"steps" json:"steps"`

	toolLookup map[string]ToolConfiguration
}

// NamedToolConfiguration captures a reusable operation definition along with its canonical reference name.
type NamedToolConfiguration struct {
	Name              string `yaml:"name" json:"name"`
	ToolConfiguration `yaml:",inline" json:",inline"`
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
	directUnmarshalError := yaml.Unmarshal(contentBytes, &configuration)

	wrapperConfiguration, workflowFound, workflowError := decodeWorkflowBlock(contentBytes)

	if directUnmarshalError != nil {
		if workflowError != nil {
			return Configuration{}, fmt.Errorf(configurationParseErrorTemplateConstant, workflowError)
		}
		if !workflowFound {
			return Configuration{}, fmt.Errorf(configurationParseErrorTemplateConstant, directUnmarshalError)
		}
		configuration = wrapperConfiguration
	} else {
		if workflowError != nil {
			return Configuration{}, fmt.Errorf(configurationParseErrorTemplateConstant, workflowError)
		}
		if workflowFound {
			if len(configuration.Tools) == 0 && len(wrapperConfiguration.Tools) > 0 {
				configuration.Tools = wrapperConfiguration.Tools
			}
			if len(configuration.Steps) == 0 && len(wrapperConfiguration.Steps) > 0 {
				configuration.Steps = wrapperConfiguration.Steps
			}
		}
	}

	if len(configuration.Tools) == 0 {
		workflowTools, workflowToolsFound, workflowToolsError := decodeWorkflowTools(contentBytes)
		if workflowToolsError != nil {
			return Configuration{}, fmt.Errorf(configurationParseErrorTemplateConstant, workflowToolsError)
		}
		if workflowToolsFound {
			configuration.Tools = workflowTools
		}
	}

	toolLookup, toolsError := buildToolLookup(configuration.Tools)
	if toolsError != nil {
		return Configuration{}, toolsError
	}
	configuration.toolLookup = toolLookup

	if len(configuration.Steps) == 0 {
		return Configuration{}, errors.New(configurationEmptyStepsMessageConstant)
	}

	for stepIndex := range configuration.Steps {
		trimmedOperation := strings.TrimSpace(string(configuration.Steps[stepIndex].Operation))
		if len(trimmedOperation) == 0 {
			if !stepIncludesToolReference(configuration.Steps[stepIndex].Options) {
				return Configuration{}, errors.New(configurationOperationMissingMessageConstant)
			}
		} else {
			configuration.Steps[stepIndex].Operation = OperationType(trimmedOperation)
		}
	}

	return configuration, nil
}

func decodeWorkflowBlock(contentBytes []byte) (Configuration, bool, error) {
	var workflowWrapper struct {
		Workflow yaml.Node `yaml:"workflow" json:"workflow"`
	}

	if unmarshalError := yaml.Unmarshal(contentBytes, &workflowWrapper); unmarshalError != nil {
		return Configuration{}, false, unmarshalError
	}

	if workflowWrapper.Workflow.Kind == 0 {
		return Configuration{}, false, nil
	}

	switch workflowWrapper.Workflow.Kind {
	case yaml.MappingNode:
		var nestedConfiguration Configuration
		if decodeError := workflowWrapper.Workflow.Decode(&nestedConfiguration); decodeError != nil {
			return Configuration{}, true, decodeError
		}
		return nestedConfiguration, true, nil
	case yaml.SequenceNode:
		var stepConfigurations []StepConfiguration
		if decodeError := workflowWrapper.Workflow.Decode(&stepConfigurations); decodeError != nil {
			return Configuration{}, true, decodeError
		}
		return Configuration{Steps: stepConfigurations}, true, nil
	default:
		return Configuration{}, true, errors.New(configurationWorkflowTypeErrorMessageConstant)
	}
}

func decodeWorkflowTools(contentBytes []byte) ([]NamedToolConfiguration, bool, error) {
	var toolsWrapper struct {
		WorkflowTools yaml.Node `yaml:"workflow_tools" json:"workflow_tools"`
	}

	if unmarshalError := yaml.Unmarshal(contentBytes, &toolsWrapper); unmarshalError != nil {
		return nil, false, unmarshalError
	}

	if toolsWrapper.WorkflowTools.Kind == 0 {
		return nil, false, nil
	}

	var namedTools []NamedToolConfiguration
	if decodeError := toolsWrapper.WorkflowTools.Decode(&namedTools); decodeError != nil {
		return nil, true, decodeError
	}

	return namedTools, true, nil
}

func buildToolLookup(tools []NamedToolConfiguration) (map[string]ToolConfiguration, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	lookup := make(map[string]ToolConfiguration, len(tools))
	for toolIndex := range tools {
		trimmedName := strings.TrimSpace(tools[toolIndex].Name)
		if len(trimmedName) == 0 {
			return nil, errors.New(configurationToolNameRequiredMessageConstant)
		}
		if _, exists := lookup[trimmedName]; exists {
			return nil, errors.New(configurationDuplicateToolNameMessageConstant)
		}
		if len(strings.TrimSpace(string(tools[toolIndex].Operation))) == 0 {
			return nil, fmt.Errorf(configurationToolOperationMissingTemplateConstant, trimmedName)
		}
		tools[toolIndex].Name = trimmedName
		lookup[trimmedName] = ToolConfiguration{
			Operation: tools[toolIndex].Operation,
			Options:   tools[toolIndex].Options,
		}
	}

	return lookup, nil
}

func stepIncludesToolReference(options map[string]any) bool {
	if len(options) == 0 {
		return false
	}

	for rawKey := range options {
		if strings.EqualFold(strings.TrimSpace(rawKey), optionToolReferenceKeyConstant) {
			return true
		}
	}

	return false
}
