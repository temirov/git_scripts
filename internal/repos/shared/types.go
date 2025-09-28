package shared

import (
	"context"
	"io/fs"
	"time"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
)

const (
	// OriginRemoteNameConstant identifies the default upstream remote used for GitHub repositories.
	OriginRemoteNameConstant = "origin"
	// GitProtocolURLPrefixConstant matches git protocol remote URLs.
	GitProtocolURLPrefixConstant = "git@github.com:"
	// SSHProtocolURLPrefixConstant matches ssh protocol remote URLs.
	SSHProtocolURLPrefixConstant = "ssh://git@github.com/"
	// HTTPSProtocolURLPrefixConstant matches https protocol remote URLs.
	HTTPSProtocolURLPrefixConstant = "https://github.com/"
)

// RemoteProtocol enumerates supported git remote protocols.
type RemoteProtocol string

// Supported remote protocols.
const (
	RemoteProtocolGit   RemoteProtocol = "git"
	RemoteProtocolSSH   RemoteProtocol = "ssh"
	RemoteProtocolHTTPS RemoteProtocol = "https"
	RemoteProtocolOther RemoteProtocol = "other"
)

// Clock abstracts time acquisition for deterministic testing.
type Clock interface {
	Now() time.Time
}

// SystemClock implements Clock using the system time source.
type SystemClock struct{}

// Now returns the current system time.
func (SystemClock) Now() time.Time {
	return time.Now()
}

// FileSystem exposes filesystem operations required by repository services.
type FileSystem interface {
	Stat(path string) (fs.FileInfo, error)
	Rename(oldPath string, newPath string) error
	Abs(path string) (string, error)
}

// ConfirmationPrompter collects user confirmations prior to mutating actions.
type ConfirmationPrompter interface {
	Confirm(prompt string) (bool, error)
}

// GitExecutor exposes the subset of shell execution used by repository services.
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

// RepositoryDiscoverer locates Git repositories for bulk operations.
type RepositoryDiscoverer interface {
	DiscoverRepositories(roots []string) ([]string, error)
}
