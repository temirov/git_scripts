package history

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/repos/shared"
)

const (
	gitIgnoreFileName              = ".gitignore"
	gitCommitMessage               = "chore: ignore purged paths"
	planMessageTemplate            = "PLAN-HISTORY-PURGE: %s paths=%s remote=%s push=%t restore=%t push_missing=%t\n"
	skipMessageTemplate            = "HISTORY-SKIP: %s (no matching history for %s)\n"
	successMessageTemplate         = "HISTORY-PURGE: %s removed=%s remote=%s push=%t restore=%t push_missing=%t\n"
	pathsRequiredErrorMessage      = "history purge requires at least one path"
	gitFilterRepoSubcommand        = "filter-repo"
	gitRemoteSubcommand            = "remote"
	gitRemoteAddSubcommand         = "add"
	gitPushSubcommand              = "push"
	gitForceFlag                   = "--force"
	gitAllFlag                     = "--all"
	gitTagsFlag                    = "--tags"
	gitFetchSubcommand             = "fetch"
	gitPruneFlag                   = "--prune"
	gitForEachRefSubcommand        = "for-each-ref"
	gitFormatFlag                  = "--format=%(refname)"
	gitUpstreamFormatFlag          = "--format=%(upstream:short)"
	gitRefsHeadsPrefix             = "refs/heads/"
	gitRefsRemotesPrefix           = "refs/remotes/"
	gitUpdateRefSubcommand         = "update-ref"
	gitDeleteFlag                  = "-d"
	gitReflogExpireSubcommand      = "reflog"
	gitReflogExpireCommand         = "expire"
	gitReflogExpireNowFlag         = "--expire=now"
	gitReflogExpireUnreachableFlag = "--expire-unreachable=now"
	gitReflogExpireAllFlag         = "--all"
	gitGCSubcommand                = "gc"
	gitGCPruneNowFlag              = "--prune=now"
	gitGCAggressiveFlag            = "--aggressive"
	gitShowRefSubcommand           = "show-ref"
	gitQuietFlag                   = "--quiet"
	gitBranchSubcommand            = "branch"
	gitSetUpstreamFlag             = "--set-upstream-to"
	gitLfsSubcommand               = "lfs"
	gitLfsPruneSubcommand          = "prune"
	gitRevParseSubcommand          = "rev-parse"
	gitRevParseUpstreamArg         = "--abbrev-ref"
	gitRevParseSymbolicArg         = "--symbolic-full-name"
	gitRevParseUpstreamReference   = "@{u}"
)

// Dependencies captures collaborators required to purge repository history.
type Dependencies struct {
	GitExecutor       shared.GitExecutor
	RepositoryManager shared.GitRepositoryManager
	FileSystem        shared.FileSystem
	Output            io.Writer
}

// Options configures the history purge workflow.
type Options struct {
	RepositoryPath string
	Paths          []string
	RemoteName     string
	Push           bool
	Restore        bool
	PushMissing    bool
	DryRun         bool
}

// Executor orchestrates history removal using git-filter-repo.
type Executor struct {
	dependencies Dependencies
}

// NewExecutor constructs an Executor using the provided dependencies.
func NewExecutor(dependencies Dependencies) Executor {
	return Executor{dependencies: dependencies}
}

// Execute rewrites repository history according to the provided options.
func (executor Executor) Execute(ctx context.Context, options Options) error {
	if executor.dependencies.GitExecutor == nil || executor.dependencies.RepositoryManager == nil || executor.dependencies.FileSystem == nil {
		return errors.New("history purge requires git executor, repository manager, and filesystem")
	}

	repositoryPath := strings.TrimSpace(options.RepositoryPath)
	if len(repositoryPath) == 0 {
		return errors.New("repository path required")
	}

	paths := normalizePaths(options.Paths)
	if len(paths) == 0 {
		return errors.New(pathsRequiredErrorMessage)
	}

	remoteName, savedRemoteURL := executor.prepareRemote(ctx, repositoryPath, strings.TrimSpace(options.RemoteName))
	joinedPaths := strings.Join(paths, ",")

	if options.DryRun {
		executor.printf(planMessageTemplate, repositoryPath, joinedPaths, remoteName, options.Push, options.Restore, options.PushMissing)
		return nil
	}

	if err := executor.ensureGitIgnore(ctx, repositoryPath, paths); err != nil {
		return err
	}

	hasHistory, historyError := executor.pathsInHistory(ctx, repositoryPath, paths)
	if historyError != nil {
		return historyError
	}
	if !hasHistory {
		executor.printf(skipMessageTemplate, repositoryPath, joinedPaths)
		return nil
	}

	if err := executor.runFilterRepo(ctx, repositoryPath, paths); err != nil {
		return err
	}

	if err := executor.cleanupFilterRepo(ctx, repositoryPath); err != nil {
		return err
	}

	executor.restoreRemote(ctx, repositoryPath, remoteName, savedRemoteURL)

	if options.Push && len(strings.TrimSpace(remoteName)) > 0 {
		if err := executor.forcePush(ctx, repositoryPath, remoteName); err != nil {
			return err
		}
	}

	if options.Restore && len(strings.TrimSpace(remoteName)) > 0 {
		if err := executor.restoreUpstreams(ctx, repositoryPath, remoteName, options.PushMissing); err != nil {
			return err
		}
	}

	executor.printf(successMessageTemplate, repositoryPath, joinedPaths, remoteName, options.Push, options.Restore, options.PushMissing)
	return nil
}

func (executor Executor) ensureGitIgnore(ctx context.Context, repositoryPath string, paths []string) error {
	filePath := filepath.Join(repositoryPath, gitIgnoreFileName)
	existingContent, readError := executor.dependencies.FileSystem.ReadFile(filePath)
	if readError != nil && !errors.Is(readError, fs.ErrNotExist) {
		return readError
	}

	lineSet := map[string]struct{}{}
	orderedLines := make([]string, 0)
	updated := false

	if len(existingContent) > 0 {
		for _, line := range strings.Split(string(existingContent), "\n") {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) == 0 {
				continue
			}
			if _, exists := lineSet[trimmed]; exists {
				continue
			}
			lineSet[trimmed] = struct{}{}
			orderedLines = append(orderedLines, trimmed)
		}
	}

	for _, pathEntry := range paths {
		if _, exists := lineSet[pathEntry]; exists {
			continue
		}
		lineSet[pathEntry] = struct{}{}
		orderedLines = append(orderedLines, pathEntry)
		updated = true
	}

	if !updated {
		return nil
	}

	contents := strings.Join(orderedLines, "\n") + "\n"

	if writeError := executor.dependencies.FileSystem.WriteFile(filePath, []byte(contents), 0o644); writeError != nil {
		return writeError
	}

	if _, err := executor.executeGit(ctx, repositoryPath, "add", gitIgnoreFileName); err != nil {
		return err
	}
	_, _ = executor.executeGit(ctx, repositoryPath, "commit", "-m", gitCommitMessage)

	return nil
}

func (executor Executor) pathsInHistory(ctx context.Context, repositoryPath string, paths []string) (bool, error) {
	for _, pathEntry := range paths {
		_, err := executor.executeGit(ctx, repositoryPath, "rev-list", "--quiet", "--all", "--", pathEntry)
		if err == nil {
			return true, nil
		}
		var commandFailed execshell.CommandFailedError
		if errors.As(err, &commandFailed) {
			continue
		}
		return false, err
	}
	return false, nil
}

func (executor Executor) runFilterRepo(ctx context.Context, repositoryPath string, paths []string) error {
	commandArguments := []string{gitFilterRepoSubcommand}
	for _, pathEntry := range paths {
		commandArguments = append(commandArguments, "--path", pathEntry)
	}
	commandArguments = append(commandArguments, "--invert-paths", "--prune-empty", "always", "--force")
	_, err := executor.executeGit(ctx, repositoryPath, commandArguments...)
	return err
}

func (executor Executor) cleanupFilterRepo(ctx context.Context, repositoryPath string) error {
	listResult, err := executor.executeGit(ctx, repositoryPath, gitForEachRefSubcommand, gitFormatFlag, "refs/filter-repo/")
	if err != nil {
		var commandFailed execshell.CommandFailedError
		if errors.As(err, &commandFailed) {
			// treat empty refs as success
			return executor.postRewriteHousekeeping(ctx, repositoryPath)
		}
		return err
	}
	refLines := strings.Split(strings.TrimSpace(listResult.StandardOutput), "\n")
	for _, ref := range refLines {
		trimmed := strings.TrimSpace(ref)
		if len(trimmed) == 0 {
			continue
		}
		if _, updateErr := executor.executeGit(ctx, repositoryPath, gitUpdateRefSubcommand, gitDeleteFlag, trimmed); updateErr != nil {
			return updateErr
		}
	}
	return executor.postRewriteHousekeeping(ctx, repositoryPath)
}

func (executor Executor) postRewriteHousekeeping(ctx context.Context, repositoryPath string) error {
	if _, err := executor.executeGit(ctx, repositoryPath, gitReflogExpireSubcommand, gitReflogExpireCommand, gitReflogExpireNowFlag, gitReflogExpireUnreachableFlag, gitReflogExpireAllFlag); err != nil {
		return err
	}
	if _, err := executor.executeGit(ctx, repositoryPath, gitGCSubcommand, gitGCPruneNowFlag, gitGCAggressiveFlag); err != nil {
		return err
	}
	_, _ = executor.executeGit(ctx, repositoryPath, gitLfsSubcommand, gitLfsPruneSubcommand)
	return nil
}

func (executor Executor) restoreRemote(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) {
	if len(strings.TrimSpace(remoteURL)) == 0 || len(strings.TrimSpace(remoteName)) == 0 {
		return
	}

	remoteList, err := executor.executeGit(ctx, repositoryPath, gitRemoteSubcommand)
	if err == nil {
		for _, existing := range strings.Split(strings.TrimSpace(remoteList.StandardOutput), "\n") {
			if strings.TrimSpace(existing) == remoteName {
				return
			}
		}
	}

	_, _ = executor.executeGit(ctx, repositoryPath, gitRemoteSubcommand, gitRemoteAddSubcommand, remoteName, remoteURL)
}

func (executor Executor) forcePush(ctx context.Context, repositoryPath string, remoteName string) error {
	if _, err := executor.executeGit(ctx, repositoryPath, gitPushSubcommand, gitForceFlag, gitAllFlag, remoteName); err != nil {
		return err
	}
	if _, err := executor.executeGit(ctx, repositoryPath, gitPushSubcommand, gitForceFlag, gitTagsFlag, remoteName); err != nil {
		return err
	}
	return nil
}

func (executor Executor) restoreUpstreams(ctx context.Context, repositoryPath string, remoteName string, pushMissing bool) error {
	if _, err := executor.executeGit(ctx, repositoryPath, gitFetchSubcommand, gitPruneFlag, remoteName); err != nil {
		return err
	}

	listResult, err := executor.executeGit(ctx, repositoryPath, gitForEachRefSubcommand, gitFormatFlag, gitRefsHeadsPrefix)
	if err != nil {
		return err
	}

	branches := strings.Split(strings.TrimSpace(listResult.StandardOutput), "\n")
	for _, branchRef := range branches {
		branch := strings.TrimPrefix(strings.TrimSpace(branchRef), gitRefsHeadsPrefix)
		if len(branch) == 0 {
			continue
		}
		if err := executor.attachUpstream(ctx, repositoryPath, branch, remoteName, pushMissing); err != nil {
			return err
		}
	}
	return nil
}

func (executor Executor) attachUpstream(ctx context.Context, repositoryPath string, branch string, remoteName string, pushMissing bool) error {
	upstreamResult, err := executor.executeGit(ctx, repositoryPath, gitForEachRefSubcommand, gitUpstreamFormatFlag, gitRefsHeadsPrefix+branch)
	if err != nil {
		return err
	}
	currentUpstream := strings.TrimSpace(upstreamResult.StandardOutput)
	desiredUpstream := fmt.Sprintf("%s/%s", remoteName, branch)

	if executor.remoteBranchExists(ctx, repositoryPath, remoteName, branch) {
		if currentUpstream == desiredUpstream {
			return nil
		}
		_, setError := executor.executeGit(ctx, repositoryPath, gitBranchSubcommand, gitSetUpstreamFlag+"="+desiredUpstream, branch)
		return setError
	}

	if !pushMissing {
		return nil
	}
	_, pushErr := executor.executeGit(ctx, repositoryPath, gitPushSubcommand, "-u", remoteName, fmt.Sprintf("%s:%s", branch, branch))
	return pushErr
}

func (executor Executor) remoteBranchExists(ctx context.Context, repositoryPath string, remoteName string, branch string) bool {
	ref := fmt.Sprintf("%s%s/%s", gitRefsRemotesPrefix, remoteName, branch)
	_, err := executor.executeGit(ctx, repositoryPath, gitShowRefSubcommand, gitQuietFlag, ref)
	var commandFailed execshell.CommandFailedError
	if errors.As(err, &commandFailed) {
		return false
	}
	return err == nil
}

func (executor Executor) prepareRemote(ctx context.Context, repositoryPath string, requestedRemote string) (string, string) {
	selectedRemote := requestedRemote
	if len(strings.TrimSpace(selectedRemote)) == 0 {
		selectedRemote = executor.detectRemote(ctx, repositoryPath)
	}
	if len(strings.TrimSpace(selectedRemote)) == 0 {
		selectedRemote = shared.OriginRemoteNameConstant
	}

	remoteURL, err := executor.dependencies.RepositoryManager.GetRemoteURL(ctx, repositoryPath, selectedRemote)
	if err != nil {
		return selectedRemote, ""
	}
	return selectedRemote, remoteURL
}

func (executor Executor) detectRemote(ctx context.Context, repositoryPath string) string {
	result, err := executor.executeGit(ctx, repositoryPath, gitRevParseSubcommand, gitRevParseUpstreamArg, gitRevParseSymbolicArg, gitRevParseUpstreamReference)
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(result.StandardOutput)
	if len(trimmed) == 0 || !strings.Contains(trimmed, "/") {
		return ""
	}
	return strings.Split(trimmed, "/")[0]
}

func (executor Executor) executeGit(ctx context.Context, repositoryPath string, arguments ...string) (execshell.ExecutionResult, error) {
	details := execshell.CommandDetails{
		Arguments:        arguments,
		WorkingDirectory: repositoryPath,
	}
	return executor.dependencies.GitExecutor.ExecuteGit(ctx, details)
}

func (executor Executor) printf(format string, values ...any) {
	if executor.dependencies.Output == nil {
		return
	}
	fmt.Fprintf(executor.dependencies.Output, format, values...)
}

func normalizePaths(entries []string) []string {
	unique := make(map[string]struct{})
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if len(trimmed) == 0 {
			continue
		}
		cleaned := strings.TrimPrefix(trimmed, "./")
		if len(cleaned) == 0 {
			continue
		}
		unique[cleaned] = struct{}{}
	}

	results := make([]string, 0, len(unique))
	for value := range unique {
		results = append(results, value)
	}
	sort.Strings(results)
	return results
}
