package packages_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/ghcr"
	"github.com/temirov/git_scripts/internal/githubcli"
	packages "github.com/temirov/git_scripts/internal/packages"
)

func TestCommandBuilderParsesConfigurationDefaults(testingInstance *testing.T) {
	testingInstance.Parallel()

	configuration := packages.Configuration{Purge: packages.PurgeConfiguration{PackageName: "config-package", DryRun: true}}

	executor := &stubPurgeExecutor{result: ghcr.PurgeResult{TotalVersions: 1}}
	resolver := &stubServiceResolver{executor: executor}
	repositoryManager := &stubRepositoryManager{remoteURL: "https://github.com/source/example.git"}
	githubResolver := &stubGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: "canonical/example", IsInOrganization: true}}

	builder := packages.CommandBuilder{
		LoggerProvider:           func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider:    func() packages.Configuration { return configuration },
		ServiceResolver:          resolver,
		RepositoryManager:        repositoryManager,
		GitHubResolver:           githubResolver,
		WorkingDirectoryResolver: func() (string, error) { return "/tmp/repo", nil },
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{})
	executionError := command.Execute()
	require.NoError(testingInstance, executionError)

	require.True(testingInstance, executor.called)
	require.Equal(testingInstance, "canonical", executor.options.Owner)
	require.Equal(testingInstance, "config-package", executor.options.PackageName)
	require.Equal(testingInstance, ghcr.OrganizationOwnerType, executor.options.OwnerType)
	require.True(testingInstance, executor.options.DryRun)
	require.Equal(testingInstance, "GITHUB_PACKAGES_TOKEN", executor.options.TokenSource.Reference)
}

func TestCommandBuilderFlagOverrides(testingInstance *testing.T) {
	testingInstance.Parallel()

	configuration := packages.Configuration{Purge: packages.PurgeConfiguration{PackageName: "config-package"}}
	executor := &stubPurgeExecutor{result: ghcr.PurgeResult{TotalVersions: 2}}
	resolver := &stubServiceResolver{executor: executor}

	repositoryManager := &stubRepositoryManager{remoteURL: "https://github.com/source/example.git"}
	githubResolver := &stubGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: "canonical/example", IsInOrganization: true}}

	builder := packages.CommandBuilder{
		LoggerProvider:           func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider:    func() packages.Configuration { return configuration },
		ServiceResolver:          resolver,
		RepositoryManager:        repositoryManager,
		GitHubResolver:           githubResolver,
		WorkingDirectoryResolver: func() (string, error) { return "/tmp/repo", nil },
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	args := []string{
		"--package", "flag-package",
		"--dry-run",
	}
	command.SetArgs(args)
	executionError := command.Execute()
	require.NoError(testingInstance, executionError)

	require.True(testingInstance, executor.called)
	require.Equal(testingInstance, "canonical", executor.options.Owner)
	require.Equal(testingInstance, "flag-package", executor.options.PackageName)
	require.Equal(testingInstance, ghcr.OrganizationOwnerType, executor.options.OwnerType)
	require.Equal(testingInstance, packages.TokenSourceTypeEnvironment, executor.options.TokenSource.Type)
	require.Equal(testingInstance, "GITHUB_PACKAGES_TOKEN", executor.options.TokenSource.Reference)
	require.True(testingInstance, executor.options.DryRun)
}

func TestCommandBuilderHandlesExecutionError(testingInstance *testing.T) {
	testingInstance.Parallel()

	executor := &stubPurgeExecutor{err: errors.New("failure")}
	resolver := &stubServiceResolver{executor: executor}

	builder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration {
			return packages.Configuration{Purge: packages.PurgeConfiguration{PackageName: "config-package"}}
		},
		ServiceResolver:          resolver,
		RepositoryManager:        &stubRepositoryManager{remoteURL: "https://github.com/source/example.git"},
		GitHubResolver:           &stubGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: "canonical/example"}},
		WorkingDirectoryResolver: func() (string, error) { return "/tmp/repo", nil },
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{"--package", "p"})
	executionError := command.Execute()
	require.Error(testingInstance, executionError)
	require.ErrorContains(testingInstance, executionError, "repo-packages-purge failed")
}

func TestCommandBuilderValidatesArguments(testingInstance *testing.T) {
	testingInstance.Parallel()

	builder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration {
			return packages.Configuration{Purge: packages.PurgeConfiguration{PackageName: "config-package"}}
		},
		ServiceResolver:          &stubServiceResolver{executor: &stubPurgeExecutor{}},
		RepositoryManager:        &stubRepositoryManager{remoteURL: "https://github.com/source/example.git"},
		GitHubResolver:           &stubGitHubResolver{metadata: githubcli.RepositoryMetadata{NameWithOwner: "canonical/example"}},
		WorkingDirectoryResolver: func() (string, error) { return "/tmp/repo", nil },
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{"unexpected"})
	executionError := command.Execute()
	require.Error(testingInstance, executionError)
	require.ErrorContains(testingInstance, executionError, "does not accept positional arguments")
}

type stubServiceResolver struct {
	executor *stubPurgeExecutor
	err      error
}

func (resolver *stubServiceResolver) Resolve(logger *zap.Logger) (packages.PurgeExecutor, error) {
	if resolver.err != nil {
		return nil, resolver.err
	}
	return resolver.executor, nil
}

type stubPurgeExecutor struct {
	options packages.PurgeOptions
	result  ghcr.PurgeResult
	err     error
	called  bool
}

func (executor *stubPurgeExecutor) Execute(executionContext context.Context, options packages.PurgeOptions) (ghcr.PurgeResult, error) {
	executor.called = true
	executor.options = options
	if executor.err != nil {
		return ghcr.PurgeResult{}, executor.err
	}
	return executor.result, nil
}

type stubRepositoryManager struct {
	remoteURL string
}

func (manager *stubRepositoryManager) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
	return true, nil
}

func (manager *stubRepositoryManager) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
	return "main", nil
}

func (manager *stubRepositoryManager) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
	return manager.remoteURL, nil
}

func (manager *stubRepositoryManager) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
	return nil
}

type stubGitHubResolver struct {
	metadata           githubcli.RepositoryMetadata
	err                error
	recordedRepository string
}

func (resolver *stubGitHubResolver) ResolveRepoMetadata(ctx context.Context, repository string) (githubcli.RepositoryMetadata, error) {
	resolver.recordedRepository = repository
	if resolver.err != nil {
		return githubcli.RepositoryMetadata{}, resolver.err
	}
	return resolver.metadata, nil
}
