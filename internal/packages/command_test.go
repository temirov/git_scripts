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

const (
	configurationRootOnePathConstant      = "/config/root"
	configurationRootTwoPathConstant      = "/config/alternate"
	flagRootPathConstant                  = "/flag/root"
	workingDirectoryPathConstant          = "/working/directory"
	discoveredRepositoryOnePathConstant   = "/repositories/one"
	discoveredRepositoryTwoPathConstant   = "/repositories/two"
	discoveredRepositoryThreePathConstant = "/repositories/three"
	repositoryOneIdentifierConstant       = "source/example"
	repositoryTwoIdentifierConstant       = "source/example-two"
	repositoryThreeIdentifierConstant     = "source/example-three"
	repositoryOneRemoteURLConstant        = "https://github.com/source/example.git"
	repositoryTwoRemoteURLConstant        = "https://github.com/source/example-two.git"
	repositoryThreeRemoteURLConstant      = "https://github.com/source/example-three.git"
	repositoryOneOwnerConstant            = "canonical"
	repositoryTwoOwnerConstant            = "second-owner"
	repositoryThreeOwnerConstant          = "third-owner"
)

func TestCommandBuilderExecutesAcrossRepositories(testInstance *testing.T) {
	testInstance.Parallel()

	testCases := []struct {
		name                   string
		configuration          packages.Configuration
		arguments              []string
		discoveredRepositories []string
		remoteURLsByRepository map[string]string
		metadataByRepository   map[string]githubcli.RepositoryMetadata
		expectedPackage        string
		expectedDryRun         bool
		expectedOwners         []string
		expectedOwnerTypes     []ghcr.OwnerType
		expectedRoots          []string
	}{
		{
			name: "configuration_defaults",
			configuration: packages.Configuration{Purge: packages.PurgeConfiguration{
				PackageName:     "config-package",
				DryRun:          true,
				RepositoryRoots: []string{configurationRootOnePathConstant},
			}},
			arguments: []string{},
			discoveredRepositories: []string{
				discoveredRepositoryOnePathConstant,
				discoveredRepositoryTwoPathConstant,
			},
			remoteURLsByRepository: map[string]string{
				discoveredRepositoryOnePathConstant: repositoryOneRemoteURLConstant,
				discoveredRepositoryTwoPathConstant: repositoryTwoRemoteURLConstant,
			},
			metadataByRepository: map[string]githubcli.RepositoryMetadata{
				repositoryOneIdentifierConstant: {NameWithOwner: repositoryOneOwnerConstant + "/ignored", IsInOrganization: true},
				repositoryTwoIdentifierConstant: {NameWithOwner: repositoryTwoOwnerConstant + "/ignored", IsInOrganization: false},
			},
			expectedPackage:    "config-package",
			expectedDryRun:     true,
			expectedOwners:     []string{repositoryOneOwnerConstant, repositoryTwoOwnerConstant},
			expectedOwnerTypes: []ghcr.OwnerType{ghcr.OrganizationOwnerType, ghcr.UserOwnerType},
			expectedRoots:      []string{configurationRootOnePathConstant},
		},
		{
			name: "flag_overrides_configuration",
			configuration: packages.Configuration{Purge: packages.PurgeConfiguration{
				PackageName:     "config-package",
				DryRun:          false,
				RepositoryRoots: []string{configurationRootTwoPathConstant},
			}},
			arguments: []string{
				"--package", "flag-package",
				"--dry-run",
				"--roots", flagRootPathConstant,
			},
			discoveredRepositories: []string{
				discoveredRepositoryThreePathConstant,
			},
			remoteURLsByRepository: map[string]string{
				discoveredRepositoryThreePathConstant: repositoryThreeRemoteURLConstant,
			},
			metadataByRepository: map[string]githubcli.RepositoryMetadata{
				repositoryThreeIdentifierConstant: {NameWithOwner: repositoryThreeOwnerConstant + "/ignored", IsInOrganization: true},
			},
			expectedPackage:    "flag-package",
			expectedDryRun:     true,
			expectedOwners:     []string{repositoryThreeOwnerConstant},
			expectedOwnerTypes: []ghcr.OwnerType{ghcr.OrganizationOwnerType},
			expectedRoots:      []string{flagRootPathConstant},
		},
		{
			name: "falls_back_to_working_directory",
			configuration: packages.Configuration{Purge: packages.PurgeConfiguration{
				PackageName: "config-package",
			}},
			arguments: []string{},
			discoveredRepositories: []string{
				discoveredRepositoryOnePathConstant,
			},
			remoteURLsByRepository: map[string]string{
				discoveredRepositoryOnePathConstant: repositoryOneRemoteURLConstant,
			},
			metadataByRepository: map[string]githubcli.RepositoryMetadata{
				repositoryOneIdentifierConstant: {NameWithOwner: repositoryOneOwnerConstant + "/ignored", IsInOrganization: true},
			},
			expectedPackage:    "config-package",
			expectedDryRun:     false,
			expectedOwners:     []string{repositoryOneOwnerConstant},
			expectedOwnerTypes: []ghcr.OwnerType{ghcr.OrganizationOwnerType},
			expectedRoots:      []string{workingDirectoryPathConstant},
		},
	}

	for _, testCase := range testCases {
		testInstance.Run(testCase.name, func(subTest *testing.T) {
			subTest.Parallel()

			executor := &stubPurgeExecutor{result: ghcr.PurgeResult{TotalVersions: 1}}
			resolver := &stubServiceResolver{executor: executor}
			repositoryManager := &stubRepositoryManager{remoteURLByPath: testCase.remoteURLsByRepository}
			githubResolver := &stubGitHubResolver{metadataByRepository: testCase.metadataByRepository}
			discoverer := &stubRepositoryDiscoverer{repositories: testCase.discoveredRepositories}

			builder := packages.CommandBuilder{
				LoggerProvider:           func() *zap.Logger { return zap.NewNop() },
				ConfigurationProvider:    func() packages.Configuration { return testCase.configuration },
				ServiceResolver:          resolver,
				RepositoryManager:        repositoryManager,
				GitHubResolver:           githubResolver,
				RepositoryDiscoverer:     discoverer,
				WorkingDirectoryResolver: func() (string, error) { return workingDirectoryPathConstant, nil },
			}

			command, buildError := builder.Build()
			require.NoError(subTest, buildError)

			command.SetContext(context.Background())
			command.SetArgs(testCase.arguments)
			executionError := command.Execute()
			require.NoError(subTest, executionError)

			require.Len(subTest, executor.executions, len(testCase.discoveredRepositories))
			for executionIndex, execution := range executor.executions {
				require.Equal(subTest, testCase.expectedPackage, execution.PackageName)
				require.Equal(subTest, testCase.expectedDryRun, execution.DryRun)
				require.Equal(subTest, testCase.expectedOwners[executionIndex], execution.Owner)
				require.Equal(subTest, testCase.expectedOwnerTypes[executionIndex], execution.OwnerType)
				require.Equal(subTest, packages.TokenSourceTypeEnvironment, execution.TokenSource.Type)
				require.Equal(subTest, "GITHUB_PACKAGES_TOKEN", execution.TokenSource.Reference)
			}

			require.NotEmpty(subTest, discoverer.recordedRoots)
			lastRoots := discoverer.recordedRoots[len(discoverer.recordedRoots)-1]
			require.Equal(subTest, testCase.expectedRoots, lastRoots)
		})
	}
}

func TestCommandBuilderAggregatesErrorsAcrossRepositories(testInstance *testing.T) {
	testInstance.Parallel()

	managerError := errors.New("remote lookup failed")
	executionError := errors.New("purge failure")

	configuration := packages.Configuration{Purge: packages.PurgeConfiguration{
		PackageName:     "config-package",
		RepositoryRoots: []string{configurationRootOnePathConstant},
	}}

	executor := &stubPurgeExecutor{errorByOwner: map[string]error{repositoryThreeOwnerConstant: executionError}}
	resolver := &stubServiceResolver{executor: executor}
	repositoryManager := &stubRepositoryManager{
		remoteURLByPath: map[string]string{
			discoveredRepositoryOnePathConstant:   repositoryOneRemoteURLConstant,
			discoveredRepositoryThreePathConstant: repositoryThreeRemoteURLConstant,
		},
		errorByPath: map[string]error{
			discoveredRepositoryOnePathConstant: managerError,
		},
	}
	githubResolver := &stubGitHubResolver{metadataByRepository: map[string]githubcli.RepositoryMetadata{
		repositoryThreeIdentifierConstant: {NameWithOwner: repositoryThreeOwnerConstant + "/ignored", IsInOrganization: true},
	}}
	discoverer := &stubRepositoryDiscoverer{repositories: []string{discoveredRepositoryOnePathConstant, discoveredRepositoryThreePathConstant}}

	builder := packages.CommandBuilder{
		LoggerProvider:           func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider:    func() packages.Configuration { return configuration },
		ServiceResolver:          resolver,
		RepositoryManager:        repositoryManager,
		GitHubResolver:           githubResolver,
		RepositoryDiscoverer:     discoverer,
		WorkingDirectoryResolver: func() (string, error) { return workingDirectoryPathConstant, nil },
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)

	command.SetContext(context.Background())
	executionErrorResult := command.Execute()
	require.Error(testInstance, executionErrorResult)
	require.ErrorContains(testInstance, executionErrorResult, "unable to resolve repository context")
	require.ErrorContains(testInstance, executionErrorResult, "repo-packages-purge failed")
	require.Len(testInstance, executor.executions, 1)
	require.Equal(testInstance, repositoryThreeOwnerConstant, executor.executions[0].Owner)
}

func TestCommandBuilderPropagatesContextCancellation(testInstance *testing.T) {
	testInstance.Parallel()

	configuration := packages.Configuration{Purge: packages.PurgeConfiguration{
		PackageName:     "config-package",
		RepositoryRoots: []string{configurationRootOnePathConstant},
	}}

	executor := &stubPurgeExecutor{defaultError: context.Canceled}
	resolver := &stubServiceResolver{executor: executor}
	repositoryManager := &stubRepositoryManager{remoteURLByPath: map[string]string{
		discoveredRepositoryOnePathConstant: repositoryOneRemoteURLConstant,
	}}
	githubResolver := &stubGitHubResolver{metadataByRepository: map[string]githubcli.RepositoryMetadata{
		repositoryOneIdentifierConstant: {NameWithOwner: repositoryOneOwnerConstant + "/ignored", IsInOrganization: true},
	}}
	discoverer := &stubRepositoryDiscoverer{repositories: []string{discoveredRepositoryOnePathConstant}}

	builder := packages.CommandBuilder{
		LoggerProvider:           func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider:    func() packages.Configuration { return configuration },
		ServiceResolver:          resolver,
		RepositoryManager:        repositoryManager,
		GitHubResolver:           githubResolver,
		RepositoryDiscoverer:     discoverer,
		WorkingDirectoryResolver: func() (string, error) { return workingDirectoryPathConstant, nil },
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)

	command.SetContext(context.Background())
	executionError := command.Execute()
	require.Error(testInstance, executionError)
	require.ErrorIs(testInstance, executionError, context.Canceled)
}

func TestCommandBuilderValidatesArguments(testInstance *testing.T) {
	testInstance.Parallel()

	builder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration {
			return packages.Configuration{Purge: packages.PurgeConfiguration{PackageName: "config-package", RepositoryRoots: []string{configurationRootOnePathConstant}}}
		},
		ServiceResolver:          &stubServiceResolver{executor: &stubPurgeExecutor{}},
		RepositoryManager:        &stubRepositoryManager{remoteURLByPath: map[string]string{discoveredRepositoryOnePathConstant: repositoryOneRemoteURLConstant}},
		GitHubResolver:           &stubGitHubResolver{metadataByRepository: map[string]githubcli.RepositoryMetadata{repositoryOneIdentifierConstant: {NameWithOwner: repositoryOneOwnerConstant + "/ignored", IsInOrganization: true}}},
		RepositoryDiscoverer:     &stubRepositoryDiscoverer{repositories: []string{discoveredRepositoryOnePathConstant}},
		WorkingDirectoryResolver: func() (string, error) { return workingDirectoryPathConstant, nil },
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{"unexpected"})
	executionError := command.Execute()
	require.Error(testInstance, executionError)
	require.ErrorContains(testInstance, executionError, "does not accept positional arguments")
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
	executions   []packages.PurgeOptions
	result       ghcr.PurgeResult
	defaultError error
	errorByOwner map[string]error
}

func (executor *stubPurgeExecutor) Execute(executionContext context.Context, options packages.PurgeOptions) (ghcr.PurgeResult, error) {
	executor.executions = append(executor.executions, options)
	if executor.errorByOwner != nil {
		if ownerError, exists := executor.errorByOwner[options.Owner]; exists {
			return ghcr.PurgeResult{}, ownerError
		}
	}
	if executor.defaultError != nil {
		return ghcr.PurgeResult{}, executor.defaultError
	}
	return executor.result, nil
}

type stubRepositoryManager struct {
	remoteURLByPath         map[string]string
	errorByPath             map[string]error
	recordedRepositoryPaths []string
}

func (manager *stubRepositoryManager) CheckCleanWorktree(ctx context.Context, repositoryPath string) (bool, error) {
	return true, nil
}

func (manager *stubRepositoryManager) GetCurrentBranch(ctx context.Context, repositoryPath string) (string, error) {
	return "main", nil
}

func (manager *stubRepositoryManager) GetRemoteURL(ctx context.Context, repositoryPath string, remoteName string) (string, error) {
	manager.recordedRepositoryPaths = append(manager.recordedRepositoryPaths, repositoryPath)
	if manager.errorByPath != nil {
		if lookupError, exists := manager.errorByPath[repositoryPath]; exists {
			return "", lookupError
		}
	}
	if manager.remoteURLByPath != nil {
		if remoteURL, exists := manager.remoteURLByPath[repositoryPath]; exists {
			return remoteURL, nil
		}
	}
	return "", errors.New("remote not found")
}

func (manager *stubRepositoryManager) SetRemoteURL(ctx context.Context, repositoryPath string, remoteName string, remoteURL string) error {
	return nil
}

type stubGitHubResolver struct {
	metadata             githubcli.RepositoryMetadata
	metadataByRepository map[string]githubcli.RepositoryMetadata
	err                  error
	recordedRepositories []string
}

func (resolver *stubGitHubResolver) ResolveRepoMetadata(ctx context.Context, repository string) (githubcli.RepositoryMetadata, error) {
	resolver.recordedRepositories = append(resolver.recordedRepositories, repository)
	if resolver.err != nil {
		return githubcli.RepositoryMetadata{}, resolver.err
	}
	if resolver.metadataByRepository != nil {
		if metadata, exists := resolver.metadataByRepository[repository]; exists {
			return metadata, nil
		}
	}
	return resolver.metadata, nil
}

type stubRepositoryDiscoverer struct {
	repositories  []string
	err           error
	recordedRoots [][]string
}

func (discoverer *stubRepositoryDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	copiedRoots := make([]string, len(roots))
	copy(copiedRoots, roots)
	discoverer.recordedRoots = append(discoverer.recordedRoots, copiedRoots)
	if discoverer.err != nil {
		return nil, discoverer.err
	}
	return discoverer.repositories, nil
}
