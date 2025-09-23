package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/branches"
	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	integrationRemoteDirectoryNameConstant        = "remote.git"
	integrationLocalDirectoryNameConstant         = "workspace"
	integrationGitExecutableNameConstant          = "git"
	integrationGHExecutableNameConstant           = "gh"
	integrationFakeGHDirectoryNameConstant        = "fake_gh"
	integrationInitialFileNameConstant            = "initial.txt"
	integrationInitialFileContentsConstant        = "initial commit contents\n"
	integrationUpdatedFileContentsConstant        = "updated contents\n"
	integrationInitialCommitMessageConstant       = "Initial commit"
	integrationFeatureDeleteCommitMessageConstant = "Feature delete changes"
	integrationFeatureSkipCommitMessageConstant   = "Feature skip changes"
	integrationUserNameConstant                   = "Integration Tester"
	integrationUserEmailConstant                  = "tester@example.com"
	integrationMainBranchNameConstant             = "main"
	integrationFeatureDeleteBranchConstant        = "feature/delete"
	integrationFeatureSkipBranchConstant          = "feature/skip"
	integrationFeatureMissingBranchConstant       = "feature/missing"
	integrationRemoteNameConstant                 = "origin"
	integrationPullRequestLimitConstant           = 100
	prCleanupCommandTimeoutConstant               = 10 * time.Second
	integrationFakeGHPayloadConstant              = "[{\"headRefName\":\"feature/delete\"},{\"headRefName\":\"feature/missing\"}]"
	integrationFakeGHScriptTemplateConstant       = "#!/bin/sh\ncat <<'JSON'\n%s\nJSON\n"
	integrationExpectationMessageTemplateConstant = "expected branch state: %s"
)

func TestPullRequestCleanupIntegration(testInstance *testing.T) {
	temporaryRoot := testInstance.TempDir()
	remoteRepositoryPath := filepath.Join(temporaryRoot, integrationRemoteDirectoryNameConstant)
	localRepositoryPath := filepath.Join(temporaryRoot, integrationLocalDirectoryNameConstant)

	runGitCommand(testInstance, temporaryRoot, []string{integrationGitExecutableNameConstant, "init", "--bare", remoteRepositoryPath})

	runGitCommand(testInstance, temporaryRoot, []string{integrationGitExecutableNameConstant, "init", localRepositoryPath})
	configureLocalRepository(testInstance, localRepositoryPath)

	initialFilePath := filepath.Join(localRepositoryPath, integrationInitialFileNameConstant)
	writeFile(testInstance, initialFilePath, integrationInitialFileContentsConstant)

	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "add", integrationInitialFileNameConstant})
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "commit", "-m", integrationInitialCommitMessageConstant})
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "branch", "-M", integrationMainBranchNameConstant})
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "remote", "add", integrationRemoteNameConstant, remoteRepositoryPath})
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "push", "-u", integrationRemoteNameConstant, integrationMainBranchNameConstant})

	createFeatureBranch(testInstance, localRepositoryPath, integrationFeatureDeleteBranchConstant, integrationFeatureDeleteCommitMessageConstant, integrationUpdatedFileContentsConstant)
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "push", integrationRemoteNameConstant, integrationFeatureDeleteBranchConstant})
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "checkout", integrationMainBranchNameConstant})

	createFeatureBranch(testInstance, localRepositoryPath, integrationFeatureSkipBranchConstant, integrationFeatureSkipCommitMessageConstant, integrationUpdatedFileContentsConstant)
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "push", integrationRemoteNameConstant, integrationFeatureSkipBranchConstant})
	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "checkout", integrationMainBranchNameConstant})

	runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "branch", integrationFeatureMissingBranchConstant})

	fakeGHDirectoryPath := filepath.Join(temporaryRoot, integrationFakeGHDirectoryNameConstant)
	require.NoError(testInstance, os.MkdirAll(fakeGHDirectoryPath, 0o755))
	fakeGHScriptPath := filepath.Join(fakeGHDirectoryPath, integrationGHExecutableNameConstant)
	scriptContents := fmt.Sprintf(integrationFakeGHScriptTemplateConstant, integrationFakeGHPayloadConstant)
	writeFile(testInstance, fakeGHScriptPath, scriptContents)
	require.NoError(testInstance, os.Chmod(fakeGHScriptPath, 0o755))

	originalPathVariable := os.Getenv("PATH")
	updatedPathVariable := fmt.Sprintf("%s%c%s", fakeGHDirectoryPath, os.PathListSeparator, originalPathVariable)
	require.NoError(testInstance, os.Setenv("PATH", updatedPathVariable))
	defer func() {
		require.NoError(testInstance, os.Setenv("PATH", originalPathVariable))
	}()

	commandRunner := execshell.NewOSCommandRunner()
	commandLogger := zap.NewNop()
	shellExecutor, executorError := execshell.NewShellExecutor(commandLogger, commandRunner)
	require.NoError(testInstance, executorError)

	serviceLogger := zap.NewNop()
	cleanupService, serviceError := branches.NewService(serviceLogger, shellExecutor)
	require.NoError(testInstance, serviceError)

	cleanupOptions := branches.CleanupOptions{
		RemoteName:       integrationRemoteNameConstant,
		PullRequestLimit: integrationPullRequestLimitConstant,
		DryRun:           false,
		WorkingDirectory: localRepositoryPath,
	}

	cleanupError := cleanupService.Cleanup(context.Background(), cleanupOptions)
	require.NoError(testInstance, cleanupError)

	remoteDeletedOutput := runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "ls-remote", "--heads", integrationRemoteNameConstant, integrationFeatureDeleteBranchConstant})
	require.Empty(testInstance, strings.TrimSpace(remoteDeletedOutput), fmt.Sprintf(integrationExpectationMessageTemplateConstant, integrationFeatureDeleteBranchConstant))

	remoteSkipOutput := runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "ls-remote", "--heads", integrationRemoteNameConstant, integrationFeatureSkipBranchConstant})
	require.NotEmpty(testInstance, strings.TrimSpace(remoteSkipOutput), fmt.Sprintf(integrationExpectationMessageTemplateConstant, integrationFeatureSkipBranchConstant))

	localDeletedOutput := runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "branch", "--list", integrationFeatureDeleteBranchConstant})
	require.Empty(testInstance, strings.TrimSpace(localDeletedOutput), fmt.Sprintf(integrationExpectationMessageTemplateConstant, integrationFeatureDeleteBranchConstant))

	localMissingOutput := runGitCommand(testInstance, localRepositoryPath, []string{integrationGitExecutableNameConstant, "branch", "--list", integrationFeatureMissingBranchConstant})
	require.NotEmpty(testInstance, strings.TrimSpace(localMissingOutput), fmt.Sprintf(integrationExpectationMessageTemplateConstant, integrationFeatureMissingBranchConstant))
}

func configureLocalRepository(testInstance *testing.T, repositoryPath string) {
	runGitCommand(testInstance, repositoryPath, []string{integrationGitExecutableNameConstant, "config", "user.name", integrationUserNameConstant})
	runGitCommand(testInstance, repositoryPath, []string{integrationGitExecutableNameConstant, "config", "user.email", integrationUserEmailConstant})
}

func createFeatureBranch(testInstance *testing.T, repositoryPath string, branchName string, commitMessage string, fileContents string) {
	runGitCommand(testInstance, repositoryPath, []string{integrationGitExecutableNameConstant, "checkout", "-b", branchName})
	writeFile(testInstance, filepath.Join(repositoryPath, integrationInitialFileNameConstant), fileContents)
	runGitCommand(testInstance, repositoryPath, []string{integrationGitExecutableNameConstant, "commit", "-am", commitMessage})
}

func writeFile(testInstance *testing.T, filePath string, contents string) {
	require.NoError(testInstance, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(testInstance, os.WriteFile(filePath, []byte(contents), 0o644))
}

func runGitCommand(testInstance *testing.T, workingDirectory string, arguments []string) string {
	executionContext, cancelFunction := context.WithTimeout(context.Background(), prCleanupCommandTimeoutConstant)
	defer cancelFunction()

	command := exec.CommandContext(executionContext, arguments[0], arguments[1:]...)
	if len(workingDirectory) > 0 {
		command.Dir = workingDirectory
	}

	outputBytes, commandError := command.CombinedOutput()
	require.NoError(testInstance, commandError, string(outputBytes))
	return string(outputBytes)
}
