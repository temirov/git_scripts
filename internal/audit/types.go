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

// RepositoryInspection captures gathered repository state.
type RepositoryInspection struct {
	Path                   string
	FolderName             string
	OriginURL              string
	OriginOwnerRepo        string
	CanonicalOwnerRepo     string
	FinalOwnerRepo         string
	DesiredFolderName      string
	RemoteProtocol         RemoteProtocolType
	RemoteDefaultBranch    string
	LocalBranch            string
	InSyncStatus           TernaryValue
	OriginMatchesCanonical TernaryValue
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
