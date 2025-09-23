package audit

import "time"

// RemoteProtocolType enumerates supported git remote protocols.
type RemoteProtocolType string

// Remote protocol values supported by the audit command.
const (
	RemoteProtocolGit   RemoteProtocolType = "git"
	RemoteProtocolSSH   RemoteProtocolType = "ssh"
	RemoteProtocolHTTPS RemoteProtocolType = "https"
	RemoteProtocolOther RemoteProtocolType = "other"
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
	Roots                []string
	AuditReport          bool
	RenameRepositories   bool
	UpdateRemotes        bool
	ProtocolFrom         RemoteProtocolType
	ProtocolTo           RemoteProtocolType
	DryRun               bool
	AssumeYes            bool
	RequireCleanWorktree bool
	DebugOutput          bool
	Clock                Clock
}

// Clock abstracts time-dependent functionality for deterministic testing.
type Clock interface {
	Now() time.Time
}

// SystemClock implements Clock using the standard library.
type SystemClock struct{}

// Now returns the current system time.
func (SystemClock) Now() time.Time {
	return time.Now()
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
