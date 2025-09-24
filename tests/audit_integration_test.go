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
	auditIntegrationTimeout                     = 10 * time.Second
	auditIntegrationLogLevelFlag                = "--log-level"
	auditIntegrationErrorLevel                  = "error"
	auditIntegrationCommand                     = "go"
	auditIntegrationRunSubcommand               = "run"
	auditIntegrationModulePathConstant          = "."
	auditIntegrationAuditSubcommand             = "audit"
	auditIntegrationAuditFlag                   = "--audit"
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
	auditIntegrationHelpCaseNameConstant        = "audit_help_missing_flag"
	auditIntegrationHelpUsageSnippetConstant    = "audit [flags]"
	auditIntegrationMissingAuditMessageConstant = "specify --audit"
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
		repositoryPath,
	}

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
	}

	for testCaseIndex, testCase := range testCases {
		testInstance.Run(fmt.Sprintf(auditIntegrationSubtestNameTemplateConstant, testCaseIndex, testCase.name), func(subtest *testing.T) {
			subtestOutput := runIntegrationCommand(subtest, repositoryRoot, extendedPath, auditIntegrationTimeout, testCase.arguments)
			require.Equal(subtest, testCase.expectedOutput, filterStructuredOutput(subtestOutput))
		})
	}
}

func TestAuditCommandDisplaysHelpWhenAuditFlagMissing(testInstance *testing.T) {
	workingDirectory, workingDirectoryError := os.Getwd()
	require.NoError(testInstance, workingDirectoryError)
	repositoryRoot := filepath.Dir(workingDirectory)

	testCases := []struct {
		name             string
		arguments        []string
		expectedSnippets []string
	}{
		{
			name: auditIntegrationHelpCaseNameConstant,
			arguments: []string{
				auditIntegrationRunSubcommand,
				auditIntegrationModulePathConstant,
				auditIntegrationLogLevelFlag,
				auditIntegrationErrorLevel,
				auditIntegrationAuditSubcommand,
			},
			expectedSnippets: []string{
				integrationHelpUsagePrefixConstant,
				auditIntegrationHelpUsageSnippetConstant,
				auditIntegrationMissingAuditMessageConstant,
			},
		},
	}

	for testCaseIndex, testCase := range testCases {
		subtestName := fmt.Sprintf(auditIntegrationSubtestNameTemplateConstant, testCaseIndex, testCase.name)
		testInstance.Run(subtestName, func(subtest *testing.T) {
			outputText, _ := runFailingIntegrationCommand(subtest, repositoryRoot, "", auditIntegrationTimeout, testCase.arguments)
			filteredOutput := filterStructuredOutput(outputText)
			for _, expectedSnippet := range testCase.expectedSnippets {
				require.Contains(subtest, filteredOutput, expectedSnippet)
			}
		})
	}
}
