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
	reposIntegrationRenameCommand               = "repo-folders-rename"
	reposIntegrationRemotesCommand              = "repo-remote-update"
	reposIntegrationProtocolCommand             = "repo-protocol-convert"
	reposIntegrationDryRunFlag                  = "--dry-run"
	reposIntegrationBooleanTrueLiteral          = "true"
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
	reposIntegrationRemoteConfigCaseName        = "update_canonical_remote_config"
	reposIntegrationRemoteTildeCaseName         = "update_canonical_remote_tilde_flag"
	reposIntegrationProtocolCaseName            = "convert_protocol"
	reposIntegrationProtocolConfigCaseName      = "convert_protocol_config"
	reposIntegrationProtocolConfigDryRunCase    = "convert_protocol_config_dry_run_literal"
	reposIntegrationProtocolHelpCaseName        = "protocol_help_missing_flags"
	reposIntegrationProtocolUsageSnippet        = "repo-protocol-convert [root ...]"
	reposIntegrationProtocolMissingFlagsMessage = "specify both --from and --to"
	reposIntegrationConfigFlagName              = "--config"
	reposIntegrationConfigFileName              = "config.yaml"
	reposIntegrationConfigTemplate              = "common:\n  log_level: error\noperations:\n  - operation: repo-remote-update\n    with:\n      roots:\n        - %s\n      assume_yes: true\n  - operation: repo-protocol-convert\n    with:\n      roots:\n        - %s\n      assume_yes: true\n      from: https\n      to: ssh\nworkflow: []\n"
	reposIntegrationConfigSearchEnvName         = "GITSCRIPTS_CONFIG_SEARCH_PATH"
	reposIntegrationHomeSymbolConstant          = "~"
	reposIntegrationHomeRootPatternConstant     = "git-scripts-home-root-*"
)

func TestReposCommandIntegration(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	testCases := []struct {
		name                   string
		arguments              []string
		setup                  func(testInstance *testing.T) (string, string)
		expectedOutput         func(repositoryPath string) string
		verify                 func(testInstance *testing.T, repositoryPath string)
		prepare                func(testInstance *testing.T, repositoryPath string, arguments *[]string)
		omitRepositoryArgument bool
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
			name: reposIntegrationRemoteConfigCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationRemotesCommand,
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
			prepare: func(testInstance *testing.T, repositoryPath string, arguments *[]string) {
				configDirectory := testInstance.TempDir()
				configPath := filepath.Join(configDirectory, reposIntegrationConfigFileName)
				configContent := fmt.Sprintf(reposIntegrationConfigTemplate, repositoryPath, repositoryPath)
				writeError := os.WriteFile(configPath, []byte(configContent), 0o644)
				require.NoError(testInstance, writeError)
				*arguments = append(*arguments, reposIntegrationConfigFlagName, configPath)
			},
			omitRepositoryArgument: true,
		},
		{
			name: reposIntegrationRemoteTildeCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				repositoryPath, extendedPath := initializeRepositoryWithStub(testInstance)
				homeDirectory, homeError := os.UserHomeDir()
				require.NoError(testInstance, homeError)
				homeRoot, homeRootError := os.MkdirTemp(homeDirectory, reposIntegrationHomeRootPatternConstant)
				require.NoError(testInstance, homeRootError)
				testInstance.Cleanup(func() {
					_ = os.RemoveAll(homeRoot)
				})
				destinationPath := filepath.Join(homeRoot, filepath.Base(repositoryPath))
				renameError := os.Rename(repositoryPath, destinationPath)
				require.NoError(testInstance, renameError)
				return destinationPath, extendedPath
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
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
			prepare: func(testInstance *testing.T, repositoryPath string, arguments *[]string) {
				homeDirectory, homeError := os.UserHomeDir()
				require.NoError(testInstance, homeError)
				relativePath, relativeError := filepath.Rel(homeDirectory, repositoryPath)
				require.NoError(testInstance, relativeError)
				tildeRoot := reposIntegrationHomeSymbolConstant + string(os.PathSeparator) + relativePath
				*arguments = append(*arguments, tildeRoot)
			},
			omitRepositoryArgument: true,
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
		{
			name: reposIntegrationProtocolConfigCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationProtocolCommand,
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
			prepare: func(testInstance *testing.T, repositoryPath string, arguments *[]string) {
				configDirectory := testInstance.TempDir()
				configPath := filepath.Join(configDirectory, reposIntegrationConfigFileName)
				configContent := fmt.Sprintf(reposIntegrationConfigTemplate, repositoryPath, repositoryPath)
				writeError := os.WriteFile(configPath, []byte(configContent), 0o644)
				require.NoError(testInstance, writeError)
				*arguments = append(*arguments, reposIntegrationConfigFlagName, configPath)
			},
			omitRepositoryArgument: true,
		},
		{
			name: reposIntegrationProtocolConfigDryRunCase,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationProtocolCommand,
				reposIntegrationDryRunFlag,
				reposIntegrationBooleanTrueLiteral,
			},
			expectedOutput: func(repositoryPath string) string {
				return fmt.Sprintf("PLAN-CONVERT: %s origin https://github.com/origin/example.git → ssh://git@github.com/canonical/example.git\n", repositoryPath)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {
				remoteCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, reposIntegrationRemoteSubcommand, reposIntegrationGetURLSubcommand, reposIntegrationOriginRemoteName)
				remoteCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				outputBytes, remoteError := remoteCommand.CombinedOutput()
				require.NoError(testInstance, remoteError, string(outputBytes))
				require.Equal(testInstance, "https://github.com/origin/example.git\n", string(outputBytes))
			},
			prepare: func(testInstance *testing.T, repositoryPath string, arguments *[]string) {
				configDirectory := testInstance.TempDir()
				configPath := filepath.Join(configDirectory, reposIntegrationConfigFileName)
				configContent := fmt.Sprintf(reposIntegrationConfigTemplate, repositoryPath, repositoryPath)
				writeError := os.WriteFile(configPath, []byte(configContent), 0o644)
				require.NoError(testInstance, writeError)
				*arguments = append(*arguments, reposIntegrationConfigFlagName, configPath)
			},
			omitRepositoryArgument: true,
		},
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(reposIntegrationSubtestNameTemplate, testCaseIndex, testCase.name), func(subtest *testing.T) {
			repositoryPath, extendedPath := testCase.setup(subtest)

			commandArguments := append([]string{}, testCase.arguments...)
			if testCase.prepare != nil {
				testCase.prepare(subtest, repositoryPath, &commandArguments)
			}
			if !testCase.omitRepositoryArgument {
				commandArguments = append(commandArguments, repositoryPath)
			}

			commandOptions := integrationCommandOptions{PathVariable: extendedPath}
			rawOutput := runIntegrationCommand(subtest, repositoryRoot, commandOptions, reposIntegrationTimeout, commandArguments)
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
			overrideDirectory := subtest.TempDir()
			commandOptions := integrationCommandOptions{
				EnvironmentOverrides: map[string]string{
					reposIntegrationConfigSearchEnvName: overrideDirectory,
				},
			}
			outputText, _ := runFailingIntegrationCommand(subtest, repositoryRoot, commandOptions, reposIntegrationTimeout, testCase.arguments)
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
