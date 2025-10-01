package branches_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	branches "github.com/temirov/gix/internal/branches"
	"github.com/temirov/gix/internal/execshell"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	rootutils "github.com/temirov/gix/internal/utils/roots"
)

const (
	commandRemoteFlagConstant            = "--" + flagutils.RemoteFlagName
	commandLimitFlagConstant             = "--limit"
	commandRootFlagConstant              = "--" + flagutils.DefaultRootFlagName
	commandDryRunFlagConstant            = "--" + flagutils.DryRunFlagName
	testDefaultRemoteNameConstant        = "origin"
	testRemoteDescriptionConstant        = "Name of the remote containing pull request branches"
	commandLimitValueConstant            = "5"
	multiRootFirstArgumentConstant       = "root-one"
	multiRootSecondArgumentConstant      = "root-two"
	defaultRootArgumentConstant          = "."
	repositoryOnePathConstant            = "/tmp/repository-one"
	repositoryTwoPathConstant            = "/tmp/repository-two"
	cleanupBranchNameConstant            = "feature/shared"
	repositoryLogFieldNameConstant       = "repository"
	configurationRemoteNameConstant      = "configured-remote"
	configurationRootConstant            = "/tmp/config-root"
	flagOverrideRemoteConstant           = "override-remote"
	flagOverrideLimitValueConstant       = 7
	invalidRemoteErrorMessageConstant    = "remote name must not be empty or whitespace"
	invalidLimitErrorMessageConstant     = "limit must be greater than zero"
	defaultPullRequestLimitValueConstant = 100
	invalidLimitArgumentValueConstant    = "0"
	whitespaceRemoteArgumentConstant     = "   "
)

type fakeRepositoryDiscoverer struct {
	repositories   []string
	discoveryError error
	receivedRoots  []string
}

func (discoverer *fakeRepositoryDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	discoverer.receivedRoots = append([]string{}, roots...)
	if discoverer.discoveryError != nil {
		return nil, discoverer.discoveryError
	}
	return append([]string{}, discoverer.repositories...), nil
}

func TestCommandRunScenarios(testInstance *testing.T) {
	remoteBranches := []string{cleanupBranchNameConstant}
	remoteOutput := buildRemoteOutput(remoteBranches)

	pullRequestJSON, jsonError := buildPullRequestJSON(remoteBranches)
	require.NoError(testInstance, jsonError)

	gitListArguments := []string{gitListRemoteSubcommandConstant, gitHeadsFlagConstant, testRemoteNameConstant}
	githubListArguments := []string{
		githubPullRequestSubcommandConstant,
		githubListSubcommandConstant,
		githubStateFlagConstant,
		githubClosedStateConstant,
		githubJSONFlagConstant,
		pullRequestJSONFieldNameConstant,
		githubLimitFlagConstant,
		commandLimitValueConstant,
	}

	testCases := []struct {
		name                        string
		arguments                   []string
		discoveredRepositories      []string
		expectedRoots               []string
		setup                       func(*testing.T, *fakeCommandExecutor)
		expectedRepositories        []string
		expectedWarningRepositories []string
		verify                      func(*testing.T, *fakeCommandExecutor, []observer.LoggedEntry)
	}{
		{
			name: "processes_multiple_repositories",
			arguments: []string{
				commandRemoteFlagConstant,
				testRemoteNameConstant,
				commandLimitFlagConstant,
				commandLimitValueConstant,
				commandRootFlagConstant,
				multiRootFirstArgumentConstant,
				commandRootFlagConstant,
				multiRootSecondArgumentConstant,
			},
			discoveredRepositories: []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			expectedRoots:          []string{multiRootFirstArgumentConstant, multiRootSecondArgumentConstant},
			setup: func(t *testing.T, executor *fakeCommandExecutor) {
				registerResponse(executor, gitCommandLabelConstant, gitListArguments, execshell.ExecutionResult{StandardOutput: remoteOutput, ExitCode: 0}, nil)
				registerResponse(executor, githubCommandLabelConstant, githubListArguments, execshell.ExecutionResult{StandardOutput: pullRequestJSON, ExitCode: 0}, nil)
				registerResponse(executor, gitCommandLabelConstant, []string{gitPushSubcommandConstant, testRemoteNameConstant, gitDeleteFlagConstant, cleanupBranchNameConstant}, execshell.ExecutionResult{ExitCode: 0}, nil)
				registerResponse(executor, gitCommandLabelConstant, []string{gitBranchSubcommandConstant, gitForceDeleteFlagConstant, cleanupBranchNameConstant}, execshell.ExecutionResult{ExitCode: 0}, nil)
			},
			expectedRepositories:        []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			expectedWarningRepositories: nil,
			verify:                      nil,
		},
		{
			name: "dry_run_avoids_deletions",
			arguments: []string{
				commandDryRunFlagConstant,
				commandRemoteFlagConstant,
				testRemoteNameConstant,
				commandLimitFlagConstant,
				commandLimitValueConstant,
				commandRootFlagConstant,
				defaultRootArgumentConstant,
			},
			discoveredRepositories: []string{repositoryOnePathConstant},
			expectedRoots:          []string{defaultRootArgumentConstant},
			setup: func(t *testing.T, executor *fakeCommandExecutor) {
				registerResponse(executor, gitCommandLabelConstant, gitListArguments, execshell.ExecutionResult{StandardOutput: remoteOutput, ExitCode: 0}, nil)
				registerResponse(executor, githubCommandLabelConstant, githubListArguments, execshell.ExecutionResult{StandardOutput: pullRequestJSON, ExitCode: 0}, nil)
			},
			expectedRepositories:        []string{repositoryOnePathConstant},
			expectedWarningRepositories: nil,
			verify: func(t *testing.T, executor *fakeCommandExecutor, _ []observer.LoggedEntry) {
				for _, executedCommand := range executor.executedCommands {
					require.NotEqual(t, gitPushSubcommandConstant, executedCommand.arguments[0])
					require.NotEqual(t, gitBranchSubcommandConstant, executedCommand.arguments[0])
				}
			},
		},
		{
			name: "continues_when_repository_cleanup_fails",
			arguments: []string{
				commandRemoteFlagConstant,
				testRemoteNameConstant,
				commandLimitFlagConstant,
				commandLimitValueConstant,
				commandRootFlagConstant,
				multiRootFirstArgumentConstant,
			},
			discoveredRepositories: []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			expectedRoots:          []string{multiRootFirstArgumentConstant},
			setup: func(t *testing.T, executor *fakeCommandExecutor) {
				failureError := errors.New(remoteListFailureMessageConstant)
				registerRepositoryResponse(executor, repositoryOnePathConstant, gitCommandLabelConstant, gitListArguments, execshell.ExecutionResult{}, failureError)

				registerRepositoryResponse(executor, repositoryTwoPathConstant, gitCommandLabelConstant, gitListArguments, execshell.ExecutionResult{StandardOutput: remoteOutput, ExitCode: 0}, nil)
				registerRepositoryResponse(executor, repositoryTwoPathConstant, githubCommandLabelConstant, githubListArguments, execshell.ExecutionResult{StandardOutput: pullRequestJSON, ExitCode: 0}, nil)
				registerRepositoryResponse(executor, repositoryTwoPathConstant, gitCommandLabelConstant, []string{gitPushSubcommandConstant, testRemoteNameConstant, gitDeleteFlagConstant, cleanupBranchNameConstant}, execshell.ExecutionResult{ExitCode: 0}, nil)
				registerRepositoryResponse(executor, repositoryTwoPathConstant, gitCommandLabelConstant, []string{gitBranchSubcommandConstant, gitForceDeleteFlagConstant, cleanupBranchNameConstant}, execshell.ExecutionResult{ExitCode: 0}, nil)
			},
			expectedRepositories:        []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			expectedWarningRepositories: []string{repositoryOnePathConstant},
			verify: func(t *testing.T, executor *fakeCommandExecutor, logs []observer.LoggedEntry) {
				warnCount := 0
				for _, entry := range logs {
					if entry.Level == zap.WarnLevel {
						warnCount++
						repositoryValue, repositoryFound := entry.ContextMap()[repositoryLogFieldNameConstant]
						require.True(t, repositoryFound)
						require.Equal(t, repositoryOnePathConstant, repositoryValue)
					}
				}
				require.Equal(t, 1, warnCount)

				successfulCleanup := false
				for _, executedCommand := range executor.executedCommands {
					if executedCommand.workingDirectory == repositoryTwoPathConstant && executedCommand.arguments[0] == gitPushSubcommandConstant {
						successfulCleanup = true
					}
				}
				require.True(t, successfulCleanup)
			},
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(fmt.Sprintf(subtestNameTemplateConstant, testCaseIndex, testCase.name), func(subTest *testing.T) {
			fakeExecutorInstance := &fakeCommandExecutor{}
			if testCase.setup != nil {
				testCase.setup(subTest, fakeExecutorInstance)
			}

			fakeDiscoverer := &fakeRepositoryDiscoverer{repositories: append([]string{}, testCase.discoveredRepositories...)}

			logCore, observedLogs := observer.New(zap.DebugLevel)
			logger := zap.New(logCore)

			builder := branches.CommandBuilder{
				LoggerProvider:       func() *zap.Logger { return logger },
				Executor:             fakeExecutorInstance,
				RepositoryDiscoverer: fakeDiscoverer,
			}

			command, buildError := builder.Build()
			require.NoError(subTest, buildError)
			bindGlobalBranchFlags(command)

			command.SetContext(context.Background())
			command.SetArgs(testCase.arguments)

			executionError := command.Execute()
			require.NoError(subTest, executionError)

			require.Equal(subTest, testCase.expectedRoots, fakeDiscoverer.receivedRoots)

			uniqueWorkingDirectories := collectWorkingDirectories(fakeExecutorInstance.executedCommands)
			require.ElementsMatch(subTest, testCase.expectedRepositories, uniqueWorkingDirectories)

			if testCase.expectedWarningRepositories != nil {
				verifyWarnings(subTest, observedLogs.All(), testCase.expectedWarningRepositories)
			} else {
				verifyWarnings(subTest, observedLogs.All(), []string{})
			}

			if testCase.verify != nil {
				testCase.verify(subTest, fakeExecutorInstance, observedLogs.All())
			}
		})
	}
}

func TestCommandRunDisplaysHelpWhenRootsMissing(testInstance *testing.T) {
	fakeExecutorInstance := &fakeCommandExecutor{}
	fakeDiscoverer := &fakeRepositoryDiscoverer{}

	builder := branches.CommandBuilder{
		Executor:             fakeExecutorInstance,
		RepositoryDiscoverer: fakeDiscoverer,
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)
	bindGlobalBranchFlags(command)

	outputBuffer := &strings.Builder{}
	command.SetOut(outputBuffer)
	command.SetErr(outputBuffer)
	command.SetArgs([]string{commandDryRunFlagConstant})

	executionError := command.Execute()
	require.Error(testInstance, executionError)
	require.Equal(testInstance, rootutils.MissingRootsMessage(), executionError.Error())
	require.Contains(testInstance, outputBuffer.String(), command.UseLine())
}

func bindGlobalBranchFlags(command *cobra.Command) {
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
	flagutils.BindExecutionFlags(command, flagutils.ExecutionDefaults{}, flagutils.ExecutionFlagDefinitions{
		DryRun:    flagutils.ExecutionFlagDefinition{Name: flagutils.DryRunFlagName, Usage: flagutils.DryRunFlagUsage, Enabled: true},
		AssumeYes: flagutils.ExecutionFlagDefinition{Name: flagutils.AssumeYesFlagName, Usage: flagutils.AssumeYesFlagUsage, Shorthand: flagutils.AssumeYesFlagShorthand, Enabled: true},
	})
	flagutils.EnsureRemoteFlag(command, testDefaultRemoteNameConstant, testRemoteDescriptionConstant)
}

func TestCommandConfigurationPrecedence(testInstance *testing.T) {
	remoteBranches := []string{cleanupBranchNameConstant}
	remoteOutput := buildRemoteOutput(remoteBranches)

	pullRequestJSON, jsonError := buildPullRequestJSON(remoteBranches)
	require.NoError(testInstance, jsonError)

	testCases := []struct {
		name                 string
		configuration        branches.CommandConfiguration
		useConfiguration     bool
		arguments            []string
		expectedRoots        []string
		expectedRemote       string
		expectedLimit        int
		expectDryRun         bool
		expectError          bool
		expectedErrorMessage string
	}{
		{
			name: "configuration_values_apply",
			configuration: branches.CommandConfiguration{
				RemoteName:       configurationRemoteNameConstant,
				PullRequestLimit: 12,
				DryRun:           false,
				RepositoryRoots:  []string{configurationRootConstant},
			},
			useConfiguration: true,
			arguments:        []string{},
			expectedRoots:    []string{configurationRootConstant},
			expectedRemote:   configurationRemoteNameConstant,
			expectedLimit:    12,
		},
		{
			name: "flags_override_configuration",
			configuration: branches.CommandConfiguration{
				RemoteName:       configurationRemoteNameConstant,
				PullRequestLimit: 25,
				DryRun:           false,
				RepositoryRoots:  []string{configurationRootConstant},
			},
			useConfiguration: true,
			arguments: []string{
				commandRemoteFlagConstant,
				flagOverrideRemoteConstant,
				commandLimitFlagConstant,
				strconv.Itoa(flagOverrideLimitValueConstant),
				commandDryRunFlagConstant,
				commandRootFlagConstant,
				repositoryTwoPathConstant,
			},
			expectedRoots:  []string{repositoryTwoPathConstant},
			expectedRemote: flagOverrideRemoteConstant,
			expectedLimit:  flagOverrideLimitValueConstant,
			expectDryRun:   true,
		},
		{
			name:             "cli_defaults_apply_without_configuration",
			useConfiguration: false,
			arguments: []string{
				commandRootFlagConstant,
				repositoryOnePathConstant,
			},
			expectedRoots:  []string{repositoryOnePathConstant},
			expectedRemote: testRemoteNameConstant,
			expectedLimit:  defaultPullRequestLimitValueConstant,
		},
		{
			name: "configuration_missing_remote_returns_error",
			configuration: branches.CommandConfiguration{
				RemoteName:       whitespaceRemoteArgumentConstant,
				PullRequestLimit: 12,
				RepositoryRoots:  []string{configurationRootConstant},
			},
			useConfiguration:     true,
			arguments:            []string{},
			expectError:          true,
			expectedErrorMessage: invalidRemoteErrorMessageConstant,
		},
		{
			name: "configuration_invalid_limit_returns_error",
			configuration: branches.CommandConfiguration{
				RemoteName:       configurationRemoteNameConstant,
				PullRequestLimit: 0,
				RepositoryRoots:  []string{configurationRootConstant},
			},
			useConfiguration:     true,
			arguments:            []string{},
			expectError:          true,
			expectedErrorMessage: invalidLimitErrorMessageConstant,
		},
		{
			name:             "flag_invalid_remote_returns_error",
			useConfiguration: false,
			arguments: []string{
				commandRemoteFlagConstant,
				whitespaceRemoteArgumentConstant,
				commandLimitFlagConstant,
				commandLimitValueConstant,
				commandRootFlagConstant,
				multiRootFirstArgumentConstant,
			},
			expectError:          true,
			expectedErrorMessage: invalidRemoteErrorMessageConstant,
		},
		{
			name:             "flag_invalid_limit_returns_error",
			useConfiguration: false,
			arguments: []string{
				commandRemoteFlagConstant,
				testRemoteNameConstant,
				commandLimitFlagConstant,
				invalidLimitArgumentValueConstant,
				commandRootFlagConstant,
				multiRootFirstArgumentConstant,
			},
			expectError:          true,
			expectedErrorMessage: invalidLimitErrorMessageConstant,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		testInstance.Run(fmt.Sprintf(subtestNameTemplateConstant, testCaseIndex, testCase.name), func(subtest *testing.T) {
			fakeExecutorInstance := &fakeCommandExecutor{}

			if !testCase.expectError {
				gitListArguments := []string{gitListRemoteSubcommandConstant, gitHeadsFlagConstant, testCase.expectedRemote}
				registerResponse(fakeExecutorInstance, gitCommandLabelConstant, gitListArguments, execshell.ExecutionResult{StandardOutput: remoteOutput, ExitCode: 0}, nil)

				githubListArguments := []string{
					githubPullRequestSubcommandConstant,
					githubListSubcommandConstant,
					githubStateFlagConstant,
					githubClosedStateConstant,
					githubJSONFlagConstant,
					pullRequestJSONFieldNameConstant,
					githubLimitFlagConstant,
					strconv.Itoa(testCase.expectedLimit),
				}
				registerResponse(fakeExecutorInstance, githubCommandLabelConstant, githubListArguments, execshell.ExecutionResult{StandardOutput: pullRequestJSON, ExitCode: 0}, nil)

				if !testCase.expectDryRun {
					registerResponse(fakeExecutorInstance, gitCommandLabelConstant, []string{gitPushSubcommandConstant, testCase.expectedRemote, gitDeleteFlagConstant, cleanupBranchNameConstant}, execshell.ExecutionResult{ExitCode: 0}, nil)
					registerResponse(fakeExecutorInstance, gitCommandLabelConstant, []string{gitBranchSubcommandConstant, gitForceDeleteFlagConstant, cleanupBranchNameConstant}, execshell.ExecutionResult{ExitCode: 0}, nil)
				}
			}

			fakeDiscoverer := &fakeRepositoryDiscoverer{repositories: append([]string{}, testCase.expectedRoots...)}

			builder := branches.CommandBuilder{
				LoggerProvider:       func() *zap.Logger { return zap.NewNop() },
				Executor:             fakeExecutorInstance,
				RepositoryDiscoverer: fakeDiscoverer,
			}

			if testCase.useConfiguration {
				builder.ConfigurationProvider = func() branches.CommandConfiguration {
					return testCase.configuration
				}
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)
			bindGlobalBranchFlags(command)

			outputBuffer := &strings.Builder{}
			command.SetOut(outputBuffer)
			command.SetErr(outputBuffer)
			command.SetContext(context.Background())
			command.SetArgs(testCase.arguments)

			executionError := command.Execute()
			if testCase.expectError {
				require.Error(subtest, executionError)
				require.Equal(subtest, testCase.expectedErrorMessage, executionError.Error())
				require.Contains(subtest, outputBuffer.String(), command.UseLine())
				require.Empty(subtest, fakeDiscoverer.receivedRoots)
				return
			}

			require.NoError(subtest, executionError)
			require.Equal(subtest, testCase.expectedRoots, fakeDiscoverer.receivedRoots)

			if testCase.expectDryRun {
				for _, executed := range fakeExecutorInstance.executedCommands {
					require.NotEqual(subtest, gitPushSubcommandConstant, executed.arguments[0])
				}
			}
		})
	}
}

func collectWorkingDirectories(executedCommands []executedCommandRecord) []string {
	seen := make(map[string]struct{})
	var directories []string
	for _, commandRecord := range executedCommands {
		if _, alreadySeen := seen[commandRecord.workingDirectory]; alreadySeen {
			continue
		}
		seen[commandRecord.workingDirectory] = struct{}{}
		directories = append(directories, commandRecord.workingDirectory)
	}
	return directories
}

func verifyWarnings(testInstance *testing.T, logEntries []observer.LoggedEntry, expectedRepositories []string) {
	expectedSet := make(map[string]int)
	for _, repositoryPath := range expectedRepositories {
		expectedSet[repositoryPath]++
	}

	actualSet := make(map[string]int)
	for _, entry := range logEntries {
		if entry.Level != zap.WarnLevel {
			continue
		}
		repositoryValue, found := entry.ContextMap()[repositoryLogFieldNameConstant]
		require.True(testInstance, found)
		repositoryPath, ok := repositoryValue.(string)
		require.True(testInstance, ok)
		actualSet[repositoryPath]++
	}

	require.Equal(testInstance, expectedSet, actualSet)
}
