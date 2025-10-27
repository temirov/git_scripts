package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

const (
	reposIntegrationTimeout                     = 10 * time.Second
	reposIntegrationLogLevelFlag                = "--log-level"
	reposIntegrationErrorLevel                  = "error"
	reposIntegrationRunSubcommand               = "run"
	reposIntegrationModulePathConstant          = "."
	reposIntegrationRepoNamespaceCommand        = "repo"
	reposIntegrationFolderNamespaceCommand      = "folder"
	reposIntegrationRemoteNamespaceCommand      = "remote"
	reposIntegrationRenameActionCommand         = "rename"
	reposIntegrationUpdateCanonicalAction       = "update-to-canonical"
	reposIntegrationUpdateProtocolAction        = "update-protocol"
	reposIntegrationDryRunFlag                  = "--dry-run"
	reposIntegrationYesFlag                     = "--yes"
	reposIntegrationOwnerFlag                   = "--owner"
	reposIntegrationRootFlag                    = "--" + flagutils.DefaultRootFlagName
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
	reposIntegrationRenameOwnerPlanCaseName     = "rename_plan_with_owner"
	reposIntegrationRenameOwnerExecuteCaseName  = "rename_execute_with_owner"
	reposIntegrationNestedRenameCaseName        = "rename_nested_repositories"
	reposIntegrationNestedToolsDirectoryName    = "tools"
	reposIntegrationNestedRepositoryName        = "svg_tools"
	reposIntegrationNestedOriginURL             = "https://github.com/temirov/svg_tools.git"
	reposIntegrationGitUserName                 = "Integration Test"
	reposIntegrationGitUserEmail                = "integration@example.com"
	reposIntegrationNestedIgnoreEntry           = "tools/"
	reposIntegrationNestedIgnoreCommitMessage   = "Add nested ignore"
	reposIntegrationRemoteCaseName              = "update_canonical_remote"
	reposIntegrationRemoteConfigCaseName        = "update_canonical_remote_config"
	reposIntegrationRemoteTildeCaseName         = "update_canonical_remote_tilde_flag"
	reposIntegrationProtocolCaseName            = "convert_protocol"
	reposIntegrationProtocolConfigCaseName      = "convert_protocol_config"
	reposIntegrationProtocolConfigDryRunCase    = "convert_protocol_config_dry_run_literal"
	reposIntegrationProtocolHelpCaseName        = "protocol_help_missing_flags"
	reposIntegrationProtocolUsageSnippet        = "gix repo remote update-protocol [flags]"
	reposIntegrationProtocolMissingFlagsMessage = "specify both --from and --to"
	reposIntegrationConfigFlagName              = "--config"
	reposIntegrationConfigFileName              = "config.yaml"
	reposIntegrationConfigTemplate              = "common:\n  log_level: error\noperations:\n  - operation: repo-remote-update\n    with:\n      roots:\n        - %s\n      assume_yes: true\n  - operation: repo-protocol-convert\n    with:\n      roots:\n        - %s\n      assume_yes: true\n      from: https\n      to: ssh\nworkflow: []\n"
	reposIntegrationConfigSearchEnvName         = "GIX_CONFIG_SEARCH_PATH"
	reposIntegrationHomeSymbolConstant          = "~"
	reposIntegrationHomeRootPatternConstant     = "gix-home-root-*"
	reposIntegrationOwnerDirectoryName          = "canonical"
	reposIntegrationRepositoryName              = "example"
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationFolderNamespaceCommand,
				reposIntegrationRenameActionCommand,
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
			name: reposIntegrationRenameOwnerPlanCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationFolderNamespaceCommand,
				reposIntegrationRenameActionCommand,
				reposIntegrationDryRunFlag,
				reposIntegrationOwnerFlag,
			},
			expectedOutput: func(repositoryPath string) string {
				absolutePath, absError := filepath.Abs(repositoryPath)
				require.NoError(testInstance, absError)
				parent := filepath.Dir(absolutePath)
				target := filepath.Join(parent, reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				return fmt.Sprintf("PLAN-OK: %s → %s\n", absolutePath, target)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {
				_, statError := os.Stat(repositoryPath)
				require.NoError(testInstance, statError)
			},
			prepare: func(testInstance *testing.T, repositoryPath string, arguments *[]string) {
				ownerDirectory := filepath.Join(filepath.Dir(repositoryPath), reposIntegrationOwnerDirectoryName)
				require.NoError(testInstance, os.MkdirAll(ownerDirectory, 0o755))
			},
		},
		{
			name: reposIntegrationRenameOwnerExecuteCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				return initializeRepositoryWithStub(testInstance)
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationFolderNamespaceCommand,
				reposIntegrationRenameActionCommand,
				reposIntegrationYesFlag,
				reposIntegrationOwnerFlag,
			},
			expectedOutput: func(repositoryPath string) string {
				absolutePath, absError := filepath.Abs(repositoryPath)
				require.NoError(testInstance, absError)
				parent := filepath.Dir(absolutePath)
				target := filepath.Join(parent, reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				return fmt.Sprintf("Renamed %s → %s\n", absolutePath, target)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {
				parent := filepath.Dir(repositoryPath)
				target := filepath.Join(parent, reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				_, targetError := os.Stat(target)
				require.NoError(testInstance, targetError)
				_, originalError := os.Stat(repositoryPath)
				require.Error(testInstance, originalError)
			},
			prepare: func(testInstance *testing.T, repositoryPath string, arguments *[]string) {
				ownerDirectory := filepath.Join(filepath.Dir(repositoryPath), reposIntegrationOwnerDirectoryName)
				require.NoError(testInstance, os.MkdirAll(ownerDirectory, 0o755))
			},
		},
		{
			name: reposIntegrationNestedRenameCaseName,
			setup: func(testInstance *testing.T) (string, string) {
				repositoryPath, extendedPath := initializeRepositoryWithStub(testInstance)
				configNameCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, "config", "user.name", reposIntegrationGitUserName)
				configNameCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				require.NoError(testInstance, configNameCommand.Run())
				configEmailCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, "config", "user.email", reposIntegrationGitUserEmail)
				configEmailCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				require.NoError(testInstance, configEmailCommand.Run())
				ignorePath := filepath.Join(repositoryPath, ".gitignore")
				require.NoError(testInstance, os.WriteFile(ignorePath, []byte(reposIntegrationNestedIgnoreEntry+"\n"), 0o644))
				addIgnoreCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, "add", ".gitignore")
				addIgnoreCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				require.NoError(testInstance, addIgnoreCommand.Run())
				commitIgnoreCommand := exec.Command(reposIntegrationGitExecutable, "-C", repositoryPath, "commit", "-m", reposIntegrationNestedIgnoreCommitMessage)
				commitIgnoreCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				require.NoError(testInstance, commitIgnoreCommand.Run())
				nestedParentPath := filepath.Join(repositoryPath, reposIntegrationNestedToolsDirectoryName)
				require.NoError(testInstance, os.MkdirAll(nestedParentPath, 0o755))
				nestedRepositoryPath := filepath.Join(nestedParentPath, reposIntegrationNestedRepositoryName)
				initCommand := exec.Command(reposIntegrationGitExecutable, reposIntegrationInitFlag, reposIntegrationInitialBranchFlag, nestedRepositoryPath)
				initCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				require.NoError(testInstance, initCommand.Run())
				remoteCommand := exec.Command(reposIntegrationGitExecutable, "-C", nestedRepositoryPath, reposIntegrationRemoteSubcommand, reposIntegrationAddSubcommand, reposIntegrationOriginRemoteName, reposIntegrationNestedOriginURL)
				remoteCommand.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				require.NoError(testInstance, remoteCommand.Run())
				return repositoryPath, extendedPath
			},
			arguments: []string{
				reposIntegrationRunSubcommand,
				reposIntegrationModulePathConstant,
				reposIntegrationLogLevelFlag,
				reposIntegrationErrorLevel,
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationFolderNamespaceCommand,
				reposIntegrationRenameActionCommand,
				reposIntegrationYesFlag,
				reposIntegrationOwnerFlag,
			},
			expectedOutput: func(repositoryPath string) string {
				absolutePath, absError := filepath.Abs(repositoryPath)
				require.NoError(testInstance, absError)
				nestedOriginalPath := filepath.Join(absolutePath, reposIntegrationNestedToolsDirectoryName, reposIntegrationNestedRepositoryName)
				nestedTargetPath := filepath.Join(absolutePath, reposIntegrationNestedToolsDirectoryName, reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				parentTargetPath := filepath.Join(filepath.Dir(absolutePath), reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				return fmt.Sprintf("Renamed %s → %s\nRenamed %s → %s\n", nestedOriginalPath, nestedTargetPath, absolutePath, parentTargetPath)
			},
			verify: func(testInstance *testing.T, repositoryPath string) {
				absolutePath, absError := filepath.Abs(repositoryPath)
				require.NoError(testInstance, absError)
				parentTargetPath := filepath.Join(filepath.Dir(absolutePath), reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				_, parentTargetError := os.Stat(parentTargetPath)
				require.NoError(testInstance, parentTargetError)
				_, originalParentError := os.Stat(repositoryPath)
				require.Error(testInstance, originalParentError)
				nestedTargetPath := filepath.Join(parentTargetPath, reposIntegrationNestedToolsDirectoryName, reposIntegrationOwnerDirectoryName, reposIntegrationRepositoryName)
				_, nestedTargetError := os.Stat(nestedTargetPath)
				require.NoError(testInstance, nestedTargetError)
				nestedOriginalPath := filepath.Join(absolutePath, reposIntegrationNestedToolsDirectoryName, reposIntegrationNestedRepositoryName)
				_, nestedOriginalError := os.Stat(nestedOriginalPath)
				require.Error(testInstance, nestedOriginalError)
			},
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateCanonicalAction,
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateCanonicalAction,
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateCanonicalAction,
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
				*arguments = append(*arguments, reposIntegrationRootFlag, tildeRoot)
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateProtocolAction,
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateProtocolAction,
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateProtocolAction,
				reposIntegrationDryRunFlag,
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
				commandArguments = append(commandArguments, reposIntegrationRootFlag, repositoryPath)
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
				reposIntegrationRepoNamespaceCommand,
				reposIntegrationRemoteNamespaceCommand,
				reposIntegrationUpdateProtocolAction,
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
