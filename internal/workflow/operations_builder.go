package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/temirov/git_scripts/internal/repos/shared"
)

const (
	protocolConversionInvalidFromMessageConstant  = "convert-protocol step requires a valid 'from' protocol"
	protocolConversionInvalidToMessageConstant    = "convert-protocol step requires a valid 'to' protocol"
	protocolConversionSameProtocolMessageConstant = "convert-protocol step requires distinct source and target protocols"
	branchMigrationTargetsRequiredMessageConstant = "migrate-branch step requires at least one target"
)

// BuildOperations converts the declarative configuration into executable operations.
func BuildOperations(configuration Configuration) ([]Operation, error) {
	operations := make([]Operation, 0, len(configuration.Steps))
	for stepIndex := range configuration.Steps {
		step := configuration.Steps[stepIndex]
		operation, buildError := buildOperationFromStep(step)
		if buildError != nil {
			return nil, buildError
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

func buildOperationFromStep(step StepConfiguration) (Operation, error) {
	switch step.Operation {
	case OperationTypeProtocolConversion:
		return buildProtocolConversionOperation(step.Options)
	case OperationTypeCanonicalRemote:
		return &CanonicalRemoteOperation{}, nil
	case OperationTypeRenameDirectories:
		return buildRenameOperation(step.Options)
	case OperationTypeBranchMigration:
		return buildBranchMigrationOperation(step.Options)
	case OperationTypeAuditReport:
		return buildAuditReportOperation(step.Options)
	default:
		return nil, fmt.Errorf("unsupported workflow operation: %s", step.Operation)
	}
}

func buildProtocolConversionOperation(options map[string]any) (Operation, error) {
	reader := newOptionReader(options)
	fromValue, fromExists, fromError := reader.stringValue(optionFromKeyConstant)
	if fromError != nil {
		return nil, fromError
	}
	if !fromExists || len(fromValue) == 0 {
		return nil, errors.New(protocolConversionInvalidFromMessageConstant)
	}

	toValue, toExists, toError := reader.stringValue(optionToKeyConstant)
	if toError != nil {
		return nil, toError
	}
	if !toExists || len(toValue) == 0 {
		return nil, errors.New(protocolConversionInvalidToMessageConstant)
	}

	fromProtocol, fromParseError := parseProtocolValue(fromValue)
	if fromParseError != nil {
		return nil, fromParseError
	}

	toProtocol, toParseError := parseProtocolValue(toValue)
	if toParseError != nil {
		return nil, toParseError
	}

	if fromProtocol == toProtocol {
		return nil, errors.New(protocolConversionSameProtocolMessageConstant)
	}

	return &ProtocolConversionOperation{FromProtocol: fromProtocol, ToProtocol: toProtocol}, nil
}

func buildRenameOperation(options map[string]any) (Operation, error) {
	reader := newOptionReader(options)
	requireClean, _, requireCleanError := reader.boolValue(optionRequireCleanKeyConstant)
	if requireCleanError != nil {
		return nil, requireCleanError
	}
	return &RenameOperation{RequireCleanWorktree: requireClean}, nil
}

func buildBranchMigrationOperation(options map[string]any) (Operation, error) {
	reader := newOptionReader(options)
	targetEntries, targetsExist, targetsError := reader.mapSlice(optionTargetsKeyConstant)
	if targetsError != nil {
		return nil, targetsError
	}
	if !targetsExist || len(targetEntries) == 0 {
		return nil, errors.New(branchMigrationTargetsRequiredMessageConstant)
	}

	targets := make([]BranchMigrationTarget, 0, len(targetEntries))
	for targetIndex := range targetEntries {
		targetReader := newOptionReader(targetEntries[targetIndex])
		repositoryValue, _, repositoryError := targetReader.stringValue(optionRepositoryKeyConstant)
		if repositoryError != nil {
			return nil, repositoryError
		}
		pathValue, _, pathError := targetReader.stringValue(optionPathKeyConstant)
		if pathError != nil {
			return nil, pathError
		}
		remoteNameValue, remoteNameExists, remoteNameError := targetReader.stringValue(optionRemoteNameKeyConstant)
		if remoteNameError != nil {
			return nil, remoteNameError
		}
		sourceBranchValue, sourceExists, sourceError := targetReader.stringValue(optionSourceBranchKeyConstant)
		if sourceError != nil {
			return nil, sourceError
		}
		targetBranchValue, targetExists, targetError := targetReader.stringValue(optionTargetBranchKeyConstant)
		if targetError != nil {
			return nil, targetError
		}
		workflowsDirectoryValue, workflowsExists, workflowsError := targetReader.stringValue(optionWorkflowsDirectoryKeyConstant)
		if workflowsError != nil {
			return nil, workflowsError
		}
		pushUpdatesValue, pushUpdatesExists, pushUpdatesError := targetReader.boolValue(optionPushUpdatesKeyConstant)
		if pushUpdatesError != nil {
			return nil, pushUpdatesError
		}

		targets = append(targets, BranchMigrationTarget{
			RepositoryIdentifier: strings.TrimSpace(repositoryValue),
			RepositoryPath:       normalizePathCandidate(pathValue),
			RemoteName:           defaultRemoteName(remoteNameExists, remoteNameValue),
			SourceBranch:         defaultSourceBranch(sourceExists, sourceBranchValue),
			TargetBranch:         defaultTargetBranch(targetExists, targetBranchValue),
			WorkflowsDirectory:   defaultWorkflowsDirectory(workflowsExists, workflowsDirectoryValue),
			PushUpdates:          defaultPushUpdates(pushUpdatesExists, pushUpdatesValue),
		})
	}

	return &BranchMigrationOperation{Targets: targets}, nil
}

func buildAuditReportOperation(options map[string]any) (Operation, error) {
	reader := newOptionReader(options)
	outputPath, outputExists, outputError := reader.stringValue(optionOutputPathKeyConstant)
	if outputError != nil {
		return nil, outputError
	}

	return &AuditReportOperation{OutputPath: strings.TrimSpace(outputPath), WriteToFile: outputExists && len(strings.TrimSpace(outputPath)) > 0}, nil
}

func parseProtocolValue(raw string) (shared.RemoteProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(shared.RemoteProtocolGit):
		return shared.RemoteProtocolGit, nil
	case string(shared.RemoteProtocolSSH):
		return shared.RemoteProtocolSSH, nil
	case string(shared.RemoteProtocolHTTPS):
		return shared.RemoteProtocolHTTPS, nil
	default:
		return "", fmt.Errorf("unsupported protocol value: %s", raw)
	}
}

func defaultRemoteName(explicit bool, value string) string {
	if explicit {
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 0 {
			return trimmed
		}
	}
	return defaultMigrationRemoteNameConstant
}

func defaultSourceBranch(explicit bool, value string) string {
	if explicit {
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 0 {
			return trimmed
		}
	}
	return defaultMigrationSourceBranchConstant
}

func defaultTargetBranch(explicit bool, value string) string {
	if explicit {
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 0 {
			return trimmed
		}
	}
	return defaultMigrationTargetBranchConstant
}

func defaultWorkflowsDirectory(explicit bool, value string) string {
	if explicit {
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 0 {
			return trimmed
		}
	}
	return defaultMigrationWorkflowsDirectoryConstant
}

func defaultPushUpdates(explicit bool, value bool) bool {
	if explicit {
		return value
	}
	return true
}
