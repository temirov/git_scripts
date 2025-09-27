package workflow_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	workflowcmd "github.com/temirov/git_scripts/cmd/cli/workflow"
	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	workflowConfigFileNameConstant = "config.yaml"
	workflowConfigContentConstant  = "workflow:\n  - operation: audit-report\n"
	workflowConfiguredRootConstant = "/tmp/workflow-config-root"
	workflowCliRootConstant        = "/tmp/workflow-cli-root"
	workflowPlanMessageSnippet     = "WORKFLOW-PLAN: audit report"
	workflowCSVHeaderSnippet       = "final_github_repo,folder_name"
	workflowRootsFlagConstant      = "--roots"
	workflowDryRunFlagConstant     = "--dry-run"
)

func TestWorkflowCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name              string
		configuration     workflowcmd.CommandConfiguration
		additionalArgs    []string
		expectedRoots     []string
		expectPlanMessage bool
	}{
		{
			name: "configuration_applies_without_flags",
			configuration: workflowcmd.CommandConfiguration{
				Roots:  []string{workflowConfiguredRootConstant},
				DryRun: true,
			},
			additionalArgs:    []string{},
			expectedRoots:     []string{workflowConfiguredRootConstant},
			expectPlanMessage: true,
		},
		{
			name: "flags_override_configuration",
			configuration: workflowcmd.CommandConfiguration{
				Roots:  []string{workflowConfiguredRootConstant},
				DryRun: false,
			},
			additionalArgs: []string{
				workflowRootsFlagConstant,
				workflowCliRootConstant,
				workflowDryRunFlagConstant,
			},
			expectedRoots:     []string{workflowCliRootConstant},
			expectPlanMessage: true,
		},
		{
			name:              "defaults_fill_when_configuration_empty",
			configuration:     workflowcmd.CommandConfiguration{},
			additionalArgs:    []string{},
			expectedRoots:     []string{"."},
			expectPlanMessage: false,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			tempDirectory := subtest.TempDir()
			configPath := filepath.Join(tempDirectory, workflowConfigFileNameConstant)
			writeError := os.WriteFile(configPath, []byte(workflowConfigContentConstant), 0o644)
			require.NoError(subtest, writeError)

			discoverer := &fakeWorkflowDiscoverer{}
			executor := &fakeWorkflowGitExecutor{}

			builder := workflowcmd.CommandBuilder{
				LoggerProvider: func() *zap.Logger { return zap.NewNop() },
				Discoverer:     discoverer,
				GitExecutor:    executor,
				ConfigurationProvider: func() workflowcmd.CommandConfiguration {
					return testCase.configuration
				},
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)

			var outputBuffer bytes.Buffer
			var errorBuffer bytes.Buffer
			command.SetOut(&outputBuffer)
			command.SetErr(&errorBuffer)
			command.SetContext(context.Background())

			arguments := append([]string{configPath}, testCase.additionalArgs...)
			command.SetArgs(arguments)

			executionError := command.Execute()
			require.NoError(subtest, executionError)

			require.Equal(subtest, testCase.expectedRoots, discoverer.receivedRoots)

			outputText := outputBuffer.String()
			if testCase.expectPlanMessage {
				require.Contains(subtest, outputText, workflowPlanMessageSnippet)
			} else {
				require.Contains(subtest, outputText, workflowCSVHeaderSnippet)
			}
		})
	}
}

type fakeWorkflowDiscoverer struct {
	receivedRoots []string
}

func (discoverer *fakeWorkflowDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	discoverer.receivedRoots = append([]string{}, roots...)
	return []string{}, nil
}

type fakeWorkflowGitExecutor struct{}

func (executor *fakeWorkflowGitExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}

func (executor *fakeWorkflowGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}
