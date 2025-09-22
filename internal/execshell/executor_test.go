package execshell_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/temirov/git_scripts/internal/execshell"
)

const (
	testExecutionSuccessCaseNameConstant         = "success"
	testExecutionFailureCaseNameConstant         = "failure_exit_code"
	testExecutionRunnerErrorCaseNameConstant     = "runner_error"
	testGitWrapperCaseNameConstant               = "git_wrapper"
	testGitHubWrapperCaseNameConstant            = "github_wrapper"
	testCurlWrapperCaseNameConstant              = "curl_wrapper"
	testCommandArgumentConstant                  = "--version"
	testWorkingDirectoryConstant                 = "."
	testStandardErrorOutputConstant              = "failure"
	testLoggerInitializationCaseNameConstant     = "logger_validation"
	testRunnerInitializationCaseNameConstant     = "runner_validation"
	testSuccessfulInitializationCaseNameConstant = "successful_initialization"
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
			executor, creationError := execshell.NewShellExecutor(testCase.logger, testCase.runner)
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
				StandardOutput: "ok",
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
			runnerError:      errors.New("runner failure"),
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

			shellExecutor, creationError := execshell.NewShellExecutor(logger, recordingRunner)
			require.NoError(testInstance, creationError)

			commandDetails := execshell.CommandDetails{Arguments: []string{testCommandArgumentConstant}, WorkingDirectory: testWorkingDirectoryConstant}
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

			executor, creationError := execshell.NewShellExecutor(logger, recordingRunner)
			require.NoError(testInstance, creationError)

			executionError := testCase.invoke(executor)
			require.Error(testInstance, executionError)
			require.Len(testInstance, recordingRunner.recordedCommands, 1)
			recordedCommand := recordingRunner.recordedCommands[0]
			require.Equal(testInstance, testCase.expectedCommand, recordedCommand.Name)
		})
	}
}
