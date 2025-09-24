package execshell_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	testExecutionSuccessCaseNameConstant                   = "success"
	testExecutionFailureCaseNameConstant                   = "failure_exit_code"
	testExecutionRunnerErrorCaseNameConstant               = "runner_error"
	testGitWrapperCaseNameConstant                         = "git_wrapper"
	testGitHubWrapperCaseNameConstant                      = "github_wrapper"
	testCurlWrapperCaseNameConstant                        = "curl_wrapper"
	testGitSubcommandConstant                              = "rev-parse"
	testGitWorktreeFlagConstant                            = "--is-inside-work-tree"
	testRepositoryWorkingDirectoryConstant                 = "/workspace/repository"
	testGenericCommandArgumentConstant                     = "--version"
	testGenericWorkingDirectoryConstant                    = "."
	testSuccessfulCommandOutputConstant                    = "ok"
	testStandardErrorOutputConstant                        = "failure"
	testRunnerFailureMessageConstant                       = "runner failure"
	testLoggerInitializationCaseNameConstant               = "logger_validation"
	testRunnerInitializationCaseNameConstant               = "runner_validation"
	testSuccessfulInitializationCaseNameConstant           = "successful_initialization"
	testRepositoryAnalysisStartMessageConstant             = "Analyzing repository at /workspace/repository"
	testRepositoryAnalysisSuccessMessageConstant           = "/workspace/repository is a Git repository"
	testRepositoryAnalysisFailureMessageTemplateConstant   = "Could not confirm /workspace/repository is a Git repository (exit code %d: %s)"
	testRepositoryAnalysisExecutionFailureTemplateConstant = "Could not analyze /workspace/repository: %s"
	testGenericCommandStartMessageConstant                 = "Running git --version (in .)"
	testGenericCommandSuccessMessageConstant               = "Completed git --version (in .)"
	testGenericCommandFailureMessageTemplateConstant       = "git --version (in .) failed with exit code %d: %s"
	testGenericCommandExecutionFailureTemplateConstant     = "git --version (in .) failed: %s"
)

type recordingCommandRunner struct {
	executionResult  execshell.ExecutionResult
	executionError   error
	recordedCommands []execshell.ShellCommand
}

func (runner *recordingCommandRunner) Run(executionContext context.Context, command execshell.ShellCommand) (execshell.ExecutionResult, error) {
	runner.recordedCommands = append(runner.recordedCommands, command)
	return runner.executionResult, runner.executionError
}

func TestShellExecutorInitializationValidation(testInstance *testing.T) {
	testCases := []struct {
		name          string
		logger        *zap.Logger
		runner        execshell.CommandRunner
		expectError   error
		expectSuccess bool
	}{
		{
			name:        testLoggerInitializationCaseNameConstant,
			logger:      nil,
			runner:      &recordingCommandRunner{},
			expectError: execshell.ErrLoggerNotConfigured,
		},
		{
			name:        testRunnerInitializationCaseNameConstant,
			logger:      zap.NewNop(),
			runner:      nil,
			expectError: execshell.ErrCommandRunnerNotConfigured,
		},
		{
			name:          testSuccessfulInitializationCaseNameConstant,
			logger:        zap.NewNop(),
			runner:        &recordingCommandRunner{},
			expectSuccess: true,
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			executor, creationError := execshell.NewShellExecutor(testCase.logger, testCase.runner, false)
			if testCase.expectSuccess {
				require.NoError(testInstance, creationError)
				require.NotNil(testInstance, executor)
			} else {
				require.Error(testInstance, creationError)
				require.ErrorIs(testInstance, creationError, testCase.expectError)
			}
		})
	}
}

func TestShellExecutorExecuteBehavior(testInstance *testing.T) {
	testCases := []struct {
		name             string
		runnerResult     execshell.ExecutionResult
		runnerError      error
		expectErrorType  any
		expectedLogCount int
	}{
		{
			name: testExecutionSuccessCaseNameConstant,
			runnerResult: execshell.ExecutionResult{
				StandardOutput: testSuccessfulCommandOutputConstant,
				ExitCode:       0,
			},
			expectedLogCount: 2,
		},
		{
			name: testExecutionFailureCaseNameConstant,
			runnerResult: execshell.ExecutionResult{
				StandardError: testStandardErrorOutputConstant,
				ExitCode:      1,
			},
			expectErrorType:  execshell.CommandFailedError{},
			expectedLogCount: 2,
		},
		{
			name:             testExecutionRunnerErrorCaseNameConstant,
			runnerError:      errors.New(testRunnerFailureMessageConstant),
			expectErrorType:  execshell.CommandExecutionError{},
			expectedLogCount: 2,
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			observerCore, observerLogs := observer.New(zap.DebugLevel)
			logger := zap.New(observerCore)

			recordingRunner := &recordingCommandRunner{
				executionResult: testCase.runnerResult,
				executionError:  testCase.runnerError,
			}

			shellExecutor, creationError := execshell.NewShellExecutor(logger, recordingRunner, false)
			require.NoError(testInstance, creationError)

			commandDetails := execshell.CommandDetails{Arguments: []string{testGitSubcommandConstant, testGitWorktreeFlagConstant}, WorkingDirectory: testRepositoryWorkingDirectoryConstant}
			executionResult, executionError := shellExecutor.ExecuteGit(context.Background(), commandDetails)

			if testCase.expectErrorType != nil {
				require.Error(testInstance, executionError)
				require.IsType(testInstance, testCase.expectErrorType, executionError)
				require.Empty(testInstance, executionResult.StandardOutput)
			} else {
				require.NoError(testInstance, executionError)
				require.Equal(testInstance, testCase.runnerResult.StandardOutput, executionResult.StandardOutput)
			}

			require.Len(testInstance, observerLogs.All(), testCase.expectedLogCount)
		})
	}
}

func TestShellExecutorHumanReadableLogging(testInstance *testing.T) {
	testCases := []struct {
		name             string
		runnerResult     execshell.ExecutionResult
		runnerError      error
		commandDetails   execshell.CommandDetails
		expectedMessages []string
		expectedLevels   []zapcore.Level
	}{
		{
			name:         testExecutionSuccessCaseNameConstant + "_rev_parse",
			runnerResult: execshell.ExecutionResult{StandardOutput: testSuccessfulCommandOutputConstant, ExitCode: 0},
			commandDetails: execshell.CommandDetails{
				Arguments:        []string{testGitSubcommandConstant, testGitWorktreeFlagConstant},
				WorkingDirectory: testRepositoryWorkingDirectoryConstant,
			},
			expectedMessages: []string{
				testRepositoryAnalysisStartMessageConstant,
				testRepositoryAnalysisSuccessMessageConstant,
			},
			expectedLevels: []zapcore.Level{zap.InfoLevel, zap.InfoLevel},
		},
		{
			name:         testExecutionFailureCaseNameConstant + "_rev_parse",
			runnerResult: execshell.ExecutionResult{StandardError: testStandardErrorOutputConstant, ExitCode: 1},
			commandDetails: execshell.CommandDetails{
				Arguments:        []string{testGitSubcommandConstant, testGitWorktreeFlagConstant},
				WorkingDirectory: testRepositoryWorkingDirectoryConstant,
			},
			expectedMessages: []string{
				testRepositoryAnalysisStartMessageConstant,
				fmt.Sprintf(testRepositoryAnalysisFailureMessageTemplateConstant, 1, testStandardErrorOutputConstant),
			},
			expectedLevels: []zapcore.Level{zap.InfoLevel, zap.WarnLevel},
		},
		{
			name:        testExecutionRunnerErrorCaseNameConstant + "_rev_parse",
			runnerError: errors.New(testRunnerFailureMessageConstant),
			commandDetails: execshell.CommandDetails{
				Arguments:        []string{testGitSubcommandConstant, testGitWorktreeFlagConstant},
				WorkingDirectory: testRepositoryWorkingDirectoryConstant,
			},
			expectedMessages: []string{
				testRepositoryAnalysisStartMessageConstant,
				fmt.Sprintf(testRepositoryAnalysisExecutionFailureTemplateConstant, testRunnerFailureMessageConstant),
			},
			expectedLevels: []zapcore.Level{zap.InfoLevel, zap.ErrorLevel},
		},
		{
			name:         testExecutionSuccessCaseNameConstant + "_generic",
			runnerResult: execshell.ExecutionResult{StandardOutput: testSuccessfulCommandOutputConstant, ExitCode: 0},
			commandDetails: execshell.CommandDetails{
				Arguments:        []string{testGenericCommandArgumentConstant},
				WorkingDirectory: testGenericWorkingDirectoryConstant,
			},
			expectedMessages: []string{
				testGenericCommandStartMessageConstant,
				testGenericCommandSuccessMessageConstant,
			},
			expectedLevels: []zapcore.Level{zap.InfoLevel, zap.InfoLevel},
		},
		{
			name:         testExecutionFailureCaseNameConstant + "_generic",
			runnerResult: execshell.ExecutionResult{StandardError: testStandardErrorOutputConstant, ExitCode: 1},
			commandDetails: execshell.CommandDetails{
				Arguments:        []string{testGenericCommandArgumentConstant},
				WorkingDirectory: testGenericWorkingDirectoryConstant,
			},
			expectedMessages: []string{
				testGenericCommandStartMessageConstant,
				fmt.Sprintf(testGenericCommandFailureMessageTemplateConstant, 1, testStandardErrorOutputConstant),
			},
			expectedLevels: []zapcore.Level{zap.InfoLevel, zap.WarnLevel},
		},
		{
			name:        testExecutionRunnerErrorCaseNameConstant + "_generic",
			runnerError: errors.New(testRunnerFailureMessageConstant),
			commandDetails: execshell.CommandDetails{
				Arguments:        []string{testGenericCommandArgumentConstant},
				WorkingDirectory: testGenericWorkingDirectoryConstant,
			},
			expectedMessages: []string{
				testGenericCommandStartMessageConstant,
				fmt.Sprintf(testGenericCommandExecutionFailureTemplateConstant, testRunnerFailureMessageConstant),
			},
			expectedLevels: []zapcore.Level{zap.InfoLevel, zap.ErrorLevel},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			observerCore, observedLogs := observer.New(zap.InfoLevel)
			logger := zap.New(observerCore)

			recordingRunner := &recordingCommandRunner{
				executionResult: testCase.runnerResult,
				executionError:  testCase.runnerError,
			}

			shellExecutor, creationError := execshell.NewShellExecutor(logger, recordingRunner, true)
			require.NoError(testInstance, creationError)

			_, _ = shellExecutor.ExecuteGit(context.Background(), testCase.commandDetails)

			capturedLogs := observedLogs.All()
			require.Len(testInstance, capturedLogs, len(testCase.expectedMessages))
			for logIndex := range capturedLogs {
				require.Equal(testInstance, testCase.expectedMessages[logIndex], capturedLogs[logIndex].Message)
				require.Equal(testInstance, testCase.expectedLevels[logIndex], capturedLogs[logIndex].Level)
			}
		})
	}
}

func TestShellExecutorWrappersSetCommandNames(testInstance *testing.T) {
	observerCore, _ := observer.New(zap.DebugLevel)
	logger := zap.New(observerCore)

	testCases := []struct {
		name            string
		invoke          func(executor *execshell.ShellExecutor) error
		expectedCommand execshell.CommandName
	}{
		{
			name: testGitWrapperCaseNameConstant,
			invoke: func(executor *execshell.ShellExecutor) error {
				_, executionError := executor.ExecuteGit(context.Background(), execshell.CommandDetails{})
				return executionError
			},
			expectedCommand: execshell.CommandGit,
		},
		{
			name: testGitHubWrapperCaseNameConstant,
			invoke: func(executor *execshell.ShellExecutor) error {
				_, executionError := executor.ExecuteGitHubCLI(context.Background(), execshell.CommandDetails{})
				return executionError
			},
			expectedCommand: execshell.CommandGitHub,
		},
		{
			name: testCurlWrapperCaseNameConstant,
			invoke: func(executor *execshell.ShellExecutor) error {
				_, executionError := executor.ExecuteCurl(context.Background(), execshell.CommandDetails{})
				return executionError
			},
			expectedCommand: execshell.CommandCurl,
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			recordingRunner := &recordingCommandRunner{
				executionResult: execshell.ExecutionResult{ExitCode: 1},
			}

			executor, creationError := execshell.NewShellExecutor(logger, recordingRunner, false)
			require.NoError(testInstance, creationError)

			executionError := testCase.invoke(executor)
			require.Error(testInstance, executionError)
			require.Len(testInstance, recordingRunner.recordedCommands, 1)
			recordedCommand := recordingRunner.recordedCommands[0]
			require.Equal(testInstance, testCase.expectedCommand, recordedCommand.Name)
		})
	}
}
