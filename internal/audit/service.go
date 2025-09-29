package audit

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/repos/shared"
)

// Service coordinates repository discovery, reporting, and reconciliation.
type Service struct {
	discoverer   RepositoryDiscoverer
	gitManager   GitRepositoryManager
	gitExecutor  GitExecutor
	githubClient GitHubMetadataResolver
	outputWriter io.Writer
	errorWriter  io.Writer
}

// NewService constructs a Service using the provided dependencies.
func NewService(discoverer RepositoryDiscoverer, gitManager GitRepositoryManager, gitExecutor GitExecutor, githubClient GitHubMetadataResolver, outputWriter io.Writer, errorWriter io.Writer) *Service {
	return &Service{
		discoverer:   discoverer,
		gitManager:   gitManager,
		gitExecutor:  gitExecutor,
		githubClient: githubClient,
		outputWriter: outputWriter,
		errorWriter:  errorWriter,
	}
}

// Run executes the service according to the provided options.
func (service *Service) Run(executionContext context.Context, options CommandOptions) error {
	roots := options.Roots
	if len(roots) == 0 {
		return errors.New(missingRootsErrorMessageConstant)
	}

	inspections, inspectionError := service.DiscoverInspections(executionContext, roots, options.DebugOutput, options.InspectionDepth)
	if inspectionError != nil {
		return inspectionError
	}

	return service.writeAuditReport(inspections)
}

// DiscoverInspections collects repository inspections for the provided roots.
func (service *Service) DiscoverInspections(executionContext context.Context, roots []string, debug bool, inspectionDepth InspectionDepth) ([]RepositoryInspection, error) {
	normalizedDepth := normalizeInspectionDepth(inspectionDepth)

	repositories, discoveryError := service.discoverer.DiscoverRepositories(roots)
	if discoveryError != nil {
		return nil, discoveryError
	}

	if debug {
		fmt.Fprintf(service.errorWriter, debugDiscoveredTemplate, len(repositories), strings.Join(roots, " "))
	}

	uniqueRepositories := deduplicatePaths(repositories)
	inspections := make([]RepositoryInspection, 0, len(uniqueRepositories))

	for _, repositoryPath := range uniqueRepositories {
		if debug {
			fmt.Fprintf(service.errorWriter, debugCheckingTemplate, repositoryPath)
		}

		if !service.isGitRepository(executionContext, repositoryPath) {
			continue
		}

		inspection, inspectError := service.inspectRepository(executionContext, repositoryPath, normalizedDepth)
		if inspectError != nil {
			continue
		}

		if len(inspection.OriginOwnerRepo) == 0 && len(inspection.CanonicalOwnerRepo) == 0 {
			continue
		}

		inspections = append(inspections, inspection)
	}

	return inspections, nil
}

func (service *Service) writeAuditReport(inspections []RepositoryInspection) error {
	csvWriter := csv.NewWriter(service.outputWriter)
	header := []string{
		csvHeaderFinalRepository,
		csvHeaderFolderName,
		csvHeaderNameMatches,
		csvHeaderRemoteDefault,
		csvHeaderLocalBranch,
		csvHeaderInSync,
		csvHeaderRemoteProtocol,
		csvHeaderOriginCanonical,
	}
	if writeError := csvWriter.Write(header); writeError != nil {
		return writeError
	}

	for inspectionIndex := range inspections {
		record := inspectionReportRow(inspections[inspectionIndex])
		if writeError := csvWriter.Write(record.CSVRecord()); writeError != nil {
			return writeError
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

func deduplicatePaths(paths []string) []string {
	seen := make(map[string]struct{})
	var unique []string
	for _, path := range paths {
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	sort.Strings(unique)
	return unique
}

func normalizeInspectionDepth(depth InspectionDepth) InspectionDepth {
	switch depth {
	case InspectionDepthMinimal:
		return InspectionDepthMinimal
	default:
		return InspectionDepthFull
	}
}

func (service *Service) isGitRepository(executionContext context.Context, repositoryPath string) bool {
	commandDetails := execshell.CommandDetails{
		Arguments:        []string{gitRevParseSubcommandConstant, gitIsInsideWorkTreeFlagConstant},
		WorkingDirectory: repositoryPath,
	}

	executionResult, executionError := service.gitExecutor.ExecuteGit(executionContext, commandDetails)
	if executionError != nil {
		return false
	}

	return strings.TrimSpace(executionResult.StandardOutput) == gitTrueOutputConstant
}

func (service *Service) inspectRepository(executionContext context.Context, repositoryPath string, inspectionDepth InspectionDepth) (RepositoryInspection, error) {
	folderName := filepath.Base(repositoryPath)

	originURL, originError := service.gitManager.GetRemoteURL(executionContext, repositoryPath, shared.OriginRemoteNameConstant)
	if originError != nil {
		return RepositoryInspection{}, originError
	}

	if !strings.Contains(strings.ToLower(originURL), githubHostConstant) {
		return RepositoryInspection{}, errors.New(notGitHubRemoteMessageConstant)
	}

	originOwnerRepo, ownerError := canonicalizeOwnerRepo(originURL)
	if ownerError != nil {
		originOwnerRepo = ""
	}

	remoteProtocol := detectRemoteProtocol(originURL)

	canonicalOwnerRepo := ""
	remoteDefaultBranch := ""
	metadata, metadataError := service.githubClient.ResolveRepoMetadata(executionContext, originOwnerRepo)
	if metadataError == nil {
		canonicalOwnerRepo = strings.TrimSpace(metadata.NameWithOwner)
		remoteDefaultBranch = strings.TrimSpace(metadata.DefaultBranch)
	}

	if len(remoteDefaultBranch) == 0 {
		remoteDefaultBranch = service.resolveDefaultBranchFromGit(executionContext, repositoryPath)
	}

	localBranch := ""
	inSyncStatus := TernaryValueNotApplicable
	if inspectionDepth == InspectionDepthFull {
		branchName, localBranchError := service.gitManager.GetCurrentBranch(executionContext, repositoryPath)
		if localBranchError == nil {
			sanitizedBranch := sanitizeBranchName(branchName)
			localBranch = sanitizedBranch
			inSyncStatus = service.computeInSync(executionContext, repositoryPath, remoteDefaultBranch, sanitizedBranch, remoteProtocol)
		}
	}

	finalOwnerRepo := originOwnerRepo
	if len(strings.TrimSpace(canonicalOwnerRepo)) > 0 {
		finalOwnerRepo = canonicalOwnerRepo
	}

	inspection := RepositoryInspection{
		Path:                   repositoryPath,
		FolderName:             folderName,
		OriginURL:              originURL,
		OriginOwnerRepo:        originOwnerRepo,
		CanonicalOwnerRepo:     canonicalOwnerRepo,
		FinalOwnerRepo:         finalOwnerRepo,
		DesiredFolderName:      finalRepositoryName(finalOwnerRepo),
		RemoteProtocol:         remoteProtocol,
		RemoteDefaultBranch:    remoteDefaultBranch,
		LocalBranch:            localBranch,
		InSyncStatus:           inSyncStatus,
		OriginMatchesCanonical: matchesCanonical(originOwnerRepo, canonicalOwnerRepo),
	}
	return inspection, nil
}

func matchesCanonical(origin string, canonical string) TernaryValue {
	if len(strings.TrimSpace(origin)) == 0 || len(strings.TrimSpace(canonical)) == 0 {
		return TernaryValueNotApplicable
	}
	if ownerRepoCaseInsensitiveEqual(origin, canonical) {
		return TernaryValueYes
	}
	return TernaryValueNo
}

func inspectionReportRow(inspection RepositoryInspection) AuditReportRow {
	finalRepo := inspection.CanonicalOwnerRepo
	if len(strings.TrimSpace(finalRepo)) == 0 {
		finalRepo = inspection.OriginOwnerRepo
	}
	nameMatches := TernaryValueNo
	if len(inspection.DesiredFolderName) > 0 && inspection.DesiredFolderName == inspection.FolderName {
		nameMatches = TernaryValueYes
	}
	return AuditReportRow{
		FinalRepository:        finalRepo,
		FolderName:             inspection.FolderName,
		NameMatches:            nameMatches,
		RemoteDefaultBranch:    inspection.RemoteDefaultBranch,
		LocalBranch:            inspection.LocalBranch,
		InSync:                 inspection.InSyncStatus,
		RemoteProtocol:         inspection.RemoteProtocol,
		OriginMatchesCanonical: inspection.OriginMatchesCanonical,
	}
}

func (service *Service) resolveDefaultBranchFromGit(executionContext context.Context, repositoryPath string) string {
	commandDetails := execshell.CommandDetails{
		Arguments:        lsRemoteHeadArguments(),
		WorkingDirectory: repositoryPath,
	}

	executionResult, executionError := service.gitExecutor.ExecuteGit(executionContext, commandDetails)
	if executionError != nil {
		return ""
	}

	lines := strings.Split(executionResult.StandardOutput, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "ref:") {
			continue
		}
		components := strings.Split(line, gitReferenceSeparator)
		if len(components) < 1 {
			continue
		}
		referenceParts := strings.Fields(components[0])
		if len(referenceParts) < 2 {
			continue
		}
		reference := referenceParts[1]
		return strings.TrimPrefix(reference, refsHeadsPrefixConstant)
	}

	return ""
}

func (service *Service) computeInSync(executionContext context.Context, repositoryPath string, remoteDefaultBranch string, localBranch string, protocol RemoteProtocolType) TernaryValue {
	if len(remoteDefaultBranch) == 0 || len(localBranch) == 0 || !strings.EqualFold(remoteDefaultBranch, localBranch) {
		return TernaryValueNotApplicable
	}

	if protocol != RemoteProtocolGit && protocol != RemoteProtocolSSH {
		return TernaryValueNotApplicable
	}

	fetchDetails := execshell.CommandDetails{
		Arguments:        remoteFetchArguments(remoteDefaultBranch),
		WorkingDirectory: repositoryPath,
	}

	if _, fetchError := service.gitExecutor.ExecuteGit(executionContext, fetchDetails); fetchError != nil {
		return TernaryValueNotApplicable
	}

	upstreamRef := service.resolveUpstreamReference(executionContext, repositoryPath)

	headRevision, headError := service.resolveRevision(executionContext, repositoryPath, headRevisionArguments())
	if headError != nil {
		return TernaryValueNotApplicable
	}

	remoteRevision := service.resolveRemoteRevision(executionContext, repositoryPath, upstreamRef, remoteDefaultBranch)
	if len(remoteRevision) == 0 {
		return TernaryValueNotApplicable
	}

	if headRevision == remoteRevision {
		return TernaryValueYes
	}
	return TernaryValueNo
}

func (service *Service) resolveUpstreamReference(executionContext context.Context, repositoryPath string) string {
	upstreamDetails := execshell.CommandDetails{
		Arguments:        upstreamReferenceArguments(),
		WorkingDirectory: repositoryPath,
	}

	executionResult, executionError := service.gitExecutor.ExecuteGit(executionContext, upstreamDetails)
	if executionError != nil {
		return ""
	}
	return strings.TrimSpace(executionResult.StandardOutput)
}

func (service *Service) resolveRevision(executionContext context.Context, repositoryPath string, arguments []string) (string, error) {
	commandDetails := execshell.CommandDetails{
		Arguments:        arguments,
		WorkingDirectory: repositoryPath,
	}
	executionResult, executionError := service.gitExecutor.ExecuteGit(executionContext, commandDetails)
	if executionError != nil {
		return "", executionError
	}
	return strings.TrimSpace(executionResult.StandardOutput), nil
}

func (service *Service) resolveRemoteRevision(executionContext context.Context, repositoryPath string, upstreamRef string, branch string) string {
	if len(strings.TrimSpace(upstreamRef)) > 0 {
		revision, revisionError := service.resolveRevision(executionContext, repositoryPath, revisionArguments(upstreamRef))
		if revisionError == nil && len(revision) > 0 {
			return revision
		}
	}

	for _, reference := range fallbackRemoteRevisionReferences(branch) {
		revision, revisionError := service.resolveRevision(executionContext, repositoryPath, revisionArguments(reference))
		if revisionError == nil && len(revision) > 0 {
			return revision
		}
	}

	return ""
}
