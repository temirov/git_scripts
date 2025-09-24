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
	reposIntegrationTimeout                     = 10 * time.Second
	reposIntegrationLogLevelFlag                = "--log-level"
	reposIntegrationErrorLevel                  = "error"
	reposIntegrationRunSubcommand               = "run"
	reposIntegrationModulePathConstant          = "."
	reposIntegrationGroup                       = "repos"
	reposIntegrationRenameCommand               = "rename-folders"
	reposIntegrationRemotesCommand              = "update-canonical-remote"
	reposIntegrationProtocolCommand             = "convert-remote-protocol"
	reposIntegrationDryRunFlag                  = "--dry-run"
	reposIntegrationYesFlag                     = "--yes"
	reposIntegrationFromFlag                    = "--from"
	reposIntegrationToFlag                      = "--to"
	reposIntegrationHTTPSProtocol               = "https"
	reposIntegrationSSHProtocol                 = "ssh"
	reposIntegrationGitExecutable               = "git"
	reposIntegrationInitFlag                    = "init"
	reposIntegrationInitialBranchFlag           = "--initial-branch=main"
	reposIntegrationRemoteSubcommand            = "remote"
	reposIntegrationAddSubcommand               = "add"
	reposIntegrationGetURLSubcommand            = "get-url"
	reposIntegrationOriginRemoteName            = "origin"
	reposIntegrationOriginURL                   = "https://github.com/origin/example.git"
	reposIntegrationStubExecutableName          = "gh"
	reposIntegrationStubScript                  = "#!/bin/sh\nif [ \"$1\" = \"repo\" ] && [ \"$2\" = \"view\" ]; then\n  cat <<'EOF'\n{\"nameWithOwner\":\"canonical/example\",\"defaultBranchRef\":{\"name\":\"main\"},\"description\":\"\"}\nEOF\n  exit 0\nfi\nexit 0\n"
	reposIntegrationSubtestNameTemplate         = "%d_%s"
	reposIntegrationRenameCaseName              = "rename_plan"
	reposIntegrationRemoteCaseName              = "update_canonical_remote"
	reposIntegrationProtocolCaseName            = "convert_protocol"
	reposIntegrationProtocolHelpCaseName        = "protocol_help_missing_flags"
	reposIntegrationProtocolUsageSnippet        = "convert-remote-protocol [root ...]"
	reposIntegrationProtocolMissingFlagsMessage = "specify both --from and --to"
)

func TestReposCommandIntegration(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	testCases := []struct {
		name           string
		arguments      []string
		setup          func(testInstance *testing.T) (string, string)
		expectedOutput func(repositoryPath string) string
		verify         func(testInstance *testing.T, repositoryPath string)
	}{
		{
			name: reposIntegrationRenameCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationGroup,
				reposIntegrationRenameCommand,
				reposIntegrationDryRunFlag,
			},
			expectedOutput: func(repositoryPath string) string {
				absolutePath, absError := filepath.Abs(repositoryPath)
				require.NoError(testInstance, absError)
				parent := filepath.Dir(absolutePath)
				target := filepath.Join(parent, "example")
				return fmt.Sprintf("PLAN-OK: %s → %s\n", absolutePath, target)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {},
		},
		{
			name: reposIntegrationRemoteCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationGroup,
				reposIntegrationRemotesCommand,
				reposIntegrationYesFlag,
			},
			expectedOutput: func(repositoryPath string) string {
				return fmt.Sprintf("UPDATE-REMOTE-DONE: %s origin now https://github.com/canonical/example.git\n", repositoryPath)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {
				remoteCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, reposIntegrationRemoteSubcommand, reposIntegrationGetURLSubcommand, reposIntegrationOriginRemoteName)
				remoteCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				outputBytes, remoteError := remoteCommand.CombinedOutput()
				require.NoError(testInstance, remoteError, string(outputBytes))
				require.Equal(testInstance, "https://github.com/canonical/example.git\n", string(outputBytes))
			},
		},
		{
			name: reposIntegrationProtocolCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationGroup,
				reposIntegrationProtocolCommand,
				reposIntegrationYesFlag,
				reposIntegrationFromFlag,
				reposIntegrationHTTPSProtocol,
				reposIntegrationToFlag,
				reposIntegrationSSHProtocol,
			},
			expectedOutput: func(repositoryPath string) string {
				return fmt.Sprintf("CONVERT-DONE: %s origin now ssh://git@github.com/canonical/example.git\n", repositoryPath)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {
				remoteCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, reposIntegrationRemoteSubcommand, reposIntegrationGetURLSubcommand, reposIntegrationOriginRemoteName)
				remoteCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				outputBytes, remoteError := remoteCommand.CombinedOutput()
				require.NoError(testInstance, remoteError, string(outputBytes))
				require.Equal(testInstance, "ssh://git@github.com/canonical/example.git\n", string(outputBytes))
			},
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(reposIntegrationSubtestNameTemplate, testCaseIndex, testCase.name), func(subtest *testing.T) {
			repositoryPath, extendedPath := testCase.setup(subtest)

			commandArguments := append([]string{}, testCase.arguments...)
			commandArguments = append(commandArguments, repositoryPath)

			rawOutput := runIntegrationCommand(subtest, repositoryRoot, extendedPath, reposIntegrationTimeout, commandArguments)
			expectedOutput := testCase.expectedOutput(repositoryPath)
			require.Equal(subtest, expectedOutput, filterStructuredOutput(rawOutput))
			testCase.verify(subtest, repositoryPath)
		})
	}
}

func TestReposProtocolCommandDisplaysHelpWhenProtocolsMissing(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	testCases := []struct {
		name             string
		arguments        []string
		expectedSnippets []string
	}{
		{
			name: reposIntegrationProtocolHelpCaseName,
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationGroup,
				reposIntegrationProtocolCommand,
			},
			expectedSnippets: []string{
				integrationHelpUsagePrefixConstant,
				reposIntegrationProtocolUsageSnippet,
				reposIntegrationProtocolMissingFlagsMessage,
			},
		},
	}

	for testCaseIndex, testCase := range testCases {
		subtestName := fmt.Sprintf(reposIntegrationSubtestNameTemplate, testCaseIndex, testCase.name)
		testInstance.Run(subtestName, func(subtest *testing.T) {
			outputText, _ := runFailingIntegrationCommand(subtest, repositoryRoot, "", reposIntegrationTimeout, testCase.arguments)
			filteredOutput := filterStructuredOutput(outputText)
			for _, expectedSnippet := range testCase.expectedSnippets {
				require.Contains(subtest, filteredOutput, expectedSnippet)
			}
		})
	}
}

func initializeRepositoryWithStub(testInstance *testing.T) (string, string) {
	tempDirectory := testInstance.TempDir()
	repositoryPath := filepath.Join(tempDirectory, "legacy")

	initCommand := exec.Command(reposIntegrationGitExecutable, reposIntegrationInitFlag, reposIntegrationInitialBranchFlag, repositoryPath)
	initCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	require.NoError(testInstance, initCommand.Run())

	remoteCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, reposIntegrationRemoteSubcommand, reposIntegrationAddSubcommand, reposIntegrationOriginRemoteName, reposIntegrationOriginURL)
	remoteCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	require.NoError(testInstance, remoteCommand.Run())

	stubDirectory := filepath.Join(tempDirectory, "bin")
	require.NoError(testInstance, os.Mkdir(stubDirectory, 0o755))
	stubPath := filepath.Join(stubDirectory, reposIntegrationStubExecutableName)
	require.NoError(testInstance, os.WriteFile(stubPath, []byte(reposIntegrationStubScript), 0o755))

	extendedPath := stubDirectory + string(os.PathListSeparator) + os.Getenv("PATH")
	return repositoryPath, extendedPath
}
