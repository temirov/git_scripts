package audit

import "github.com/temirov/git_scripts/internal/repos/shared"

// RemoteProtocolType enumerates supported git remote protocols.
type RemoteProtocolType = shared.RemoteProtocol

// Remote protocol values supported by the audit command.
const (
        RemoteProtocolGit   RemoteProtocolType = shared.RemoteProtocolGit
        RemoteProtocolSSH   RemoteProtocolType = shared.RemoteProtocolSSH
        RemoteProtocolHTTPS RemoteProtocolType = shared.RemoteProtocolHTTPS
        RemoteProtocolOther RemoteProtocolType = shared.RemoteProtocolOther
)

// TernaryValue represents yes/no/not-applicable values used in reports.
type TernaryValue string

// Supported ternary values.
const (
        TernaryValueYes           TernaryValue = "yes"
        TernaryValueNo            TernaryValue = "no"
        TernaryValueNotApplicable TernaryValue = "n/a"
)

// CommandOptions captures the configurable parameters for the audit command.
type CommandOptions struct {
        Roots       []string
        AuditReport bool
        DebugOutput bool
}

// AuditReportRow models a single CSV audit result.
type AuditReportRow struct {
        FinalRepository        string
        FolderName             string
        NameMatches            TernaryValue
        RemoteDefaultBranch    string
        LocalBranch            string
        InSync                 TernaryValue
        RemoteProtocol         RemoteProtocolType
        OriginMatchesCanonical TernaryValue
}

// CSVRecord returns the row formatted for CSV encoding.
func (row AuditReportRow) CSVRecord() []string {
        return []string{
                row.FinalRepository,
                row.FolderName,
                string(row.NameMatches),
                row.RemoteDefaultBranch,
                row.LocalBranch,
                string(row.InSync),
                string(row.RemoteProtocol),
                string(row.OriginMatchesCanonical),
        }
}

// RenamePlan describes the outcome of computing a rename action.
type RenamePlan struct {
        AlreadyNamed  bool
        DirtyWorktree bool
        ParentMissing bool
        TargetExists  bool
        CaseOnly      bool
}

// ProtocolConversionPlan captures the data necessary to adjust remote protocols.
type ProtocolConversionPlan struct {
        CurrentURL      string
        TargetURL       string
        CurrentProtocol RemoteProtocolType
        TargetProtocol  RemoteProtocolType
        OwnerRepository string
}
