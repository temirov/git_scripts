package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	auditIntegrationTimeout               = 10 * time.Second
	auditIntegrationLogLevelFlag          = "--log-level"
	auditIntegrationErrorLevel            = "error"
	auditIntegrationRunSubcommand         = "run"
	auditIntegrationModulePathConstant    = "."
	auditIntegrationAuditCommandName      = "audit-run"
	auditIntegrationRootFlag              = "--root"
	auditIntegrationDebugFlag             = "--debug"
	auditIntegrationGitExecutable         = "git"
	auditIntegrationInitFlag              = "init"
	auditIntegrationInitialBranchFlag     = "--initial-branch=main"
	auditIntegrationRemoteSubcommand      = "remote"
	auditIntegrationAddSubcommand         = "add"
	auditIntegrationOriginRemoteName      = "origin"
	auditIntegrationOriginURL             = "https://github.com/origin/example.git"
	auditIntegrationStubExecutableName    = "gh"
	auditIntegrationStubScript            = "#!/bin/sh\nif [ \"$1\" = \"repo\" ] && [ \"$2\" = \"view\" ]; then\n  cat <<'EOF'\n{\"nameWithOwner\":\"canonical/example\",\"defaultBranchRef\":{\"name\":\"main\"},\"description\":\"\"}\nEOF\n  exit 0\nfi\nexit 0\n"
	auditIntegrationCSVOutput             = "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical\ncanonical/example,legacy,no,main,,n/a,https,no\n"
	auditIntegrationDebugOutput           = "DEBUG: discovered 1 candidate repos under: %[1]s\nDEBUG: checking %[2]s\n" + auditIntegrationCSVOutput
	auditIntegrationCSVCaseNameConstant   = "audit_csv"
	auditIntegrationDebugCaseNameConstant = "audit_debug"
	auditIntegrationSubtestNameTemplate   = "%d_%s"
)

func TestAuditRunCommandIntegration(testInstance *testing.T) {
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

	rootFlagArguments := []string{
		auditIntegrationRunSubcommand,
		auditIntegrationModulePathConstant,
		auditIntegrationLogLevelFlag,
		auditIntegrationErrorLevel,
		auditIntegrationAuditCommandName,
		auditIntegrationRootFlag,
		repositoryPath,
	}

	debugFlagArguments := append([]string{}, rootFlagArguments...)
	debugFlagArguments = append(debugFlagArguments, auditIntegrationDebugFlag)

	testCases := []struct {
		name           string
		arguments      []string
		expectedOutput string
	}{
		{
			name:           auditIntegrationCSVCaseNameConstant,
			arguments:      rootFlagArguments,
			expectedOutput: auditIntegrationCSVOutput,
		},
		{
			name:           auditIntegrationDebugCaseNameConstant,
			arguments:      debugFlagArguments,
			expectedOutput: fmt.Sprintf(auditIntegrationDebugOutput, repositoryPath, repositoryPath),
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(auditIntegrationSubtestNameTemplate, testCaseIndex, testCase.name), func(subtest *testing.T) {
			subtestOutput := runIntegrationCommand(subtest, repositoryRoot, extendedPath, auditIntegrationTimeout, testCase.arguments)
			require.Equal(subtest, testCase.expectedOutput, filterStructuredOutput(subtestOutput))
		})
	}
}
