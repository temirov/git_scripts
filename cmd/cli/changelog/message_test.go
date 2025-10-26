package changelog

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	changeloggen "github.com/temirov/gix/internal/changelog"
	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
	"github.com/temirov/gix/pkg/llm"
)

func TestMessageCommandGeneratesChangelog(t *testing.T) {
	tempDir := t.TempDir()
	apiKeyEnv := "TEST_LLM_KEY"
	t.Setenv(apiKeyEnv, "test-api-key")

	executor := &fakeGitExecutor{
		responses: map[string]string{
			"describe --tags --abbrev=0": "v0.9.0\n",
			"log --no-merges --date=short --pretty=format:%h %ad %an %s --max-count=200 v0.9.0..HEAD": "abc123 2025-10-07 Alice Add feature\n",
			"diff --stat v0.9.0..HEAD":      " internal/app.go | 5 ++++-\n",
			"diff --unified=3 v0.9.0..HEAD": "diff --git a/internal/app.go b/internal/app.go\n",
		},
	}
	client := &fakeChatClient{response: "## [v1.0.0]\n\n### Features ✨\n- Highlight\n"}

	builder := MessageCommandBuilder{
		GitExecutor: executor,
		ConfigurationProvider: func() MessageConfiguration {
			return MessageConfiguration{
				Roots:          []string{tempDir},
				APIKeyEnv:      apiKeyEnv,
				Model:          "mock-model",
				SinceReference: "",
				SinceDate:      "",
				Version:        "v1.0.0",
			}.Sanitize()
		},
		ClientFactory: func(config llm.Config) (changeloggen.ChatClient, error) {
			client.config = config
			return client, nil
		},
	}

	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Name: flagutils.DefaultRootFlagName, Usage: flagutils.DefaultRootFlagUsage, Enabled: true})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetContext(context.Background())

	err = command.Execute()
	require.NoError(t, err)
	require.Contains(t, output.String(), "## [v1.0.0]")
	require.Equal(t, "mock-model", client.config.Model)
	require.Equal(t, "test-api-key", client.config.APIKey)
	require.Len(t, executor.calls, 4)
	require.NotNil(t, client.request)
	require.Contains(t, client.request.Messages[1].Content, "Release version: v1.0.0")
}

func TestMessageCommandDryRunWritesPrompt(t *testing.T) {
	tempDir := t.TempDir()
	apiKeyEnv := "TEST_LLM_KEY"
	t.Setenv(apiKeyEnv, "token")

	executor := &fakeGitExecutor{
		responses: map[string]string{
			"describe --tags --abbrev=0": "v0.9.0\n",
			"log --no-merges --date=short --pretty=format:%h %ad %an %s --max-count=200 v0.9.0..HEAD": "abc123 2025-10-07 Alice Add feature\n",
			"diff --stat v0.9.0..HEAD":      " internal/app.go | 5 ++++-\n",
			"diff --unified=3 v0.9.0..HEAD": "diff --git a/internal/app.go b/internal/app.go\n",
		},
	}
	client := &fakeChatClient{}

	builder := MessageCommandBuilder{
		GitExecutor: executor,
		ConfigurationProvider: func() MessageConfiguration {
			return MessageConfiguration{
				Roots:     []string{tempDir},
				APIKeyEnv: apiKeyEnv,
				Model:     "mock-model",
			}.Sanitize()
		},
		ClientFactory: func(config llm.Config) (changeloggen.ChatClient, error) {
			return client, nil
		},
	}

	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Name: flagutils.DefaultRootFlagName, Usage: flagutils.DefaultRootFlagUsage, Enabled: true})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)

	accessor := utils.NewCommandContextAccessor()
	command.SetContext(accessor.WithExecutionFlags(context.Background(), utils.ExecutionFlags{DryRun: true, DryRunSet: true}))

	err = command.Execute()
	require.NoError(t, err)
	result := output.String()
	require.Contains(t, result, "You are an expert release engineer creating Markdown changelog sections.")
	require.Contains(t, result, "Release version:")
	require.Nil(t, client.request)
}

func TestMessageCommandValidatesSinceInputs(t *testing.T) {
	tempDir := t.TempDir()
	apiKeyEnv := "TEST_LLM_KEY"
	t.Setenv(apiKeyEnv, "token")

	builder := MessageCommandBuilder{
		GitExecutor: &fakeGitExecutor{},
		ConfigurationProvider: func() MessageConfiguration {
			return MessageConfiguration{
				Roots:     []string{tempDir},
				APIKeyEnv: apiKeyEnv,
				Model:     "mock-model",
			}.Sanitize()
		},
		ClientFactory: func(config llm.Config) (changeloggen.ChatClient, error) {
			return &fakeChatClient{}, nil
		},
	}

	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Name: flagutils.DefaultRootFlagName, Usage: flagutils.DefaultRootFlagUsage, Enabled: true})
	command.SetContext(context.Background())

	command.SetArgs([]string{"--since-tag", "v0.1.0", "--since-date", "2025-10-07"})
	err = command.Execute()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "only one of --since-tag or --since-date"))
}

type fakeGitExecutor struct {
	responses map[string]string
	calls     [][]string
}

func (executor *fakeGitExecutor) ExecuteGit(ctx context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	key := strings.Join(details.Arguments, " ")
	executor.calls = append(executor.calls, details.Arguments)
	if executor.responses == nil {
		return execshell.ExecutionResult{}, nil
	}
	value, ok := executor.responses[key]
	if !ok {
		return execshell.ExecutionResult{}, nil
	}
	return execshell.ExecutionResult{StandardOutput: value}, nil
}

func (executor *fakeGitExecutor) ExecuteGitHubCLI(ctx context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type fakeChatClient struct {
	config   llm.Config
	response string
	err      error
	request  *llm.ChatRequest
}

func (client *fakeChatClient) Chat(ctx context.Context, request llm.ChatRequest) (string, error) {
	clientCopy := request
	client.request = &clientCopy
	if client.err != nil {
		return "", client.err
	}
	return client.response, nil
}
