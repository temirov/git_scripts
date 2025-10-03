package workflow

import (
	"fmt"
	"strings"
)

const (
	optionFromKeyConstant               = "from"
	optionToKeyConstant                 = "to"
	optionRequireCleanKeyConstant       = "require_clean"
	optionIncludeOwnerKeyConstant       = "include_owner"
	optionTargetsKeyConstant            = "targets"
	optionRemoteNameKeyConstant         = "remote_name"
	optionSourceBranchKeyConstant       = "source_branch"
	optionTargetBranchKeyConstant       = "target_branch"
	optionPushToRemoteKeyConstant       = "push_to_remote"
	optionDeleteSourceBranchKeyConstant = "delete_source_branch"
	optionOutputPathKeyConstant         = "output"
)

type optionReader struct {
	entries map[string]any
}

func newOptionReader(raw map[string]any) optionReader {
	normalized := make(map[string]any, len(raw))
	for key, value := range raw {
		normalized[strings.ToLower(strings.TrimSpace(key))] = value
	}
	return optionReader{entries: normalized}
}

func (reader optionReader) stringValue(key string) (string, bool, error) {
	value, exists := reader.entries[key]
	if !exists {
		return "", false, nil
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true, nil
	case fmt.Stringer:
		return strings.TrimSpace(typed.String()), true, nil
	default:
		return "", true, fmt.Errorf("option %s must be a string", key)
	}
}

func (reader optionReader) boolValue(key string) (bool, bool, error) {
	value, exists := reader.entries[key]
	if !exists {
		return false, false, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, true, nil
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(typed))
		if trimmed == "true" {
			return true, true, nil
		}
		if trimmed == "false" {
			return false, true, nil
		}
	default:
		return false, true, fmt.Errorf("option %s must be a boolean", key)
	}
	return false, true, fmt.Errorf("option %s must be a boolean", key)
}

func (reader optionReader) mapSlice(key string) ([]map[string]any, bool, error) {
	value, exists := reader.entries[key]
	if !exists {
		return nil, false, nil
	}
	listValue, ok := value.([]any)
	if !ok {
		return nil, true, fmt.Errorf("option %s must be a list", key)
	}
	maps := make([]map[string]any, 0, len(listValue))
	for index := range listValue {
		entry, ok := listValue[index].(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("option %s entries must be maps", key)
		}
		maps = append(maps, entry)
	}
	return maps, true, nil
}
