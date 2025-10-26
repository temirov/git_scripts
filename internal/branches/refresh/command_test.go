package refresh_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/gix/internal/branches/refresh"
	"github.com/temirov/gix/internal/execshell"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

type recordingGitExecutor struct {
	invocationErrors []error
	recordedCommands []execshell.CommandDetails
}

func (executor *recordingGitExecutor) ExecuteGit(_ context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	executor.recordedCommands = append(executor.recordedCommands, details)
	if len(executor.invocationErrors) == 0 {
		return execshell.ExecutionResult{}, nil
	}
	err := executor.invocationErrors[0]
	executor.invocationErrors = executor.invocationErrors[1:]
	if err != nil {
		return execshell.ExecutionResult{}, err
	}
	return execshell.ExecutionResult{}, nil
}

func (executor *recordingGitExecutor) ExecuteGitHubCLI(_ context.Context, _ execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type constantCleanRepositoryManager struct{}

type erroringRepositoryManager struct{}

func (constantCleanRepositoryManager) CheckCleanWorktree(context.Context, string) (bool, error) {
	return true, nil
}

func (constantCleanRepositoryManager) GetCurrentBranch(context.Context, string) (string, error) {
	return "", nil
}

func (constantCleanRepositoryManager) GetRemoteURL(context.Context, string, string) (string, error) {
	return "", nil
}

func (constantCleanRepositoryManager) SetRemoteURL(context.Context, string, string, string) error {
	return nil
}

func (erroringRepositoryManager) CheckCleanWorktree(context.Context, string) (bool, error) {
	return false, nil
}

func (erroringRepositoryManager) GetCurrentBranch(context.Context, string) (string, error) {
	return "", nil
}

func (erroringRepositoryManager) GetRemoteURL(context.Context, string, string) (string, error) {
	return "", nil
}

func (erroringRepositoryManager) SetRemoteURL(context.Context, string, string, string) error {
	return nil
}

func TestBuildReturnsCommand(t *testing.T) {
	builder := refresh.CommandBuilder{}
	command, buildError := builder.Build()
	require.NoError(t, buildError)
	require.IsType(t, &cobra.Command{}, command)
}

func TestCommandRequiresBranchName(t *testing.T) {
	builder := refresh.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() refresh.CommandConfiguration {
			return refresh.CommandConfiguration{}
		},
		GitExecutor:          &recordingGitExecutor{},
		GitRepositoryManager: constantCleanRepositoryManager{},
	}
	command, buildError := builder.Build()
	require.NoError(t, buildError)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})
	require.Error(t, command.RunE(command, []string{}))
}

func TestCommandRunsSuccessfully(t *testing.T) {
	temporaryRepository := t.TempDir()
	executor := &recordingGitExecutor{}
	builder := refresh.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() refresh.CommandConfiguration {
			return refresh.CommandConfiguration{RepositoryRoots: []string{temporaryRepository}}
		},
		GitExecutor:          executor,
		GitRepositoryManager: constantCleanRepositoryManager{},
	}
	command, buildError := builder.Build()
	require.NoError(t, buildError)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	require.NoError(t, command.Flags().Set("branch", "main"))

	outputBuffer := &bytes.Buffer{}
	command.SetOut(outputBuffer)

	require.NoError(t, command.RunE(command, []string{}))
	require.Contains(t, outputBuffer.String(), temporaryRepository)
	require.Contains(t, outputBuffer.String(), "main")
	require.Len(t, executor.recordedCommands, 3)
}

func TestCommandReportsDirtyWorktree(t *testing.T) {
	temporaryRepository := t.TempDir()
	builder := refresh.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() refresh.CommandConfiguration {
			return refresh.CommandConfiguration{RepositoryRoots: []string{temporaryRepository}, BranchName: "main"}
		},
		GitExecutor:          &recordingGitExecutor{},
		GitRepositoryManager: erroringRepositoryManager{},
	}
	command, buildError := builder.Build()
	require.NoError(t, buildError)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	require.Error(t, command.RunE(command, []string{}))
}

func TestCommandRejectsConflictingFlags(t *testing.T) {
	temporaryRepository := t.TempDir()
	builder := refresh.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() refresh.CommandConfiguration {
			return refresh.CommandConfiguration{RepositoryRoots: []string{temporaryRepository}, BranchName: "main"}
		},
		GitExecutor:          &recordingGitExecutor{},
		GitRepositoryManager: constantCleanRepositoryManager{},
	}
	command, buildError := builder.Build()
	require.NoError(t, buildError)
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Enabled: true})

	require.NoError(t, command.Flags().Set("stash", "true"))
	require.NoError(t, command.Flags().Set("commit", "true"))

	require.Error(t, command.RunE(command, []string{}))
}
