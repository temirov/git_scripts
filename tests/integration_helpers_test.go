package tests

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	integrationUnexpectedSuccessMessageConstant = "command succeeded unexpectedly"
	integrationUnexpectedSuccessFormatConstant  = "%s\n%s"
	integrationCommandFailureFormatConstant     = "command failed: %v\n%s"
)

func runIntegrationCommand(testInstance *testing.T, repositoryRoot string, pathVariable string, timeout time.Duration, arguments []string) string {
	testInstance.Helper()
	outputText, commandError := executeIntegrationCommand(testInstance, repositoryRoot, pathVariable, timeout, arguments)
	requireNoError(testInstance, commandError, outputText)
	return outputText
}

func runFailingIntegrationCommand(testInstance *testing.T, repositoryRoot string, pathVariable string, timeout time.Duration, arguments []string) (string, error) {
	testInstance.Helper()
	outputText, commandError := executeIntegrationCommand(testInstance, repositoryRoot, pathVariable, timeout, arguments)
	if commandError == nil {
		testInstance.Fatalf(integrationUnexpectedSuccessFormatConstant, integrationUnexpectedSuccessMessageConstant, outputText)
	}
	return outputText, commandError
}

func executeIntegrationCommand(testInstance *testing.T, repositoryRoot string, pathVariable string, timeout time.Duration, arguments []string) (string, error) {
	testInstance.Helper()
	executionContext, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(executionContext, "go", arguments...)
	command.Dir = repositoryRoot
	environment := append([]string{}, os.Environ()...)
	if len(pathVariable) > 0 {
		environment = append(environment, "PATH="+pathVariable)
	}
	command.Env = environment

	outputBytes, runError := command.CombinedOutput()
	outputText := string(outputBytes)
	return outputText, runError
}

func filterStructuredOutput(rawOutput string) string {
	lines := strings.Split(rawOutput, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "{") {
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 {
		return ""
	}
	return strings.Join(filtered, "\n") + "\n"
}

func requireNoError(testInstance *testing.T, err error, output string) {
	testInstance.Helper()
	if err != nil {
		testInstance.Fatalf(integrationCommandFailureFormatConstant, err, output)
	}
}
