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
)

const (
	auditIntegrationTimeout                     = 10 * time.Second
	auditIntegrationLogLevelFlag                = "--log-level"
	auditIntegrationErrorLevel                  = "error"
	auditIntegrationCommand                     = "go"
	auditIntegrationRunSubcommand               = "run"
	auditIntegrationModulePathConstant          = "."
	auditIntegrationAuditSubcommand             = "audit"
	auditIntegrationDryRunFlag                  = "--dry-run"
	auditIntegrationAuditFlag                   = "--audit"
	auditIntegrationRenameFlag                  = "--rename"
	auditIntegrationRequireCleanFlag            = "--require-clean"
	auditIntegrationGitExecutable               = "git"
	auditIntegrationInitFlag                    = "init"
	auditIntegrationInitialBranchFlag           = "--initial-branch=main"
	auditIntegrationRemoteSubcommand            = "remote"
	auditIntegrationAddSubcommand               = "add"
	auditIntegrationOriginRemoteName            = "origin"
	auditIntegrationOriginURL                   = "https://github.com/origin/example.git"
	auditIntegrationStubExecutableName          = "gh"
	auditIntegrationStubScript                  = "#!/bin/sh\nif [ \"$1\" = \"repo\" ] && [ \"$2\" = \"view\" ]; then\n  cat <<'EOF'\n{\"nameWithOwner\":\"canonical/example\",\"defaultBranchRef\":{\"name\":\"main\"},\"description\":\"\"}\nEOF\n  exit 0\nfi\nexit 0\n"
	auditIntegrationCSVOutput                   = "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical\ncanonical/example,legacy,no,main,,n/a,https,no\n"
	auditIntegrationAuditCaseNameConstant       = "audit_csv"
	auditIntegrationRenameCaseNameConstant      = "rename_plan"
	auditIntegrationSubtestNameTemplateConstant = "%d_%s"
)

func TestAuditCommandIntegration(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	tempDirectory := testInstance.TempDir()
	repositoryPath := filepath.Join(tempDirectory, "legacy")

	initCommand := exec.Command(auditIntegrationGitExecutable, auditIntegrationInitFlag, auditIntegrationInitialBranchFlag, repositoryPath)
	initCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	initError := initCommand.Run()
	require.NoError(testInstance, initError)

	remoteCommand := exec.Command(auditIntegrationGitExecutable, "-C", repositoryPath, auditIntegrationRemoteSubcommand, auditIntegrationAddSubcommand, auditIntegrationOriginRemoteName, auditIntegrationOriginURL)
	remoteCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	remoteError := remoteCommand.Run()
	require.NoError(testInstance, remoteError)

	stubPath := filepath.Join(tempDirectory, auditIntegrationStubExecutableName)
	stubWriteError := os.WriteFile(stubPath, []byte(auditIntegrationStubScript), 0o755)
	require.NoError(testInstance, stubWriteError)

	pathWithStub := filepath.Join(tempDirectory, "bin")
	require.NoError(testInstance, os.Mkdir(pathWithStub, 0o755))
	finalStubPath := filepath.Join(pathWithStub, auditIntegrationStubExecutableName)
	require.NoError(testInstance, os.Rename(stubPath, finalStubPath))

	extendedPath := pathWithStub + string(os.PathListSeparator) + os.Getenv("PATH")

	auditCommandArguments := []string{
		auditIntegrationRunSubcommand,
		auditIntegrationModulePathConstant,
		auditIntegrationLogLevelFlag,
		auditIntegrationErrorLevel,
		auditIntegrationAuditSubcommand,
		auditIntegrationAuditFlag,
		auditIntegrationDryRunFlag,
		repositoryPath,
	}

	renameCommandArguments := []string{
		auditIntegrationRunSubcommand,
		auditIntegrationModulePathConstant,
		auditIntegrationLogLevelFlag,
		auditIntegrationErrorLevel,
		auditIntegrationAuditSubcommand,
		auditIntegrationRenameFlag,
		auditIntegrationRequireCleanFlag,
		auditIntegrationDryRunFlag,
		repositoryPath,
	}

	expectedRename := fmt.Sprintf("PLAN-OK: %s → %s\n", repositoryPath, filepath.Join(filepath.Dir(repositoryPath), "example"))

	testCases := []struct {
		name           string
		arguments      []string
		expectedOutput string
	}{
		{
			name:           auditIntegrationAuditCaseNameConstant,
			arguments:      auditCommandArguments,
			expectedOutput: auditIntegrationCSVOutput,
		},
		{
			name:           auditIntegrationRenameCaseNameConstant,
			arguments:      renameCommandArguments,
			expectedOutput: expectedRename,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(auditIntegrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(subtest *testing.T) {
			subtestOutput := runCommand(subtest, repositoryRoot, extendedPath, testCase.arguments)
			require.Equal(subtest, testCase.expectedOutput, filterCommandOutput(subtestOutput))
		})
	}
}

func runCommand(testInstance *testing.T, repositoryRoot string, pathVariable string, arguments []string) string {
	executionContext, cancel := context.WithTimeout(context.Background(), auditIntegrationTimeout)
	defer cancel()

	command := exec.CommandContext(executionContext, auditIntegrationCommand, arguments...)
	command.Dir = repositoryRoot
	environment := append([]string{}, os.Environ()...)
	environment = append(environment, "PATH="+pathVariable)
	command.Env = environment

	outputBytes, runError := command.CombinedOutput()
	require.NoError(testInstance, runError, string(outputBytes))
	return string(outputBytes)
}

func filterCommandOutput(rawOutput string) string {
	lines := strings.Split(rawOutput, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 {
		return ""
	}
	return strings.Join(filtered, "\n") + "\n"
}
