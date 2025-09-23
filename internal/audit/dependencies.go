package audit

import (
	"context"
	"io/fs"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
)

// RepositoryDiscoverer finds git repositories rooted under the provided paths.
type RepositoryDiscoverer interface {
	DiscoverRepositories(roots []string) ([]string, error)
}

// GitExecutor exposes the subset of shell execution used by the audit command.
type GitExecutor interface {
	ExecuteGit(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error)
	ExecuteGitHubCLI(executionContext context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error)
}

// GitRepositoryManager exposes repository-level git operations.
type GitRepositoryManager interface {
	CheckCleanWorktree(executionContext context.Context, repositoryPath string) (bool, error)
	GetCurrentBranch(executionContext context.Context, repositoryPath string) (string, error)
	GetRemoteURL(executionContext context.Context, repositoryPath string, remoteName string) (string, error)
	SetRemoteURL(executionContext context.Context, repositoryPath string, remoteName string, remoteURL string) error
}

// GitHubMetadataResolver resolves canonical repository metadata via GitHub CLI.
type GitHubMetadataResolver interface {
	ResolveRepoMetadata(executionContext context.Context, repository string) (githubcli.RepositoryMetadata, error)
}

// ConfirmationPrompter prompts users for confirmation during mutable operations.
type ConfirmationPrompter interface {
	Confirm(prompt string) (bool, error)
}

// FileSystem provides filesystem operations required by the audit workflows.
type FileSystem interface {
	Stat(path string) (fs.FileInfo, error)
	Rename(oldPath string, newPath string) error
	Abs(path string) (string, error)
}
