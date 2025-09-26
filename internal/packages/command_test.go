package packages_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/temirov/git_scripts/internal/ghcr"
	packages "github.com/temirov/git_scripts/internal/packages"
)

func TestCommandBuilderParsesConfigurationDefaults(testingInstance *testing.T) {
	testingInstance.Parallel()

	configuration := packages.Configuration{
		Purge: packages.PurgeConfiguration{
			Owner:       "config-owner",
			PackageName: "config-package",
			OwnerType:   "org",
			TokenSource: "env:CONFIG_TOKEN",
			DryRun:      true,
		},
	}

	executor := &stubPurgeExecutor{result: ghcr.PurgeResult{TotalVersions: 1}}
	resolver := &stubServiceResolver{executor: executor}

	builder := packages.CommandBuilder{
		LoggerProvider:        func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration { return configuration },
		ServiceResolver:       resolver,
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{})
	executionError := command.Execute()
	require.NoError(testingInstance, executionError)

	require.True(testingInstance, executor.called)
	require.Equal(testingInstance, "config-owner", executor.options.Owner)
	require.Equal(testingInstance, "config-package", executor.options.PackageName)
	require.Equal(testingInstance, ghcr.OrganizationOwnerType, executor.options.OwnerType)
	require.True(testingInstance, executor.options.DryRun)
	require.Equal(testingInstance, "CONFIG_TOKEN", executor.options.TokenSource.Reference)
}

func TestCommandBuilderFlagOverrides(testingInstance *testing.T) {
	testingInstance.Parallel()

	configuration := packages.Configuration{Purge: packages.PurgeConfiguration{OwnerType: "user"}}
	executor := &stubPurgeExecutor{result: ghcr.PurgeResult{TotalVersions: 2}}
	resolver := &stubServiceResolver{executor: executor}

	builder := packages.CommandBuilder{
		LoggerProvider:        func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration { return configuration },
		ServiceResolver:       resolver,
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	args := []string{
		"--owner", "flag-owner",
		"--package", "flag-package",
		"--owner-type", "org",
		"--token-source", "file:/tmp/token",
	}
	command.SetArgs(args)
	executionError := command.Execute()
	require.NoError(testingInstance, executionError)

	require.True(testingInstance, executor.called)
	require.Equal(testingInstance, "flag-owner", executor.options.Owner)
	require.Equal(testingInstance, "flag-package", executor.options.PackageName)
	require.Equal(testingInstance, ghcr.OrganizationOwnerType, executor.options.OwnerType)
	require.Equal(testingInstance, packages.TokenSourceTypeFile, executor.options.TokenSource.Type)
	require.Equal(testingInstance, "/tmp/token", executor.options.TokenSource.Reference)
	require.Equal(testingInstance, configuration.Purge.DryRun, executor.options.DryRun)
}

func TestCommandBuilderHandlesExecutionError(testingInstance *testing.T) {
	testingInstance.Parallel()

	executor := &stubPurgeExecutor{err: errors.New("failure")}
	resolver := &stubServiceResolver{executor: executor}

	builder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration {
			return packages.Configuration{Purge: packages.PurgeConfiguration{OwnerType: "user"}}
		},
		ServiceResolver: resolver,
	}

	command, buildError := builder.Build()
	require.NoError(testingInstance, buildError)

	command.SetContext(context.Background())
	command.SetArgs([]string{"--owner", "o", "--package", "p", "--token-source", "env:VAR"})
	executionError := command.Execute()
	require.Error(testingInstance, executionError)
	require.ErrorContains(testingInstance, executionError, "packages-purge failed")
}

func TestCommandBuilderValidatesArguments(testingInstance *testing.T) {
	testingInstance.Parallel()

	builder := packages.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return zap.NewNop() },
		ConfigurationProvider: func() packages.Configuration {
			return packages.Configuration{Purge: packages.PurgeConfiguration{OwnerType: "user"}}
		},
		ServiceResolver: &stubServiceResolver{executor: &stubPurgeExecutor{}},
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
