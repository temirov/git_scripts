package migrate_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/temirov/gix/internal/execshell"
	migrate "github.com/temirov/gix/internal/migrate"
	"github.com/temirov/gix/internal/migrate/testsupport"
	"github.com/temirov/gix/internal/utils"
	flagutils "github.com/temirov/gix/internal/utils/flags"
)

const (
	rootFlagArgumentConstant                 = "--roots"
	multiRootFirstArgumentConstant           = "root-one"
	multiRootSecondArgumentConstant          = "root-two"
	repositoryOnePathConstant                = "/tmp/repository-one"
	repositoryTwoPathConstant                = "/tmp/repository-two"
	repositoryOneRemoteConstant              = "git@github.com:example/repository-one.git"
	repositoryTwoRemoteConstant              = "git@github.com:example/repository-two.git"
	repositoryOneIdentifierConstant          = "example/repository-one"
	repositoryTwoIdentifierConstant          = "example/repository-two"
	blockingReasonOpenPullRequestsConstant   = "open pull requests still target source branch"
	migrationCompletedMessageTextConstant    = "Default branch update completed"
	migrationFailureMessageTextConstant      = "Default branch update failed"
	safetyWarningMessageTextConstant         = "Branch deletion blocked by safety gates"
	discoveryFailureMessageTextConstant      = "Repository discovery failed"
	defaultAlreadyMatchesMessageTextConstant = "Default branch already matches target"
	repositoryLogFieldNameConstant           = "repository"
	rootsLogFieldNameConstant                = "roots"
	workingDirectoryFallbackConstant         = "/workspace/root"
	defaultRootRepositoryPathConstant        = "/workspace/root/project"
	defaultRootRemoteConstant                = "git@github.com:example/default.git"
	defaultRootIdentifierConstant            = "example/default"
	configurationRootValueConstant           = "/tmp/configured-root"
	cliRootOverrideConstant                  = "/tmp/cli-root"
	toFlagArgumentConstant                   = "--to"
	customTargetBranchNameConstant           = "stable"
	missingRootsErrorMessageConstant         = "no repository roots provided; specify --roots or configure defaults"
)

func TestMigrateCommandRunScenarios(testInstance *testing.T) {
	testCases := []struct {
		name                         string
		arguments                    []string
		workingDirectory             string
		discoveredRepositories       []string
		discoveryError               error
		repositoryRemotes            map[string]string
		repositoryDefaultBranches    map[string]string
		repositoryErrors             map[string]error
		serviceOutcomes              map[string]testsupport.ServiceOutcome
		expectedRoots                []string
		expectedExecutedRepositories []string
		expectedIdentifiers          map[string]string
		expectedSafetyWarnings       []string
		expectedFailureWarnings      []string
		expectDiscoveryFailureLog    bool
		expectError                  bool
		expectedDebugEnabled         bool
		logLevel                     string
		expectedSourceBranch         migrate.BranchName
		expectedTargetBranch         migrate.BranchName
	}{
		{
			name:                   "processes_multiple_repositories",
			arguments:              []string{rootFlagArgumentConstant, multiRootFirstArgumentConstant, rootFlagArgumentConstant, multiRootSecondArgumentConstant},
			discoveredRepositories: []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			repositoryRemotes: map[string]string{
				repositoryOnePathConstant: repositoryOneRemoteConstant,
				repositoryTwoPathConstant: repositoryTwoRemoteConstant,
			},
			serviceOutcomes: map[string]testsupport.ServiceOutcome{
				repositoryOnePathConstant: {
					Result: migrate.MigrationResult{
						SafetyStatus: migrate.SafetyStatus{SafeToDelete: true},
					},
				},
				repositoryTwoPathConstant: {
					Result: migrate.MigrationResult{
						SafetyStatus: migrate.SafetyStatus{
							SafeToDelete:    false,
							BlockingReasons: []string{blockingReasonOpenPullRequestsConstant},
						},
					},
				},
			},
			expectedRoots:                []string{multiRootFirstArgumentConstant, multiRootSecondArgumentConstant},
			expectedExecutedRepositories: []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			expectedIdentifiers: map[string]string{
				repositoryOnePathConstant: repositoryOneIdentifierConstant,
				repositoryTwoPathConstant: repositoryTwoIdentifierConstant,
			},
			expectedSafetyWarnings:    []string{repositoryTwoPathConstant},
			expectedFailureWarnings:   nil,
			expectDiscoveryFailureLog: false,
			expectError:               false,
			expectedDebugEnabled:      false,
			expectedSourceBranch:      migrate.BranchMain,
			expectedTargetBranch:      migrate.BranchMaster,
		},
		{
			name:                   "continues_on_migration_failure",
			arguments:              []string{rootFlagArgumentConstant, multiRootFirstArgumentConstant},
			discoveredRepositories: []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			repositoryRemotes: map[string]string{
				repositoryOnePathConstant: repositoryOneRemoteConstant,
				repositoryTwoPathConstant: repositoryTwoRemoteConstant,
			},
			serviceOutcomes: map[string]testsupport.ServiceOutcome{
				repositoryOnePathConstant: {
					Error: errors.New("clean worktree required"),
				},
				repositoryTwoPathConstant: {
					Result: migrate.MigrationResult{SafetyStatus: migrate.SafetyStatus{SafeToDelete: true}},
				},
			},
			expectedRoots:                []string{multiRootFirstArgumentConstant},
			expectedExecutedRepositories: []string{repositoryOnePathConstant, repositoryTwoPathConstant},
			expectedIdentifiers: map[string]string{
				repositoryOnePathConstant: repositoryOneIdentifierConstant,
				repositoryTwoPathConstant: repositoryTwoIdentifierConstant,
			},
			expectedSafetyWarnings:    nil,
			expectedFailureWarnings:   []string{repositoryOnePathConstant},
			expectDiscoveryFailureLog: false,
			expectError:               true,
			expectedDebugEnabled:      false,
			expectedSourceBranch:      migrate.BranchMain,
			expectedTargetBranch:      migrate.BranchMaster,
		},
		{
			name:                         "reports_discovery_error",
			arguments:                    []string{rootFlagArgumentConstant, multiRootFirstArgumentConstant},
			discoveredRepositories:       nil,
			discoveryError:               errors.New("walk failure"),
			repositoryRemotes:            map[string]string{},
			serviceOutcomes:              map[string]testsupport.ServiceOutcome{},
			expectedRoots:                []string{multiRootFirstArgumentConstant},
			expectedExecutedRepositories: nil,
			expectedIdentifiers:          map[string]string{},
			expectedSafetyWarnings:       nil,
			expectedFailureWarnings:      nil,
			expectDiscoveryFailureLog:    true,
			expectError:                  true,
			expectedDebugEnabled:         false,
			expectedSourceBranch:         migrate.BranchMain,
			expectedTargetBranch:         migrate.BranchMaster,
		},
		{
			name: "uses_working_directory_when_arguments_missing",
			arguments: []string{
				rootFlagArgumentConstant, workingDirectoryFallbackConstant,
			},
			workingDirectory:       workingDirectoryFallbackConstant,
			discoveredRepositories: []string{defaultRootRepositoryPathConstant},
			repositoryRemotes: map[string]string{
				defaultRootRepositoryPathConstant: defaultRootRemoteConstant,
			},
			serviceOutcomes: map[string]testsupport.ServiceOutcome{
				defaultRootRepositoryPathConstant: {
					Result: migrate.MigrationResult{SafetyStatus: migrate.SafetyStatus{SafeToDelete: true}},
				},
			},
			expectedRoots:                []string{workingDirectoryFallbackConstant},
			expectedExecutedRepositories: []string{defaultRootRepositoryPathConstant},
			expectedIdentifiers: map[string]string{
				defaultRootRepositoryPathConstant: defaultRootIdentifierConstant,
			},
			expectedSafetyWarnings:    nil,
			expectedFailureWarnings:   nil,
			expectDiscoveryFailureLog: false,
			expectError:               false,
			expectedDebugEnabled:      true,
			logLevel:                  string(utils.LogLevelDebug),
			expectedSourceBranch:      migrate.BranchMain,
			expectedTargetBranch:      migrate.BranchMaster,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		subtestName := fmt.Sprintf("%d_%s", testCaseIndex, testCase.name)

		testInstance.Run(subtestName, func(subtest *testing.T) {
			repositoryDiscoverer := &testsupport.RepositoryDiscovererStub{
				Repositories:   append([]string{}, testCase.discoveredRepositories...),
				DiscoveryError: testCase.discoveryError,
			}

			commandExecutor := &testsupport.CommandExecutorStub{
				RepositoryRemotes:         appendMap(testCase.repositoryRemotes),
				RepositoryErrors:          appendErrorMap(testCase.repositoryErrors),
				RepositoryDefaultBranches: appendMap(testCase.repositoryDefaultBranches),
			}

			migrationService := &testsupport.ServiceStub{Outcomes: appendOutcomeMap(testCase.serviceOutcomes)}

			logCore, observedLogs := observer.New(zap.DebugLevel)
			logger := zap.New(logCore)

			builder := migrate.CommandBuilder{
				LoggerProvider: func() *zap.Logger { return logger },
				Executor:       commandExecutor,
				ServiceProvider: func(migrate.ServiceDependencies) (migrate.MigrationExecutor, error) {
					return migrationService, nil
				},
				RepositoryDiscoverer: repositoryDiscoverer,
				WorkingDirectory:     testCase.workingDirectory,
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)
			registerRootFlag(command)

			commandContext := context.Background()
			if len(strings.TrimSpace(testCase.logLevel)) > 0 {
				contextAccessor := utils.NewCommandContextAccessor()
				commandContext = contextAccessor.WithLogLevel(commandContext, testCase.logLevel)
			}
			command.SetContext(commandContext)
			commandArguments := append([]string{}, testCase.arguments...)
			command.SetArgs(commandArguments)

			executionError := command.Execute()
			if testCase.expectError {
				require.Error(subtest, executionError)
			} else {
				require.NoError(subtest, executionError)
			}

			require.Equal(subtest, testCase.expectedRoots, repositoryDiscoverer.ReceivedRoots)

			executedRepositories := collectExecutedRepositories(migrationService.ExecutedOptions)
			require.ElementsMatch(subtest, testCase.expectedExecutedRepositories, executedRepositories)

			executedGitRepositories := collectGitWorkingDirectories(commandExecutor.ExecutedGitCommands)
			require.ElementsMatch(subtest, testCase.expectedExecutedRepositories, executedGitRepositories)

			if len(migrationService.ExecutedOptions) > 0 {
				for _, options := range migrationService.ExecutedOptions {
					if testCase.expectedIdentifiers != nil {
						expectedIdentifier, exists := testCase.expectedIdentifiers[options.RepositoryPath]
						require.True(subtest, exists)
						require.Equal(subtest, expectedIdentifier, options.RepositoryIdentifier)
					}
					require.Equal(subtest, testCase.expectedDebugEnabled, options.EnableDebugLogging)
					expectedSource := testCase.expectedSourceBranch
					if len(strings.TrimSpace(string(expectedSource))) == 0 {
						expectedSource = migrate.BranchMain
					}
					require.Equal(subtest, expectedSource, options.SourceBranch)
					expectedTarget := testCase.expectedTargetBranch
					if len(strings.TrimSpace(string(expectedTarget))) == 0 {
						expectedTarget = migrate.BranchMaster
					}
					require.Equal(subtest, expectedTarget, options.TargetBranch)
				}
			}

			logEntries := observedLogs.All()
			verifySafetyWarnings(subtest, logEntries, testCase.expectedSafetyWarnings)
			verifyMigrationFailures(subtest, logEntries, testCase.expectedFailureWarnings)
			if testCase.expectDiscoveryFailureLog {
				verifyDiscoveryFailureLogged(subtest, logEntries, testCase.expectedRoots)
			} else {
				ensureNoDiscoveryFailure(subtest, logEntries)
			}

			if !testCase.expectError {
				infoEntries := findLogEntries(logEntries, zapcore.InfoLevel, migrationCompletedMessageTextConstant)
				require.Len(subtest, infoEntries, len(testCase.expectedExecutedRepositories))
				for _, entry := range infoEntries {
					repositoryValue, hasRepository := entry.ContextMap()[repositoryLogFieldNameConstant]
					require.True(subtest, hasRepository)
					require.Contains(subtest, testCase.expectedExecutedRepositories, repositoryValue)
				}
			}
		})
	}
}

func TestMigrateCommandDisplaysHelpWhenRootsMissing(testInstance *testing.T) {
	builder := migrate.CommandBuilder{}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)
	registerRootFlag(command)

	outputBuffer := &strings.Builder{}
	command.SetOut(outputBuffer)
	command.SetErr(outputBuffer)
	command.SetArgs([]string{})

	executionError := command.Execute()
	require.Error(testInstance, executionError)
	require.Equal(testInstance, missingRootsErrorMessageConstant, executionError.Error())
	require.Contains(testInstance, outputBuffer.String(), command.UseLine())
}

func TestMigrateCommandConfigurationPrecedence(testInstance *testing.T) {
	testCases := []struct {
		name                      string
		configuration             migrate.CommandConfiguration
		arguments                 []string
		workingDirectory          string
		discoveredRepositories    []string
		repositoryRemotes         map[string]string
		repositoryDefaultBranches map[string]string
		expectedRoots             []string
		expectedDebugEnabled      bool
		logLevel                  string
		expectedSourceBranch      migrate.BranchName
		expectedTargetBranch      migrate.BranchName
	}{
		{
			name: "configuration_values_apply",
			configuration: migrate.CommandConfiguration{
				EnableDebugLogging: true,
				RepositoryRoots:    []string{"  " + configurationRootValueConstant + "  "},
				TargetBranch:       "  release  ",
			},
			arguments:              []string{rootFlagArgumentConstant, configurationRootValueConstant},
			workingDirectory:       workingDirectoryFallbackConstant,
			discoveredRepositories: []string{repositoryOnePathConstant},
			repositoryRemotes: map[string]string{
				repositoryOnePathConstant: repositoryOneRemoteConstant,
			},
			repositoryDefaultBranches: map[string]string{
				repositoryOneIdentifierConstant: "develop",
			},
			expectedRoots:        []string{configurationRootValueConstant},
			expectedDebugEnabled: true,
			expectedSourceBranch: migrate.BranchName("develop"),
			expectedTargetBranch: migrate.BranchName("release"),
		},
		{
			name: "flags_override_configuration",
			configuration: migrate.CommandConfiguration{
				EnableDebugLogging: false,
				RepositoryRoots:    []string{configurationRootValueConstant},
			},
			arguments: []string{
				rootFlagArgumentConstant, cliRootOverrideConstant,
				toFlagArgumentConstant + "=" + customTargetBranchNameConstant,
			},
			workingDirectory:       workingDirectoryFallbackConstant,
			discoveredRepositories: []string{repositoryTwoPathConstant},
			repositoryRemotes: map[string]string{
				repositoryTwoPathConstant: repositoryTwoRemoteConstant,
			},
			expectedRoots:        []string{cliRootOverrideConstant},
			expectedDebugEnabled: true,
			logLevel:             string(utils.LogLevelDebug),
			expectedSourceBranch: migrate.BranchMain,
			expectedTargetBranch: migrate.BranchName(customTargetBranchNameConstant),
		},
		{
			name:                   "defaults_fill_missing_configuration",
			configuration:          migrate.CommandConfiguration{},
			arguments:              []string{rootFlagArgumentConstant, workingDirectoryFallbackConstant},
			workingDirectory:       workingDirectoryFallbackConstant,
			discoveredRepositories: []string{defaultRootRepositoryPathConstant},
			repositoryRemotes: map[string]string{
				defaultRootRepositoryPathConstant: defaultRootRemoteConstant,
			},
			expectedRoots:        []string{workingDirectoryFallbackConstant},
			expectedDebugEnabled: false,
			expectedSourceBranch: migrate.BranchMain,
			expectedTargetBranch: migrate.BranchMaster,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		subtestName := fmt.Sprintf("%d_%s", testCaseIndex, testCase.name)

		testInstance.Run(subtestName, func(subtest *testing.T) {
			repositoryDiscoverer := &testsupport.RepositoryDiscovererStub{
				Repositories: append([]string{}, testCase.discoveredRepositories...),
			}

			commandExecutor := &testsupport.CommandExecutorStub{
				RepositoryRemotes:         appendMap(testCase.repositoryRemotes),
				RepositoryDefaultBranches: appendMap(testCase.repositoryDefaultBranches),
			}

			migrationService := &testsupport.ServiceStub{}

			builder := migrate.CommandBuilder{
				LoggerProvider: func() *zap.Logger { return zap.NewNop() },
				Executor:       commandExecutor,
				ServiceProvider: func(migrate.ServiceDependencies) (migrate.MigrationExecutor, error) {
					return migrationService, nil
				},
				RepositoryDiscoverer: repositoryDiscoverer,
				WorkingDirectory:     testCase.workingDirectory,
				ConfigurationProvider: func() migrate.CommandConfiguration {
					return testCase.configuration
				},
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)
			registerRootFlag(command)

			executionContext := context.Background()
			if len(strings.TrimSpace(testCase.logLevel)) > 0 {
				contextAccessor := utils.NewCommandContextAccessor()
				executionContext = contextAccessor.WithLogLevel(executionContext, testCase.logLevel)
			}
			command.SetContext(executionContext)
			command.SetArgs(append([]string{}, testCase.arguments...))

			executionError := command.Execute()
			require.NoError(subtest, executionError)

			require.Equal(subtest, testCase.expectedRoots, repositoryDiscoverer.ReceivedRoots)

			if require.Len(subtest, migrationService.ExecutedOptions, len(testCase.discoveredRepositories)); len(migrationService.ExecutedOptions) > 0 {
				for _, options := range migrationService.ExecutedOptions {
					require.Equal(subtest, testCase.expectedDebugEnabled, options.EnableDebugLogging)
					expectedSource := testCase.expectedSourceBranch
					if len(strings.TrimSpace(string(expectedSource))) == 0 {
						expectedSource = migrate.BranchMain
					}
					require.Equal(subtest, expectedSource, options.SourceBranch)
					expectedTarget := testCase.expectedTargetBranch
					if len(strings.TrimSpace(string(expectedTarget))) == 0 {
						expectedTarget = migrate.BranchMaster
					}
					require.Equal(subtest, expectedTarget, options.TargetBranch)
				}
			}
		})
	}
}

func TestDefaultCommandSkipsWhenBranchMatchesTarget(testInstance *testing.T) {
	repositoryDiscoverer := &testsupport.RepositoryDiscovererStub{
		Repositories: []string{repositoryOnePathConstant},
	}

	logCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(logCore)

	commandExecutor := &testsupport.CommandExecutorStub{
		RepositoryRemotes: map[string]string{
			repositoryOnePathConstant: repositoryOneRemoteConstant,
		},
		RepositoryDefaultBranches: map[string]string{
			repositoryOneIdentifierConstant: "master",
		},
	}

	migrationService := &testsupport.ServiceStub{}

	builder := migrate.CommandBuilder{
		LoggerProvider: func() *zap.Logger { return logger },
		Executor:       commandExecutor,
		ServiceProvider: func(migrate.ServiceDependencies) (migrate.MigrationExecutor, error) {
			return migrationService, nil
		},
		RepositoryDiscoverer: repositoryDiscoverer,
		WorkingDirectory:     workingDirectoryFallbackConstant,
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)
	registerRootFlag(command)

	command.SetContext(context.Background())
	command.SetArgs([]string{rootFlagArgumentConstant, repositoryOnePathConstant, toFlagArgumentConstant + "=master"})

	executionError := command.Execute()
	require.NoError(testInstance, executionError)
	require.Empty(testInstance, migrationService.ExecutedOptions)

	infoEntries := findLogEntries(observedLogs.All(), zapcore.InfoLevel, defaultAlreadyMatchesMessageTextConstant)
	require.Len(testInstance, infoEntries, 1)
	repositoryValue, hasRepository := infoEntries[0].ContextMap()["repository"]
	require.True(testInstance, hasRepository)
	require.Equal(testInstance, filepath.Clean(repositoryOnePathConstant), repositoryValue)
	targetValue, hasTarget := infoEntries[0].ContextMap()["target_branch"]
	require.True(testInstance, hasTarget)
	require.Equal(testInstance, "master", targetValue)
}

func registerRootFlag(command *cobra.Command) {
	flagutils.BindRootFlags(command, flagutils.RootFlagValues{}, flagutils.RootFlagDefinition{Name: flagutils.DefaultRootFlagName, Enabled: true})
}

func collectExecutedRepositories(options []migrate.MigrationOptions) []string {
	repositories := make([]string, 0, len(options))
	for _, option := range options {
		repositories = append(repositories, option.RepositoryPath)
	}
	return repositories
}

func collectGitWorkingDirectories(details []execshell.CommandDetails) []string {
	repositories := make([]string, 0, len(details))
	for _, detail := range details {
		repositories = append(repositories, detail.WorkingDirectory)
	}
	return repositories
}

func findLogEntries(entries []observer.LoggedEntry, level zapcore.Level, message string) []observer.LoggedEntry {
	matched := make([]observer.LoggedEntry, 0)
	for _, entry := range entries {
		if entry.Level == level && entry.Message == message {
			matched = append(matched, entry)
		}
	}
	return matched
}

func verifySafetyWarnings(testInstance *testing.T, entries []observer.LoggedEntry, expectedRepositories []string) {
	warningEntries := findLogEntries(entries, zapcore.WarnLevel, safetyWarningMessageTextConstant)
	repositories := extractRepositoryValues(warningEntries)
	require.ElementsMatch(testInstance, expectedRepositories, repositories)
}

func verifyMigrationFailures(testInstance *testing.T, entries []observer.LoggedEntry, expectedRepositories []string) {
	failureEntries := findLogEntries(entries, zapcore.WarnLevel, migrationFailureMessageTextConstant)
	repositories := extractRepositoryValues(failureEntries)
	require.ElementsMatch(testInstance, expectedRepositories, repositories)
}

func verifyDiscoveryFailureLogged(testInstance *testing.T, entries []observer.LoggedEntry, expectedRoots []string) {
	discoveryEntries := findLogEntries(entries, zapcore.ErrorLevel, discoveryFailureMessageTextConstant)
	require.Len(testInstance, discoveryEntries, 1)
	recordedRoots, present := discoveryEntries[0].ContextMap()[rootsLogFieldNameConstant]
	require.True(testInstance, present)
	actualRoots := normalizeStringSlice(recordedRoots)
	require.ElementsMatch(testInstance, expectedRoots, actualRoots)
}

func ensureNoDiscoveryFailure(testInstance *testing.T, entries []observer.LoggedEntry) {
	discoveryEntries := findLogEntries(entries, zapcore.ErrorLevel, discoveryFailureMessageTextConstant)
	require.Len(testInstance, discoveryEntries, 0)
}

func extractRepositoryValues(entries []observer.LoggedEntry) []string {
	repositories := make([]string, 0, len(entries))
	for _, entry := range entries {
		contextValues := entry.ContextMap()
		if repositoryValue, exists := contextValues[repositoryLogFieldNameConstant]; exists {
			repositories = append(repositories, repositoryValue.(string))
		}
	}
	return repositories
}

func normalizeStringSlice(value any) []string {
	switch typedValue := value.(type) {
	case []string:
		return typedValue
	case []interface{}:
		converted := make([]string, 0, len(typedValue))
		for index := range typedValue {
			element, isString := typedValue[index].(string)
			if isString {
				converted = append(converted, element)
			}
		}
		return converted
	default:
		return nil
	}
}

func appendMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func appendErrorMap(input map[string]error) map[string]error {
	if input == nil {
		return nil
	}
	cloned := make(map[string]error, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func appendOutcomeMap(input map[string]testsupport.ServiceOutcome) map[string]testsupport.ServiceOutcome {
	if input == nil {
		return nil
	}
	cloned := make(map[string]testsupport.ServiceOutcome, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func TestMigrateCommandExpandsTildeArguments(testInstance *testing.T) {
	testInstance.Helper()

	homeDirectory, homeDirectoryError := os.UserHomeDir()
	require.NoError(testInstance, homeDirectoryError)

	tildeArgument := "~/migrate/roots"
	expectedRoot := filepath.Join(homeDirectory, "migrate", "roots")

	repositoryDiscoverer := &testsupport.RepositoryDiscovererStub{}
	commandExecutor := &testsupport.CommandExecutorStub{}
	migrationService := &testsupport.ServiceStub{}

	builder := migrate.CommandBuilder{
		LoggerProvider:       func() *zap.Logger { return zap.NewNop() },
		Executor:             commandExecutor,
		RepositoryDiscoverer: repositoryDiscoverer,
		ServiceProvider: func(dependencies migrate.ServiceDependencies) (migrate.MigrationExecutor, error) {
			return migrationService, nil
		},
		ConfigurationProvider: func() migrate.CommandConfiguration {
			return migrate.CommandConfiguration{}
		},
	}

	command, buildError := builder.Build()
	require.NoError(testInstance, buildError)
	registerRootFlag(command)

	command.SetContext(context.Background())
	command.SetArgs([]string{rootFlagArgumentConstant, tildeArgument})

	executionError := command.Execute()
	require.NoError(testInstance, executionError)
	require.Equal(testInstance, []string{expectedRoot}, repositoryDiscoverer.ReceivedRoots)
}
