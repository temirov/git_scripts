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
	workflowpkg "github.com/temirov/gix/internal/workflow"
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
			runner := &recordingTaskRunner{}

			builder := workflowcmd.CommandBuilder{
				LoggerProvider: func() *zap.Logger { return zap.NewNop() },
				Discoverer:     discoverer,
				GitExecutor:    executor,
				ConfigurationProvider: func() workflowcmd.CommandConfiguration {
					return testCase.configuration
				},
				TaskRunnerFactory: func(workflowpkg.Dependencies) workflowcmd.TaskRunnerExecutor {
					return runner
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
				require.Equal(subtest, 0, runner.invocations)

				outputText := outputBuffer.String()
				require.Contains(subtest, outputText, workflowUsageSnippet)
				return
			}

			require.NoError(subtest, executionError)

			require.Equal(subtest, 1, runner.invocations)
			require.Equal(subtest, testCase.expectedRoots, runner.roots)
			require.Equal(subtest, testCase.expectPlanMessage, runner.runtimeOptions.DryRun)
			require.NotEmpty(subtest, runner.definitions)
			require.Equal(subtest, "audit.report", runner.definitions[0].Actions[0].Type)
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
	gitExecutor := &fakeWorkflowGitExecutor{}
	runner := &recordingTaskRunner{}

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
		TaskRunnerFactory: func(workflowpkg.Dependencies) workflowcmd.TaskRunnerExecutor {
			return runner
		},
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)
	bindGlobalWorkflowFlags(command)

	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetContext(context.Background())

	command.SetArgs([]string{configPath})

	executionError := command.Execute()
	require.NoError(testInstance, executionError)

	require.Equal(testInstance, 1, runner.invocations)
	require.Equal(testInstance, []string{repositoryPath}, runner.roots)
	require.True(testInstance, runner.runtimeOptions.DryRun)
	require.Len(testInstance, runner.definitions, 1)
	task := runner.definitions[0]
	require.Equal(testInstance, "Add Notes", task.Name)
	require.Len(testInstance, task.Files, 1)
	require.Equal(testInstance, "NOTES.md", task.Files[0].PathTemplate)
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

type recordingTaskRunner struct {
	roots          []string
	definitions    []workflowpkg.TaskDefinition
	runtimeOptions workflowpkg.RuntimeOptions
	invocations    int
}

func (runner *recordingTaskRunner) Run(_ context.Context, roots []string, definitions []workflowpkg.TaskDefinition, options workflowpkg.RuntimeOptions) error {
	runner.invocations++
	runner.roots = append([]string{}, roots...)
	runner.definitions = append([]workflowpkg.TaskDefinition{}, definitions...)
	runner.runtimeOptions = options
	return nil
}
