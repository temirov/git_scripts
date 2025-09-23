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

	"github.com/temirov/git_scripts/internal/execshell"
)

// Service coordinates repository discovery, reporting, and reconciliation.
type Service struct {
	discoverer   RepositoryDiscoverer
	gitManager   GitRepositoryManager
	gitExecutor  GitExecutor
	githubClient GitHubMetadataResolver
	fileSystem   FileSystem
	prompter     ConfirmationPrompter
	outputWriter io.Writer
	errorWriter  io.Writer
	clock        Clock
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

// NewService constructs a Service using the provided dependencies.
func NewService(discoverer RepositoryDiscoverer, gitManager GitRepositoryManager, gitExecutor GitExecutor, githubClient GitHubMetadataResolver, fileSystem FileSystem, prompter ConfirmationPrompter, outputWriter io.Writer, errorWriter io.Writer, clock Clock) *Service {
	if clock == nil {
		clock = SystemClock{}
	}
	return &Service{
		discoverer:   discoverer,
		gitManager:   gitManager,
		gitExecutor:  gitExecutor,
		githubClient: githubClient,
		fileSystem:   fileSystem,
		prompter:     prompter,
		outputWriter: outputWriter,
		errorWriter:  errorWriter,
		clock:        clock,
	}
}

// Run executes the service according to the provided options.
func (service *Service) Run(executionContext context.Context, options CommandOptions) error {
	roots := options.Roots
	if len(roots) == 0 {
		roots = []string{defaultRootPathConstant}
	}

	repositories, discoveryError := service.discoverer.DiscoverRepositories(roots)
	if discoveryError != nil {
		return discoveryError
	}

	if options.DebugOutput {
		fmt.Fprintf(service.errorWriter, debugDiscoveredTemplate, len(repositories), strings.Join(roots, " "))
	}

	uniqueRepositories := deduplicatePaths(repositories)

	auditAllowed := options.AuditReport && !options.RenameRepositories && !options.UpdateRemotes && len(strings.TrimSpace(string(options.ProtocolFrom))) == 0 && len(strings.TrimSpace(string(options.ProtocolTo))) == 0

	var csvWriter *csv.Writer
	if options.AuditReport && auditAllowed {
		csvWriter = csv.NewWriter(service.outputWriter)
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
	}

	for _, repositoryPath := range uniqueRepositories {
		if options.DebugOutput {
			fmt.Fprintf(service.errorWriter, debugCheckingTemplate, repositoryPath)
		}

		if !service.isGitRepository(executionContext, repositoryPath) {
			continue
		}

		inspection, inspectionError := service.inspectRepository(executionContext, repositoryPath)
		if inspectionError != nil {
			continue
		}

		if len(inspection.OriginOwnerRepo) == 0 && len(inspection.CanonicalOwnerRepo) == 0 {
			continue
		}

		if csvWriter != nil {
			record := inspectionReportRow(inspection)
			if writeError := csvWriter.Write(record.CSVRecord()); writeError != nil {
				return writeError
			}
		}

		if options.RenameRepositories {
			service.handleRename(executionContext, inspection, options)
		}

		if options.UpdateRemotes {
			service.handleRemoteUpdate(executionContext, inspection, options)
		}

		if len(options.ProtocolFrom) > 0 && len(options.ProtocolTo) > 0 {
			service.handleProtocolConversion(executionContext, inspection, options)
		}
	}

	if csvWriter != nil {
		csvWriter.Flush()
		if flushError := csvWriter.Error(); flushError != nil {
			return flushError
		}
	}

	return nil
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

func (service *Service) inspectRepository(executionContext context.Context, repositoryPath string) (RepositoryInspection, error) {
	folderName := filepath.Base(repositoryPath)

	originURL, originError := service.gitManager.GetRemoteURL(executionContext, repositoryPath, originRemoteNameConstant)
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

	localBranch, localBranchError := service.gitManager.GetCurrentBranch(executionContext, repositoryPath)
	if localBranchError != nil {
		localBranch = ""
	}
	localBranch = sanitizeBranchName(localBranch)

	inSync := service.computeInSync(executionContext, repositoryPath, remoteDefaultBranch, localBranch, remoteProtocol)

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
		InSyncStatus:           inSync,
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

func (service *Service) handleRename(executionContext context.Context, inspection RepositoryInspection, options CommandOptions) {
	if len(inspection.DesiredFolderName) == 0 || inspection.DesiredFolderName == inspection.FolderName {
		return
	}

	oldAbsolutePath, absError := service.fileSystem.Abs(inspection.Path)
	if absError != nil {
		fmt.Fprintf(service.errorWriter, renameFailureTemplate, inspection.Path, inspection.DesiredFolderName)
		return
	}

	parentDirectory := filepath.Dir(oldAbsolutePath)
	newAbsolutePath := filepath.Join(parentDirectory, inspection.DesiredFolderName)

	if options.DryRun {
		service.printRenamePlan(executionContext, oldAbsolutePath, newAbsolutePath, options.RequireCleanWorktree)
		return
	}

	if !service.validateRenamePrerequisites(executionContext, oldAbsolutePath, newAbsolutePath, options.RequireCleanWorktree) {
		return
	}

	if !options.AssumeYes && service.prompter != nil {
		prompt := fmt.Sprintf(renamePromptTemplate, oldAbsolutePath, newAbsolutePath)
		confirmed, promptError := service.prompter.Confirm(prompt)
		if promptError != nil {
			fmt.Fprintf(service.errorWriter, renameFailureTemplate, oldAbsolutePath, newAbsolutePath)
			return
		}
		if !confirmed {
			fmt.Fprintf(service.outputWriter, renameSkipTemplate, oldAbsolutePath)
			return
		}
	}

	if service.performRename(oldAbsolutePath, newAbsolutePath) {
		fmt.Fprintf(service.outputWriter, renameSuccessTemplate, oldAbsolutePath, newAbsolutePath)
	} else {
		fmt.Fprintf(service.errorWriter, renameFailureTemplate, oldAbsolutePath, newAbsolutePath)
	}
}

func (service *Service) printRenamePlan(executionContext context.Context, oldAbsolutePath string, newAbsolutePath string, requireClean bool) {
	switch {
	case oldAbsolutePath == newAbsolutePath:
		fmt.Fprintf(service.outputWriter, renamePlanSkipAlready, oldAbsolutePath)
		return
	case requireClean && !service.isClean(executionContext, oldAbsolutePath):
		fmt.Fprintf(service.outputWriter, renamePlanSkipDirty, oldAbsolutePath)
		return
	case !service.parentExists(newAbsolutePath):
		fmt.Fprintf(service.outputWriter, renamePlanSkipParent, filepath.Dir(newAbsolutePath))
		return
	case service.targetExists(newAbsolutePath):
		fmt.Fprintf(service.outputWriter, renamePlanSkipExists, newAbsolutePath)
		return
	}

	if isCaseOnlyRename(oldAbsolutePath, newAbsolutePath) {
		fmt.Fprintf(service.outputWriter, renamePlanCaseOnlyTemplate, oldAbsolutePath, newAbsolutePath)
		return
	}
	fmt.Fprintf(service.outputWriter, renamePlanOKTemplate, oldAbsolutePath, newAbsolutePath)
}

func (service *Service) validateRenamePrerequisites(executionContext context.Context, oldAbsolutePath string, newAbsolutePath string, requireClean bool) bool {
	if oldAbsolutePath == newAbsolutePath {
		fmt.Fprintf(service.errorWriter, renameErrorAlready, oldAbsolutePath)
		return false
	}

	if requireClean && !service.isClean(executionContext, oldAbsolutePath) {
		fmt.Fprintf(service.errorWriter, renameErrorDirty, oldAbsolutePath)
		return false
	}

	if !service.parentExists(newAbsolutePath) {
		fmt.Fprintf(service.errorWriter, renameErrorParent, filepath.Dir(newAbsolutePath))
		return false
	}

	if service.targetExists(newAbsolutePath) {
		fmt.Fprintf(service.errorWriter, renameErrorExists, newAbsolutePath)
		return false
	}

	return true
}

func (service *Service) isClean(executionContext context.Context, repositoryPath string) bool {
	clean, cleanError := service.gitManager.CheckCleanWorktree(executionContext, repositoryPath)
	if cleanError != nil {
		return false
	}
	return clean
}

func (service *Service) parentExists(newAbsolutePath string) bool {
	_, statError := service.fileSystem.Stat(filepath.Dir(newAbsolutePath))
	return statError == nil
}

func (service *Service) targetExists(newAbsolutePath string) bool {
	_, statError := service.fileSystem.Stat(newAbsolutePath)
	return statError == nil
}

func (service *Service) performRename(oldAbsolutePath string, newAbsolutePath string) bool {
	if isCaseOnlyRename(oldAbsolutePath, newAbsolutePath) {
		intermediatePath := computeIntermediateRenamePath(oldAbsolutePath, service.clock.Now().UnixNano())
		if renameError := service.fileSystem.Rename(oldAbsolutePath, intermediatePath); renameError != nil {
			return false
		}
		if renameError := service.fileSystem.Rename(intermediatePath, newAbsolutePath); renameError != nil {
			_ = service.fileSystem.Rename(intermediatePath, oldAbsolutePath)
			return false
		}
		return true
	}

	if renameError := service.fileSystem.Rename(oldAbsolutePath, newAbsolutePath); renameError != nil {
		return false
	}
	return true
}

func (service *Service) handleRemoteUpdate(executionContext context.Context, inspection RepositoryInspection, options CommandOptions) {
	if len(strings.TrimSpace(inspection.OriginOwnerRepo)) == 0 {
		fmt.Fprintf(service.outputWriter, updateRemoteSkipParse, inspection.Path)
		return
	}

	if len(strings.TrimSpace(inspection.CanonicalOwnerRepo)) == 0 {
		fmt.Fprintf(service.outputWriter, updateRemoteSkipCanonical, inspection.Path)
		return
	}

	if ownerRepoCaseInsensitiveEqual(inspection.OriginOwnerRepo, inspection.CanonicalOwnerRepo) {
		fmt.Fprintf(service.outputWriter, updateRemoteSkipSame, inspection.Path)
		return
	}

	targetURL, targetError := buildRemoteURL(inspection.RemoteProtocol, inspection.CanonicalOwnerRepo)
	if targetError != nil {
		fmt.Fprintf(service.outputWriter, updateRemoteSkipTarget, inspection.Path)
		return
	}

	if options.DryRun {
		fmt.Fprintf(service.outputWriter, updateRemotePlanTemplate, inspection.Path, inspection.OriginURL, targetURL)
		return
	}

	if !options.AssumeYes && service.prompter != nil {
		prompt := fmt.Sprintf(updateRemotePromptTemplate, inspection.Path, inspection.OriginOwnerRepo, inspection.CanonicalOwnerRepo)
		confirmed, promptError := service.prompter.Confirm(prompt)
		if promptError != nil {
			fmt.Fprintf(service.outputWriter, updateRemoteSkipTarget, inspection.Path)
			return
		}
		if !confirmed {
			fmt.Fprintf(service.outputWriter, updateRemoteDeclined, inspection.Path)
			return
		}
	}

	if updateError := service.gitManager.SetRemoteURL(executionContext, inspection.Path, originRemoteNameConstant, targetURL); updateError != nil {
		fmt.Fprintf(service.outputWriter, updateRemoteFailure, inspection.Path)
		return
	}

	fmt.Fprintf(service.outputWriter, updateRemoteSuccess, inspection.Path, targetURL)
}

func (service *Service) handleProtocolConversion(executionContext context.Context, inspection RepositoryInspection, options CommandOptions) {
	currentURL, urlError := service.gitManager.GetRemoteURL(executionContext, inspection.Path, originRemoteNameConstant)
	if urlError != nil {
		return
	}

	currentProtocol := detectRemoteProtocol(currentURL)
	if currentProtocol != options.ProtocolFrom {
		return
	}

	ownerRepo := inspection.CanonicalOwnerRepo
	if len(strings.TrimSpace(ownerRepo)) == 0 {
		ownerRepo = inspection.OriginOwnerRepo
	}

	if len(strings.TrimSpace(ownerRepo)) == 0 {
		fmt.Fprintf(service.errorWriter, convertErrorOwnerRepo, inspection.Path)
		return
	}

	targetURL, targetError := buildRemoteURL(options.ProtocolTo, ownerRepo)
	if targetError != nil {
		fmt.Fprintf(service.errorWriter, convertErrorTargetURL, string(options.ProtocolTo), inspection.Path)
		return
	}

	if options.DryRun {
		fmt.Fprintf(service.outputWriter, convertPlanTemplate, inspection.Path, currentURL, targetURL)
		return
	}

	if !options.AssumeYes && service.prompter != nil {
		prompt := fmt.Sprintf(convertPromptTemplate, inspection.Path, currentProtocol, options.ProtocolTo)
		confirmed, promptError := service.prompter.Confirm(prompt)
		if promptError != nil {
			fmt.Fprintf(service.errorWriter, convertFailureTemplate, targetURL, inspection.Path)
			return
		}
		if !confirmed {
			fmt.Fprintf(service.outputWriter, convertDeclinedTemplate, inspection.Path)
			return
		}
	}

	if updateError := service.gitManager.SetRemoteURL(executionContext, inspection.Path, originRemoteNameConstant, targetURL); updateError != nil {
		fmt.Fprintf(service.errorWriter, convertFailureTemplate, targetURL, inspection.Path)
		return
	}

	fmt.Fprintf(service.outputWriter, convertSuccessTemplate, inspection.Path, targetURL)
}
