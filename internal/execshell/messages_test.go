package execshell

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildStartedMessageForFetchIncludesRemoteAndReferences(t *testing.T) {
	formatter := CommandMessageFormatter{}
	command := ShellCommand{
		Name: CommandGit,
		Details: CommandDetails{
			Arguments:        []string{"fetch", "--prune", "origin", "feature"},
			WorkingDirectory: "/workspace/repo",
		},
	}

	message := formatter.BuildStartedMessage(command)

	require.Equal(t, "Fetching feature from origin in /workspace/repo", message)
}

func TestBuildStartedMessageForFetchWithoutRemoteUsesAllRemotesLabel(t *testing.T) {
	formatter := CommandMessageFormatter{}
	command := ShellCommand{
		Name: CommandGit,
		Details: CommandDetails{
			Arguments:        []string{"fetch", "--prune"},
			WorkingDirectory: "/workspace/repo",
		},
	}

	message := formatter.BuildStartedMessage(command)

	require.Equal(t, "Fetching from all remotes in /workspace/repo", message)
}
