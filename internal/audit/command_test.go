package audit_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	audit "github.com/temirov/gix/internal/audit"
	"github.com/temirov/gix/internal/execshell"
	"github.com/temirov/gix/internal/githubcli"
)

const (
	auditMissingRootsErrorMessageConstant = "no repository roots provided; specify --root or configure defaults"
	auditWhitespaceRootArgumentConstant   = "   "
	auditRootFlagNameConstant             = "--root"
	auditConfigurationMissingSubtestName  = "configuration_and_flags_missing"
	auditWhitespaceRootFlagSubtestName    = "flag_provided_without_roots"
	auditTildeRootArgumentConstant        = "~/audit/repositories"
)

func TestCommandBuilderDisplaysHelpWhenRootsMissing(testInstance *testing.T) {
	testInstance.Parallel()

	testCases := []struct {
		name          string
		configuration audit.CommandConfiguration
		arguments     []string
	}{
		{
			name:          auditConfigurationMissingSubtestName,
			configuration: audit.CommandConfiguration{},
			arguments:     []string{},
		},
		{
			name:          auditWhitespaceRootFlagSubtestName,
			configuration: audit.CommandConfiguration{},
			arguments: []string{
				auditRootFlagNameConstant,
				auditWhitespaceRootArgumentConstant,
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			subTest.Parallel()

			builder := audit.CommandBuilder{
				LoggerProvider:        func() *zap.Logger { return zap.NewNop() },
				ConfigurationProvider: func() audit.CommandConfiguration { return testCase.configuration },
			}

			command, buildError := builder.Build()
			require.NoError(subTest, buildError)

			command.SetContext(context.Background())
			command.SetArgs(testCase.arguments)

			outputBuffer := &strings.Builder{}
			command.SetOut(outputBuffer)
			command.SetErr(outputBuffer)

			executionError := command.Execute()
			require.Error(subTest, executionError)
			require.Equal(subTest, auditMissingRootsErrorMessageConstant, executionError.Error())
			require.Contains(subTest, outputBuffer.String(), command.UseLine())
		})
	}
}

func TestCommandBuilderExpandsTildeRoots(testInstance *testing.T) {
	testInstance.Helper()

	homeDirectory, homeDirectoryError := os.UserHomeDir()
	require.NoError(testInstance, homeDirectoryError)

	expectedRoot := filepath.Join(homeDirectory, "audit", "repositories")

	repositoryDiscoverer := &repositoryDiscovererStub{}
	builder := audit.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		Discoverer:     repositoryDiscoverer,
		GitExecutor:    &gitExecutorStub{},
		GitManager:     &gitRepositoryManagerStub{},
		GitHubResolver: &gitHubResolverStub{},
		ConfigurationProvider: func() audit.CommandConfiguration {
			return audit.CommandConfiguration{}
		},
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{auditRootFlagNameConstant, auditTildeRootArgumentConstant})

	outputBuffer := &strings.Builder{}
	command.SetOut(outputBuffer)
	command.SetErr(outputBuffer)

	executionError := command.Execute()
	require.NoError(testInstance, executionError)
	require.Equal(testInstance, []string{expectedRoot}, repositoryDiscoverer.receivedRoots)
}

type repositoryDiscovererStub struct {
	receivedRoots []string
}

func (stub *repositoryDiscovererStub) DiscoverRepositories(roots []string) ([]string, error) {
	stub.receivedRoots = append([]string{}, roots...)
	return []string{}, nil
}

type gitExecutorStub struct{}

func (stub *gitExecutorStub) ExecuteGit(ctx context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

func (stub *gitExecutorStub) ExecuteGitHubCLI(ctx context.Context, details execshell.CommandDetails) (execshell.ExecutionResult, error) {
	return execshell.ExecutionResult{}, nil
}

type gitRepositoryManagerStub struct{}

func (stub *gitRepositoryManagerStub) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
	return true, nil
}

func (stub *gitRepositoryManagerStub) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
	return "main", nil
}

func (stub *gitRepositoryManagerStub) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
	return "https://github.com/example/repo.git", nil
}

func (stub *gitRepositoryManagerStub) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
	return nil
}

type gitHubResolverStub struct{}

func (stub *gitHubResolverStub) ResolveRepoMetadata(ctx context.Context, repository string) (githubcli.RepositoryMetadata, error) {
	return githubcli.RepositoryMetadata{}, nil
}
