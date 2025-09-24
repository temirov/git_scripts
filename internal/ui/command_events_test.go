package ui_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/temirov/git_scripts/internal/execshell"
	"github.com/temirov/git_scripts/internal/ui"
)

const (
	testCommandWorkingDirectoryConstant     = "/tmp/project"
	testCommandArgumentConstant             = "--prune"
	testCommandNameFieldExpectationConstant = "git --prune (in /tmp/project)"
	testExecutionFailureReasonConstant      = "execution failed"
	testStandardErrorMessageConstant        = "fatal: remote error"
	testStartMessageExpectationConstant     = "Running " + testCommandNameFieldExpectationConstant
	testSuccessMessageExpectationConstant   = "Completed " + testCommandNameFieldExpectationConstant
	testFailureMessageExpectationConstant   = testCommandNameFieldExpectationConstant + " failed with exit code 1: " + testStandardErrorMessageConstant
	testExecutionFailureMessageExpectation  = testCommandNameFieldExpectationConstant + " failed: " + testExecutionFailureReasonConstant
)

func TestConsoleCommandEventLoggerEmitsMessages(testInstance *testing.T) {
	command := execshell.ShellCommand{
		Name: execshell.CommandGit,
		Details: execshell.CommandDetails{
			Arguments:        []string{testCommandArgumentConstant},
			WorkingDirectory: testCommandWorkingDirectoryConstant,
		},
	}

	testCases := []struct {
		name            string
		invoke          func(logger *ui.ConsoleCommandEventLogger)
		expectedLevel   zapcore.Level
		expectedMessage string
	}{
		{
			name: "command_started",
			invoke: func(logger *ui.ConsoleCommandEventLogger) {
				logger.CommandStarted(command)
			},
			expectedLevel:   zapcore.InfoLevel,
			expectedMessage: testStartMessageExpectationConstant,
		},
		{
			name: "command_completed_success",
			invoke: func(logger *ui.ConsoleCommandEventLogger) {
				logger.CommandCompleted(command, execshell.ExecutionResult{ExitCode: 0})
			},
			expectedLevel:   zapcore.InfoLevel,
			expectedMessage: testSuccessMessageExpectationConstant,
		},
		{
			name: "command_completed_failure",
			invoke: func(logger *ui.ConsoleCommandEventLogger) {
				logger.CommandCompleted(command, execshell.ExecutionResult{ExitCode: 1, StandardError: testStandardErrorMessageConstant})
			},
			expectedLevel:   zapcore.WarnLevel,
			expectedMessage: testFailureMessageExpectationConstant,
		},
		{
			name: "command_execution_failure",
			invoke: func(logger *ui.ConsoleCommandEventLogger) {
				logger.CommandExecutionFailed(command, errors.New(testExecutionFailureReasonConstant))
			},
			expectedLevel:   zapcore.ErrorLevel,
			expectedMessage: testExecutionFailureMessageExpectation,
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(testInstance *testing.T) {
			observerCore, observedLogs := observer.New(zapcore.DebugLevel)
			consoleLogger := zap.New(observerCore)
			eventLogger := ui.NewConsoleCommandEventLogger(consoleLogger)

			testCase.invoke(eventLogger)

			entries := observedLogs.All()
			require.Len(testInstance, entries, 1)
			require.Equal(testInstance, testCase.expectedLevel, entries[0].Level)
			require.Equal(testInstance, testCase.expectedMessage, entries[0].Message)
		})
	}
}
