package tests

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func runIntegrationCommand(testInstance *testing.T, repositoryRoot string, pathVariable string, timeout time.Duration, arguments []string) string {
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
	testInstance.Helper()
	requireNoError(testInstance, runError, outputText)
	return outputText
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
		testInstance.Fatalf("command failed: %v\n%s", err, output)
	}
}
