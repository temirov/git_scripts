package workflow

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/temirov/git_scripts/internal/audit"
)

const (
	auditPlanMessageTemplateConstant      = "WORKFLOW-PLAN: audit report → %s\n"
	auditWriteMessageTemplateConstant     = "WORKFLOW-AUDIT: wrote report to %s\n"
	auditReportDestinationStdoutConstant  = "stdout"
	auditCSVHeaderFinalRepositoryConstant = "final_github_repo"
	auditCSVHeaderFolderNameConstant      = "folder_name"
	auditCSVHeaderNameMatchesConstant     = "name_matches"
	auditCSVHeaderRemoteDefaultConstant   = "remote_default_branch"
	auditCSVHeaderLocalBranchConstant     = "local_branch"
	auditCSVHeaderInSyncConstant          = "in_sync"
	auditCSVHeaderRemoteProtocolConstant  = "remote_protocol"
	auditCSVHeaderOriginCanonicalConstant = "origin_matches_canonical"
)

// AuditReportOperation emits an audit CSV summarizing repository state.
type AuditReportOperation struct {
	OutputPath  string
	WriteToFile bool
}

// Name identifies the operation type.
func (operation *AuditReportOperation) Name() string {
	return string(OperationTypeAuditReport)
}

// Execute writes the audit report using the current repository state.
func (operation *AuditReportOperation) Execute(executionContext context.Context, environment *Environment, state *State) (executionError error) {
	if environment == nil || state == nil {
		return nil
	}

	destination := auditReportDestinationStdoutConstant
	if operation.WriteToFile {
		destination = operation.OutputPath
	}

	if environment.DryRun {
		if environment.Output != nil {
			fmt.Fprintf(environment.Output, auditPlanMessageTemplateConstant, destination)
		}
		return nil
	}

	var writer io.Writer
	var closeFunction func() error
	if operation.WriteToFile {
		fileHandle, createError := os.Create(operation.OutputPath)
		if createError != nil {
			return createError
		}
		writer = fileHandle
		closeFunction = fileHandle.Close
	} else {
		if environment.Output != nil {
			writer = environment.Output
		} else {
			writer = io.Discard
		}
	}

	if closeFunction != nil {
		defer func() {
			closeError := closeFunction()
			if closeError != nil && executionError == nil {
				executionError = closeError
			}
		}()
	}

	csvWriter := csv.NewWriter(writer)
	header := []string{
		auditCSVHeaderFinalRepositoryConstant,
		auditCSVHeaderFolderNameConstant,
		auditCSVHeaderNameMatchesConstant,
		auditCSVHeaderRemoteDefaultConstant,
		auditCSVHeaderLocalBranchConstant,
		auditCSVHeaderInSyncConstant,
		auditCSVHeaderRemoteProtocolConstant,
		auditCSVHeaderOriginCanonicalConstant,
	}

	if writeError := csvWriter.Write(header); writeError != nil {
		return writeError
	}

	for repositoryIndex := range state.Repositories {
		repository := state.Repositories[repositoryIndex]
		row := buildAuditReportRow(repository.Inspection)
		if writeError := csvWriter.Write(row); writeError != nil {
			return writeError
		}
	}

	csvWriter.Flush()
	if flushError := csvWriter.Error(); flushError != nil {
		return flushError
	}

	if operation.WriteToFile && environment.Output != nil {
		fmt.Fprintf(environment.Output, auditWriteMessageTemplateConstant, destination)
	}

	return nil
}

func buildAuditReportRow(inspection audit.RepositoryInspection) []string {
	finalRepository := strings.TrimSpace(inspection.CanonicalOwnerRepo)
	if len(finalRepository) == 0 {
		finalRepository = inspection.OriginOwnerRepo
	}

	nameMatches := audit.TernaryValueNo
	if len(inspection.DesiredFolderName) > 0 && inspection.DesiredFolderName == inspection.FolderName {
		nameMatches = audit.TernaryValueYes
	}

	return []string{
		finalRepository,
		inspection.FolderName,
		string(nameMatches),
		inspection.RemoteDefaultBranch,
		inspection.LocalBranch,
		string(inspection.InSyncStatus),
		string(inspection.RemoteProtocol),
		string(inspection.OriginMatchesCanonical),
	}
}
