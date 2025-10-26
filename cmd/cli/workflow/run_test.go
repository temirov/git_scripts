package workflow_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	workflowcmd "github.com/temirov/gix/cmd/cli/workflow"
	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	workflowConfigFileNameConstant          = "config.yaml"
	workflowConfigContentConstant           = "operations:\n  - operation: workflow\n    with:\n      roots:\n        - .\nworkflow:\n  - step:\n      operation: audit-report\n"
	workflowApplyTasksConfigContentConstant = `
workflow:
  - step:
      operation: apply-tasks
      with:
        tasks:
          - name: Add Notes
            files:
              - path: NOTES.md
                content: "Repository: {{ .Repository.Name }}"
`
	workflowConfiguredRootConstant = "/tmp/workflow-config-root"
	workflowCliRootConstant        = "/tmp/workflow-cli-root"
	workflowPlanMessageSnippet     = "WORKFLOW-PLAN: audit report"
	workflowCSVHeaderSnippet       = "folder_name,final_github_repo"
	workflowRootsFlagConstant      = "--" + flagutils.DefaultRootFlagName
	workflowDryRunFlagConstant     = "--dry-run"
	workflowUsageSnippet           = "Usage:"
)

var workflowMissingRootsErrorMessage = rootutils.MissingRootsMessage()

func TestWorkflowCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                 string
		configuration        workflowcmd.CommandConfiguration
		additionalArgs       []string
		expectedRoots        []string
		expectPlanMessage    bool
		expectExecutionError bool
		expectedErrorMessage string
	}{
		{
			name: "configuration_applies_without_flags",
			configuration: workflowcmd.CommandConfiguration{
				Roots:  []string{workflowConfiguredRootConstant},
				DryRun: true,
			},
			additionalArgs:       []string{},
			expectedRoots:        []string{workflowConfiguredRootConstant},
			expectPlanMessage:    true,
			expectExecutionError: false,
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
			expectedRoots:        []string{workflowCliRootConstant},
			expectPlanMessage:    true,
			expectExecutionError: false,
		},
		{
			name: "flag_disables_require_clean_with_no_literal",
			configuration: workflowcmd.CommandConfiguration{
				Roots:        []string{workflowConfiguredRootConstant},
				RequireClean: true,
				DryRun:       false,
			},
			additionalArgs: []string{
				workflowRootsFlagConstant,
				workflowConfiguredRootConstant,
				"--require-clean",
				"no",
			},
			expectedRoots:        []string{workflowConfiguredRootConstant},
			expectPlanMessage:    false,
			expectExecutionError: false,
		},
		{
			name:                 "error_when_roots_missing",
			configuration:        workflowcmd.CommandConfiguration{},
			additionalArgs:       []string{},
			expectedRoots:        nil,
			expectPlanMessage:    false,
			expectExecutionError: true,
			expectedErrorMessage: workflowMissingRootsErrorMessage,
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
			bindGlobalWorkflowFlags(command)
			flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

			var outputBuffer bytes.Buffer
			var errorBuffer bytes.Buffer
			command.SetOut(&outputBuffer)
			command.SetErr(&errorBuffer)
			command.SetContext(context.Background())

			arguments := append([]string{configPath}, testCase.additionalArgs...)
			normalizedArguments := flagutils.NormalizeToggleArguments(arguments)
			command.SetArgs(normalizedArguments)

			executionError := command.Execute()

			if testCase.expectExecutionError {
				require.Error(subtest, executionError)
				require.EqualError(subtest, executionError, testCase.expectedErrorMessage)
				require.Nil(subtest, discoverer.receivedRoots)

				outputText := outputBuffer.String()
				require.Contains(subtest, outputText, workflowUsageSnippet)
				return
			}

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

func bindGlobalWorkflowFlags(command *cobra.Command) {
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
	flagutils.BindExecutionFlags(command, flagutils.ExecutionDefaults{}, flagutils.ExecutionFlagDefinitions{
		DryRun:    flagutils.ExecutionFlagDefinition{Name: flagutils.DryRunFlagName, Usage: flagutils.DryRunFlagUsage, Enabled: true},
		AssumeYes: flagutils.ExecutionFlagDefinition{Name: flagutils.AssumeYesFlagName, Usage: flagutils.AssumeYesFlagUsage, Shorthand: flagutils.AssumeYesFlagShorthand, Enabled: true},
	})
	command.PersistentFlags().String(flagutils.RemoteFlagName, "", flagutils.RemoteFlagUsage)
	command.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		contextAccessor := utils.NewCommandContextAccessor()
		executionFlags := utils.ExecutionFlags{}
		if dryRunValue, dryRunChanged, dryRunError := flagutils.BoolFlag(cmd, flagutils.DryRunFlagName); dryRunError == nil {
			executionFlags.DryRun = dryRunValue
			executionFlags.DryRunSet = dryRunChanged
		}
		if assumeYesValue, assumeYesChanged, assumeYesError := flagutils.BoolFlag(cmd, flagutils.AssumeYesFlagName); assumeYesError == nil {
			executionFlags.AssumeYes = assumeYesValue
			executionFlags.AssumeYesSet = assumeYesChanged
		}
		if remoteValue, remoteChanged, remoteError := flagutils.StringFlag(cmd, flagutils.RemoteFlagName); remoteError == nil {
			executionFlags.Remote = strings.TrimSpace(remoteValue)
			executionFlags.RemoteSet = remoteChanged && len(strings.TrimSpace(remoteValue)) > 0
		}
		updatedContext := contextAccessor.WithExecutionFlags(cmd.Context(), executionFlags)
		cmd.SetContext(updatedContext)
		return nil
	}
}

func TestWorkflowCommandApplyTasksDryRun(testInstance *testing.T) {
	tempDirectory := testInstance.TempDir()
	repositoryPath := filepath.Join(tempDirectory, "sample")
	writeRepositoryError := os.MkdirAll(repositoryPath, 0o755)
	require.NoError(testInstance, writeRepositoryError)

	configPath := filepath.Join(tempDirectory, workflowConfigFileNameConstant)
	configContent := strings.TrimSpace(workflowApplyTasksConfigContentConstant)
	writeConfigError := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(testInstance, writeConfigError)

	discoverer := &fakeWorkflowDiscoverer{
		repositories: []string{repositoryPath},
	}
	gitExecutor := &applyTasksWorkflowGitExecutor{}

	builder := workflowcmd.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		Discoverer:     discoverer,
		GitExecutor:    gitExecutor,
		ConfigurationProvider: func() workflowcmd.CommandConfiguration {
			return workflowcmd.CommandConfiguration{
				Roots:  []string{repositoryPath},
				DryRun: true,
			}
		},
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)
	bindGlobalWorkflowFlags(command)

	var outputBuffer bytes.Buffer
	var errorBuffer bytes.Buffer
	command.SetOut(&outputBuffer)
	command.SetErr(&errorBuffer)
	command.SetContext(context.Background())

	command.SetArgs([]string{configPath})

	executionError := command.Execute()
	require.NoError(testInstance, executionError)

	require.Equal(testInstance, []string{repositoryPath}, discoverer.receivedRoots)
	require.NotEmpty(testInstance, gitExecutor.githubCommands)

	outputText := outputBuffer.String()
	require.Contains(testInstance, outputText, "TASK-PLAN: Add Notes "+repositoryPath+" branch=automation-Add-Notes base=main")
	require.Contains(testInstance, outputText, "TASK-PLAN: Add Notes file=NOTES.md action=write")
}

type fakeWorkflowDiscoverer struct {
	receivedRoots []string
	repositories  []string
}

func (discoverer *fakeWorkflowDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	discoverer.receivedRoots = append([]string{}, roots...)
	if len(discoverer.repositories) == 0 {
		return []string{}, nil
	}
	return append([]string{}, discoverer.repositories...), nil
}

type fakeWorkflowGitExecutor struct{}

func (executor *fakeWorkflowGitExecutor) ExecuteGit(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}

func (executor *fakeWorkflowGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{StandardOutput: ""}, nil
}

type applyTasksWorkflowGitExecutor struct {
	gitCommands    []execshell.CommandDetails
	githubCommands []execshell.CommandDetails
}

func (executor *applyTasksWorkflowGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.gitCommands = append(executor.gitCommands, details)
	args := details.Arguments
	if len(args) >= 2 && args[0] == "rev-parse" {
		switch args[1] {
		case "--is-inside-work-tree":
			return execshell.ExecutionResult{StandardOutput: "true\n"}, nil
		case "--abbrev-ref":
			return execshell.ExecutionResult{StandardOutput: "master\n"}, nil
		}
	}

	if len(args) >= 3 && args[0] == "remote" && args[1] == "get-url" && args[2] == "origin" {
		return execshell.ExecutionResult{StandardOutput: "https://github.com/octocat/sample.git\n"}, nil
	}

	return execshell.ExecutionResult{}, nil
}

func (executor *applyTasksWorkflowGitExecutor) ExecuteGitHubCLI(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.githubCommands = append(executor.githubCommands, details)
	if len(details.Arguments) >= 2 && details.Arguments[0] == "repo" && details.Arguments[1] == "view" {
		response := `{"nameWithOwner":"octocat/sample","description":"","defaultBranchRef":{"name":"main"},"isInOrganization":false}`
		return execshell.ExecutionResult{StandardOutput: response}, nil
	}
	return execshell.ExecutionResult{}, nil
}
