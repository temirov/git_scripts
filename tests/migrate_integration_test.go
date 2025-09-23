package tests

import (
	"bytes"
	"context"
	"fmt"
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
	migrationDefaultCaseNameConstant         = "main_to_master"
	migrationSubtestNameTemplateConstant     = "%d_%s"
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
	testCases := []struct {
		name string
	}{
		{name: migrationDefaultCaseNameConstant},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(migrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(subtest *testing.T) {
			repositoryDirectory := subtest.TempDir()

			runMigrationGitCommand(subtest, repositoryDirectory, "init")
			runMigrationGitCommand(subtest, repositoryDirectory, "config", "user.name", "Integration User")
			runMigrationGitCommand(subtest, repositoryDirectory, "config", "user.email", "integration@example.com")
			runMigrationGitCommand(subtest, repositoryDirectory, "checkout", "-b", "main")

			readmePath := filepath.Join(repositoryDirectory, "README.md")
			require.NoError(subtest, os.WriteFile(readmePath, []byte("hello\n"), 0o644))
			runMigrationGitCommand(subtest, repositoryDirectory, "add", "README.md")
			runMigrationGitCommand(subtest, repositoryDirectory, "commit", "-m", "initial commit")

			runMigrationGitCommand(subtest, repositoryDirectory, "remote", "add", "origin", integrationRemoteURLConstant)
			runMigrationGitCommand(subtest, repositoryDirectory, "branch", "master")
			runMigrationGitCommand(subtest, repositoryDirectory, "checkout", "master")

			workflowsDirectory := filepath.Join(repositoryDirectory, integrationWorkflowDirectoryConstant)
			require.NoError(subtest, os.MkdirAll(workflowsDirectory, 0o755))
			workflowPath := filepath.Join(workflowsDirectory, integrationWorkflowFileNameConstant)
			require.NoError(subtest, os.WriteFile(workflowPath, []byte(integrationWorkflowInitialContent), 0o644))
			runMigrationGitCommand(subtest, repositoryDirectory, "add", integrationWorkflowDirectoryConstant)
			runMigrationGitCommand(subtest, repositoryDirectory, "commit", "-m", "add workflow")

			logger := zap.NewNop()
			commandRunner := execshell.NewOSCommandRunner()
			executor, creationError := execshell.NewShellExecutor(logger, commandRunner)
			require.NoError(subtest, creationError)
			repositoryManager, managerError := gitrepo.NewRepositoryManager(executor)
			require.NoError(subtest, managerError)

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
			require.NoError(subtest, serviceError)

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
			require.NoError(subtest, migrationError)

			require.Len(subtest, result.WorkflowOutcome.UpdatedFiles, 1)
			require.Contains(subtest, result.WorkflowOutcome.UpdatedFiles[0], integrationWorkflowFileNameConstant)
			require.True(subtest, result.PagesConfigurationUpdated)
			require.True(subtest, result.DefaultBranchUpdated)
			require.ElementsMatch(subtest, []int{12}, result.RetargetedPullRequests)
			require.False(subtest, result.SafetyStatus.SafeToDelete)
			require.Contains(subtest, result.SafetyStatus.BlockingReasons, "open pull requests still target source branch")

			contentBytes, readError := os.ReadFile(workflowPath)
			require.NoError(subtest, readError)
			require.Contains(subtest, string(contentBytes), "- master")

			logOutput := runMigrationGitCommand(subtest, repositoryDirectory, "log", "-1", "--pretty=%s")
			require.Equal(subtest, integrationWorkflowCommitMessageConstant, logOutput)

			statusOutput := runMigrationGitCommand(subtest, repositoryDirectory, "status", "--porcelain")
			require.Equal(subtest, "", statusOutput)

			require.NotNil(subtest, githubOperations.updatedPagesConfig)
			require.Equal(subtest, string(migrate.BranchMaster), githubOperations.updatedPagesConfig.SourceBranch)
			require.Equal(subtest, []int{12}, githubOperations.retargetedPullRequests)
			require.Equal(subtest, string(migrate.BranchMaster), githubOperations.defaultBranchTarget)
		})
	}
}

func runMigrationGitCommand(testInstance *testing.T, repositoryPath string, arguments ...string) string {
	testInstance.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = repositoryPath
	outputBytes, commandError := command.CombinedOutput()
	require.NoError(testInstance, commandError, string(outputBytes))
	return string(bytes.TrimSpace(outputBytes))
}
