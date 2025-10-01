package tests

import (
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
	auditIntegrationTimeout                    = 10 * time.Second
	auditIntegrationLogLevelFlag               = "--log-level"
	auditIntegrationErrorLevel                 = "error"
	auditIntegrationDebugLevel                 = "debug"
	auditIntegrationRunSubcommand              = "run"
	auditIntegrationModulePathConstant         = "."
	auditIntegrationAuditCommandName           = "audit"
	auditIntegrationRootFlag                   = "--root"
	auditIntegrationGitExecutable              = "git"
	auditIntegrationInitFlag                   = "init"
	auditIntegrationInitialBranchFlag          = "--initial-branch=main"
	auditIntegrationRemoteSubcommand           = "remote"
	auditIntegrationAddSubcommand              = "add"
	auditIntegrationOriginRemoteName           = "origin"
	auditIntegrationOriginURL                  = "https://github.com/origin/example.git"
	auditIntegrationStubExecutableName         = "gh"
	auditIntegrationStubScript                 = "#!/bin/sh\nif [ \"$1\" = \"repo\" ] && [ \"$2\" = \"view\" ]; then\n  cat <<'EOF'\n{\"nameWithOwner\":\"canonical/example\",\"defaultBranchRef\":{\"name\":\"main\"},\"description\":\"\"}\nEOF\n  exit 0\nfi\nexit 0\n"
	auditIntegrationRepositoryPrefixConstant   = "audit-integration-repository-"
	auditIntegrationHomeShortcutPrefixConstant = "~/"
	auditIntegrationCSVHeaderConstant          = "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical\n"
	auditIntegrationCSVRowTemplate             = "canonical/example,%[1]s,no,main,,n/a,https,no\n"
	auditIntegrationCSVTemplate                = auditIntegrationCSVHeaderConstant + auditIntegrationCSVRowTemplate
	auditIntegrationCSVCaseNameConstant        = "audit_csv"
	auditIntegrationDebugCaseNameConstant      = "audit_debug"
	auditIntegrationTildeCaseNameConstant      = "audit_tilde"
	auditIntegrationSubtestNameTemplate        = "%d_%s"
)

func TestAuditRunCommandIntegration(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	homeDirectory, homeDirectoryError := os.UserHomeDir()
	require.NoError(testInstance, homeDirectoryError)

	repositoryPath, repositoryPathError := os.MkdirTemp(homeDirectory, auditIntegrationRepositoryPrefixConstant)
	require.NoError(testInstance, repositoryPathError)
	testInstance.Cleanup(func() {
		_ = os.RemoveAll(repositoryPath)
	})

	tempDirectory := testInstance.TempDir()

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

	repositoryFolderName := filepath.Base(repositoryPath)
	expectedCSVOutput := fmt.Sprintf(auditIntegrationCSVTemplate, repositoryFolderName)
	relativeRepositoryPath := strings.TrimPrefix(repositoryPath, homeDirectory)
	relativeRepositoryPath = strings.TrimPrefix(relativeRepositoryPath, string(os.PathSeparator))
	tildeRootArgument := auditIntegrationHomeShortcutPrefixConstant + filepath.ToSlash(relativeRepositoryPath)

	buildArguments := func(logLevel string, root string) []string {
		return []string{
			auditIntegrationRunSubcommand,
			auditIntegrationModulePathConstant,
			auditIntegrationLogLevelFlag,
			logLevel,
			auditIntegrationAuditCommandName,
			auditIntegrationRootFlag,
			root,
		}
	}

	rootFlagArguments := buildArguments(auditIntegrationErrorLevel, repositoryPath)
	debugLogLevelArguments := buildArguments(auditIntegrationDebugLevel, repositoryPath)
	tildeRootArguments := buildArguments(auditIntegrationErrorLevel, tildeRootArgument)

	testCases := []struct {
		name              string
		arguments         []string
		expectedOutput    string
		expectedFragments []string
	}{
		{
			name:           auditIntegrationCSVCaseNameConstant,
			arguments:      rootFlagArguments,
			expectedOutput: expectedCSVOutput,
		},
		{
			name:      auditIntegrationDebugCaseNameConstant,
			arguments: debugLogLevelArguments,
			expectedFragments: []string{
				fmt.Sprintf("DEBUG: discovered 1 candidate repos under: %s", repositoryPath),
				fmt.Sprintf("DEBUG: checking %s", repositoryPath),
				auditIntegrationCSVHeaderConstant,
				fmt.Sprintf(auditIntegrationCSVRowTemplate, repositoryFolderName),
			},
		},
		{
			name:           auditIntegrationTildeCaseNameConstant,
			arguments:      tildeRootArguments,
			expectedOutput: expectedCSVOutput,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(auditIntegrationSubtestNameTemplate, testCaseIndex, testCase.name), func(subtest *testing.T) {
			commandOptions := integrationCommandOptions{PathVariable: extendedPath}
			subtestOutput := runIntegrationCommand(subtest, repositoryRoot, commandOptions, auditIntegrationTimeout, testCase.arguments)
			filteredOutput := filterStructuredOutput(subtestOutput)
			if len(testCase.expectedOutput) > 0 {
				require.Equal(subtest, testCase.expectedOutput, filteredOutput)
			}
			for _, fragment := range testCase.expectedFragments {
				require.Contains(subtest, filteredOutput, fragment)
			}
		})
	}
}
