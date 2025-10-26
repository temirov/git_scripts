package release

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

type stubGitExecutor struct {
	recorded []execshell.CommandDetails
}

func (executor *stubGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.recorded = append(executor.recorded, details)
	return execshell.ExecutionResult{}, nil
}

func (executor *stubGitExecutor) ExecuteGitHubCLI(context.Context, execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func TestCommandBuilds(t *testing.T) {
	builder := CommandBuilder{}
	command, err := builder.Build()
	require.NoError(t, err)
	require.IsType(t, &cobra.Command{}, command)
	require.Equal(t, commandUsageTemplate, strings.TrimSpace(command.Use))
	require.NotEmpty(t, strings.TrimSpace(command.Example))
}

func TestCommandRequiresTagArgument(t *testing.T) {
	builder := CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() CommandConfiguration {
			return CommandConfiguration{}
		},
		GitExecutor: &stubGitExecutor{},
	}
	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
	require.Error(t, command.RunE(command, []string{}))
	require.Error(t, command.RunE(command, []string{"   "}))
}

func TestCommandRunsAcrossRoots(t *testing.T) {
	executor := &stubGitExecutor{}
	root := t.TempDir()
	builder := CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() CommandConfiguration {
			return CommandConfiguration{RepositoryRoots: []string{root}, RemoteName: "origin"}
		},
		GitExecutor: executor,
	}
	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	contextAccessor := utils.NewCommandContextAccessor()
	command.SetContext(contextAccessor.WithExecutionFlags(context.Background(), utils.ExecutionFlags{}))

	output := &bytes.Buffer{}
	command.SetOut(output)

	require.NoError(t, command.RunE(command, []string{"v1.2.3"}))
	require.NotEmpty(t, output.String())
	require.NotEmpty(t, executor.recorded)
}
