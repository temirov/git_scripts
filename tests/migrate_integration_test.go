package tests

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/githubcli"
	"github.com/temirov/git_scripts/internal/gitrepo"
	migrate "github.com/temirov/git_scripts/internal/migrate"
)

const (
	integrationRepositoryIdentifierConstant  = "integration/test"
	integrationRemoteURLConstant             = "git@github.com:integration/test.git"
	integrationWorkflowDirectoryConstant     = ".github/workflows"
	integrationWorkflowFileNameConstant      = "ci.yml"
	integrationWorkflowInitialContent        = "on:\n  push:\n    branches:\n      - main\n"
	integrationWorkflowCommitMessageConstant = "CI: switch workflow branch filters to master"
)

type recordingGitHubOperations struct {
	pagesStatus             githubcli.PagesStatus
	updatedPagesConfig      *githubcli.PagesConfiguration
	defaultBranchTarget     string
	pullRequests            []githubcli.PullRequest
	retargetedPullRequests  []int
	branchProtectionEnabled bool
}

func (operations *recordingGitHubOperations) GetPagesConfig(_ context.Context, repository string) (githubcli.PagesStatus, error) {
	_ = repository
	return operations.pagesStatus, nil
}

func (operations *recordingGitHubOperations) UpdatePagesConfig(_ context.Context, repository string, configuration githubcli.PagesConfiguration) error {
	_ = repository
	operations.updatedPagesConfig = &configuration
	return nil
}

func (operations *recordingGitHubOperations) ListPullRequests(_ context.Context, repository string, options githubcli.PullRequestListOptions) ([]githubcli.PullRequest, error) {
	_ = repository
	_ = options
	return operations.pullRequests, nil
}

func (operations *recordingGitHubOperations) UpdatePullRequestBase(_ context.Context, repository string, pullRequestNumber int, baseBranch string) error {
	_ = repository
	_ = baseBranch
	operations.retargetedPullRequests = append(operations.retargetedPullRequests, pullRequestNumber)
	return nil
}

func (operations *recordingGitHubOperations) SetDefaultBranch(_ context.Context, repository string, branchName string) error {
	_ = repository
	operations.defaultBranchTarget = branchName
	return nil
}

func (operations *recordingGitHubOperations) CheckBranchProtection(_ context.Context, repository string, branchName string) (bool, error) {
	_ = repository
	_ = branchName
	return operations.branchProtectionEnabled, nil
}

func TestMigrationIntegration(testInstance *testing.T) {
	repositoryDirectory := testInstance.TempDir()

	runMigrationGitCommand(testInstance, repositoryDirectory, "init")
	runMigrationGitCommand(testInstance, repositoryDirectory, "config", "user.name", "Integration User")
	runMigrationGitCommand(testInstance, repositoryDirectory, "config", "user.email", "integration@example.com")
	runMigrationGitCommand(testInstance, repositoryDirectory, "checkout", "-b", "main")

	readmePath := filepath.Join(repositoryDirectory, "README.md")
	require.NoError(testInstance, os.WriteFile(readmePath, []byte("hello\n"), 0o644))
	runMigrationGitCommand(testInstance, repositoryDirectory, "add", "README.md")
	runMigrationGitCommand(testInstance, repositoryDirectory, "commit", "-m", "initial commit")

	runMigrationGitCommand(testInstance, repositoryDirectory, "remote", "add", "origin", integrationRemoteURLConstant)
	runMigrationGitCommand(testInstance, repositoryDirectory, "branch", "master")
	runMigrationGitCommand(testInstance, repositoryDirectory, "checkout", "master")

	workflowsDirectory := filepath.Join(repositoryDirectory, integrationWorkflowDirectoryConstant)
	require.NoError(testInstance, os.MkdirAll(workflowsDirectory, 0o755))
	workflowPath := filepath.Join(workflowsDirectory, integrationWorkflowFileNameConstant)
	require.NoError(testInstance, os.WriteFile(workflowPath, []byte(integrationWorkflowInitialContent), 0o644))
	runMigrationGitCommand(testInstance, repositoryDirectory, "add", integrationWorkflowDirectoryConstant)
	runMigrationGitCommand(testInstance, repositoryDirectory, "commit", "-m", "add workflow")

	logger := zap.NewNop()
	commandRunner := execshell.NewOSCommandRunner()
	executor, creationError := execshell.NewShellExecutor(logger, commandRunner)
	require.NoError(testInstance, creationError)
	repositoryManager, managerError := gitrepo.NewRepositoryManager(executor)
	require.NoError(testInstance, managerError)

	githubOperations := &recordingGitHubOperations{
		pagesStatus: githubcli.PagesStatus{
			Enabled:      true,
			BuildType:    githubcli.PagesBuildTypeLegacy,
			SourceBranch: "main",
			SourcePath:   "/docs",
		},
		pullRequests: []githubcli.PullRequest{{Number: 12}},
	}

	service, serviceError := migrate.NewService(migrate.ServiceDependencies{
		Logger:            logger,
		RepositoryManager: repositoryManager,
		GitHubClient:      githubOperations,
		GitExecutor:       executor,
	})
	require.NoError(testInstance, serviceError)

	options := migrate.MigrationOptions{
		RepositoryPath:       repositoryDirectory,
		RepositoryRemoteName: "origin",
		RepositoryIdentifier: integrationRepositoryIdentifierConstant,
		WorkflowsDirectory:   integrationWorkflowDirectoryConstant,
		SourceBranch:         migrate.BranchMain,
		TargetBranch:         migrate.BranchMaster,
		PushUpdates:          false,
	}

	result, migrationError := service.Execute(context.Background(), options)
	require.NoError(testInstance, migrationError)

	require.Len(testInstance, result.WorkflowOutcome.UpdatedFiles, 1)
	require.Contains(testInstance, result.WorkflowOutcome.UpdatedFiles[0], integrationWorkflowFileNameConstant)
	require.True(testInstance, result.PagesConfigurationUpdated)
	require.True(testInstance, result.DefaultBranchUpdated)
	require.ElementsMatch(testInstance, []int{12}, result.RetargetedPullRequests)
	require.False(testInstance, result.SafetyStatus.SafeToDelete)
	require.Contains(testInstance, result.SafetyStatus.BlockingReasons, "open pull requests still target source branch")

	contentBytes, readError := os.ReadFile(workflowPath)
	require.NoError(testInstance, readError)
	require.Contains(testInstance, string(contentBytes), "- master")

	logOutput := runMigrationGitCommand(testInstance, repositoryDirectory, "log", "-1", "--pretty=%s")
	require.Equal(testInstance, integrationWorkflowCommitMessageConstant, logOutput)

	statusOutput := runMigrationGitCommand(testInstance, repositoryDirectory, "status", "--porcelain")
	require.Equal(testInstance, "", statusOutput)

	require.NotNil(testInstance, githubOperations.updatedPagesConfig)
	require.Equal(testInstance, string(migrate.BranchMaster), githubOperations.updatedPagesConfig.SourceBranch)
	require.Equal(testInstance, []int{12}, githubOperations.retargetedPullRequests)
	require.Equal(testInstance, string(migrate.BranchMaster), githubOperations.defaultBranchTarget)
}

func runMigrationGitCommand(testInstance *testing.T, repositoryPath string, arguments ...string) string {
	testInstance.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = repositoryPath
	outputBytes, commandError := command.CombinedOutput()
	require.NoError(testInstance, commandError, string(outputBytes))
	return string(bytes.TrimSpace(outputBytes))
}
