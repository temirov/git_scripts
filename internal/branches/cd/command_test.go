package cd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

type recordingExecutor struct {
	stubGitExecutor
}

func TestCommandBuilds(t *testing.T) {
	builder := CommandBuilder{}
	command, err := builder.Build()
	require.NoError(t, err)
	require.IsType(t, &cobra.Command{}, command)
}

func TestCommandUsageIncludesBranchPlaceholder(t *testing.T) {
	builder := CommandBuilder{}
	command, err := builder.Build()
	require.NoError(t, err)
	require.Contains(t, command.Use, "<branch>")
}

func TestCommandRequiresBranchArgument(t *testing.T) {
	builder := CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() CommandConfiguration {
			return CommandConfiguration{}
		},
		GitExecutor: &recordingExecutor{},
	}
	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
	require.Error(t, command.RunE(command, []string{}))
}

func TestCommandExecutesAcrossRoots(t *testing.T) {
	temporaryRoot := t.TempDir()
	executor := &recordingExecutor{}
	builder := CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() CommandConfiguration {
			return CommandConfiguration{RepositoryRoots: []string{temporaryRoot}, RemoteName: "origin"}
		},
		GitExecutor: executor,
	}
	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	contextAccessor := utils.NewCommandContextAccessor()
	command.SetContext(contextAccessor.WithExecutionFlags(context.Background(), utils.ExecutionFlags{DryRun: false}))

	output := &bytes.Buffer{}
	command.SetOut(output)

	require.NoError(t, command.RunE(command, []string{"feature/foo"}))
	require.GreaterOrEqual(t, len(executor.recorded), 3)
	require.Contains(t, output.String(), temporaryRoot)
	require.Contains(t, output.String(), "feature/foo")
}

func TestCommandUsesBranchContextWhenBranchArgumentMissing(t *testing.T) {
	temporaryRoot := t.TempDir()
	executor := &recordingExecutor{}
	builder := CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() CommandConfiguration {
			return CommandConfiguration{RepositoryRoots: []string{temporaryRoot}, RemoteName: "origin"}
		},
		GitExecutor: executor,
	}
	command, err := builder.Build()
	require.NoError(t, err)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	contextAccessor := utils.NewCommandContextAccessor()
	command.SetContext(contextAccessor.WithBranchContext(context.Background(), utils.BranchContext{Name: "feature/context", RequireClean: true}))

	output := &bytes.Buffer{}
	command.SetOut(output)

	require.NoError(t, command.RunE(command, []string{}))
	require.Contains(t, output.String(), "feature/context")
	require.NotEmpty(t, executor.recorded)
}
