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
	workflowConfigFileNameConstant = "config.yaml"
	workflowConfigContentConstant  = "operations:\n  - operation: workflow\n    with:\n      roots:\n        - .\nworkflow:\n  - step:\n      operation: audit-report\n"
	workflowConfiguredRootConstant = "/tmp/workflow-config-root"
	workflowCliRootConstant        = "/tmp/workflow-cli-root"
	workflowPlanMessageSnippet     = "WORKFLOW-PLAN: audit report"
	workflowCSVHeaderSnippet       = "final_github_repo,folder_name"
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
			command.SetArgs(arguments)

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
