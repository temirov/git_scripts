package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/temirov/git_scripts/internal/execshell"
	migrate "github.com/temirov/git_scripts/internal/migrate"
	"github.com/temirov/git_scripts/internal/migrate/testsupport"
)

const (
	integrationRepositoryOneNameConstant         = "repository-one"
	integrationRepositoryTwoNameConstant         = "repository-two"
	integrationRepositoryOneRemoteConstant       = "git@github.com:integration/repository-one.git"
	integrationRepositoryTwoRemoteConstant       = "git@github.com:integration/repository-two.git"
	integrationRepositoryOneIdentifierConstant   = "integration/repository-one"
	integrationRepositoryTwoIdentifierConstant   = "integration/repository-two"
	integrationBlockingReasonConstant            = "open pull requests still target source branch"
	integrationMigrationCompletedMessageConstant = "Branch migration completed"
	integrationSafetyWarningMessageConstant      = "Branch deletion blocked by safety gates"
	integrationFailureWarningMessageConstant     = "Repository migration failed"
	integrationRepositoryFieldNameConstant       = "repository"
	integrationMigrateCommandNameConstant        = "migrate"
)

func TestBranchMigrateCommandIntegration(testInstance *testing.T) {
	testCases := []struct {
		name                  string
		repositoryDefinitions []integrationRepositoryDefinition
		expectError           bool
	}{
		{
			name: "processes_multiple_repositories",
			repositoryDefinitions: []integrationRepositoryDefinition{
				{
					directoryName:      integrationRepositoryOneNameConstant,
					remoteURL:          integrationRepositoryOneRemoteConstant,
					expectedIdentifier: integrationRepositoryOneIdentifierConstant,
					outcome: testsupport.ServiceOutcome{
						Result: migrate.MigrationResult{SafetyStatus: migrate.SafetyStatus{SafeToDelete: true}},
					},
				},
				{
					directoryName:      integrationRepositoryTwoNameConstant,
					remoteURL:          integrationRepositoryTwoRemoteConstant,
					expectedIdentifier: integrationRepositoryTwoIdentifierConstant,
					outcome: testsupport.ServiceOutcome{
						Result: migrate.MigrationResult{
							SafetyStatus: migrate.SafetyStatus{
								SafeToDelete:    false,
								BlockingReasons: []string{integrationBlockingReasonConstant},
							},
						},
					},
					expectSafetyWarning: true,
				},
			},
			expectError: false,
		},
		{
			name: "continues_after_service_failure",
			repositoryDefinitions: []integrationRepositoryDefinition{
				{
					directoryName:        integrationRepositoryOneNameConstant,
					remoteURL:            integrationRepositoryOneRemoteConstant,
					expectedIdentifier:   integrationRepositoryOneIdentifierConstant,
					outcome:              testsupport.ServiceOutcome{Error: fmt.Errorf("worktree dirty")},
					expectFailureWarning: true,
				},
				{
					directoryName:      integrationRepositoryTwoNameConstant,
					remoteURL:          integrationRepositoryTwoRemoteConstant,
					expectedIdentifier: integrationRepositoryTwoIdentifierConstant,
					outcome:            testsupport.ServiceOutcome{Result: migrate.MigrationResult{SafetyStatus: migrate.SafetyStatus{SafeToDelete: true}}},
				},
			},
			expectError: true,
		},
	}

	for testCaseIndex := range testCases {
		testCase := testCases[testCaseIndex]
		subtestName := fmt.Sprintf("%d_%s", testCaseIndex, testCase.name)

		testInstance.Run(subtestName, func(subtest *testing.T) {
			workingDirectory := subtest.TempDir()

			repositoryRemotes := make(map[string]string, len(testCase.repositoryDefinitions))
			serviceOutcomes := make(map[string]testsupport.ServiceOutcome, len(testCase.repositoryDefinitions))
			expectedSafetyWarnings := make([]string, 0, len(testCase.repositoryDefinitions))
			expectedFailureWarnings := make([]string, 0, len(testCase.repositoryDefinitions))
			expectedIdentifiers := make(map[string]string, len(testCase.repositoryDefinitions))
			expectedRepositories := make([]string, 0, len(testCase.repositoryDefinitions))

			for _, definition := range testCase.repositoryDefinitions {
				repositoryPath := filepath.Join(workingDirectory, definition.directoryName)
				gitDirectory := filepath.Join(repositoryPath, ".git")
				require.NoError(subtest, os.MkdirAll(gitDirectory, 0o755))

				cleanedPath := filepath.Clean(repositoryPath)
				repositoryRemotes[cleanedPath] = definition.remoteURL
				serviceOutcomes[cleanedPath] = definition.outcome
				expectedIdentifiers[cleanedPath] = definition.expectedIdentifier
				expectedRepositories = append(expectedRepositories, cleanedPath)

				if definition.expectSafetyWarning {
					expectedSafetyWarnings = append(expectedSafetyWarnings, cleanedPath)
				}
				if definition.expectFailureWarning {
					expectedFailureWarnings = append(expectedFailureWarnings, cleanedPath)
				}
			}

			require.Len(subtest, expectedRepositories, len(testCase.repositoryDefinitions))

			commandExecutor := &testsupport.CommandExecutorStub{RepositoryRemotes: repositoryRemotes}
			migrationService := &testsupport.ServiceStub{Outcomes: serviceOutcomes}

			logCore, observedLogs := observer.New(zap.DebugLevel)
			logger := zap.New(logCore)

			builder := migrate.CommandBuilder{
				LoggerProvider: func() *zap.Logger { return logger },
				Executor:       commandExecutor,
				ServiceProvider: func(migrate.ServiceDependencies) (migrate.MigrationExecutor, error) {
					return migrationService, nil
				},
				WorkingDirectory: workingDirectory,
			}

			command, buildError := builder.Build()
			require.NoError(subtest, buildError)

			command.SetContext(context.Background())
			command.SetArgs([]string{integrationMigrateCommandNameConstant})

			executionError := command.Execute()
			if testCase.expectError {
				require.Error(subtest, executionError)
			} else {
				require.NoError(subtest, executionError)
			}

			executedRepositories := collectIntegrationRepositories(migrationService.ExecutedOptions)
			require.ElementsMatch(subtest, expectedRepositories, executedRepositories)

			for _, options := range migrationService.ExecutedOptions {
				expectedIdentifier, exists := expectedIdentifiers[options.RepositoryPath]
				require.True(subtest, exists)
				require.Equal(subtest, expectedIdentifier, options.RepositoryIdentifier)
			}

			executedGitPaths := collectIntegrationGitPaths(commandExecutor.ExecutedGitCommands)
			require.ElementsMatch(subtest, expectedRepositories, executedGitPaths)

			logEntries := observedLogs.All()
			successfulRepositories := filterSuccessfulRepositories(expectedRepositories, expectedFailureWarnings)
			verifyIntegrationInfoLogs(subtest, logEntries, successfulRepositories)
			verifyIntegrationWarnings(subtest, logEntries, integrationSafetyWarningMessageConstant, expectedSafetyWarnings)
			verifyIntegrationWarnings(subtest, logEntries, integrationFailureWarningMessageConstant, expectedFailureWarnings)
		})
	}
}

type integrationRepositoryDefinition struct {
	directoryName        string
	remoteURL            string
	expectedIdentifier   string
	outcome              testsupport.ServiceOutcome
	expectSafetyWarning  bool
	expectFailureWarning bool
}

func collectIntegrationRepositories(options []migrate.MigrationOptions) []string {
	repositories := make([]string, 0, len(options))
	for _, option := range options {
		repositories = append(repositories, option.RepositoryPath)
	}
	return repositories
}

func collectIntegrationGitPaths(commands []execshell.CommandDetails) []string {
	paths := make([]string, 0, len(commands))
	for _, commandDetails := range commands {
		paths = append(paths, commandDetails.WorkingDirectory)
	}
	return paths
}

func verifyIntegrationInfoLogs(testInstance *testing.T, entries []observer.LoggedEntry, expectedRepositories []string) {
	infoEntries := filterIntegrationLogs(entries, zapcore.InfoLevel, integrationMigrationCompletedMessageConstant)
	require.Len(testInstance, infoEntries, len(expectedRepositories))
	for _, entry := range infoEntries {
		repositoryValue, present := entry.ContextMap()[integrationRepositoryFieldNameConstant]
		require.True(testInstance, present)
		require.Contains(testInstance, expectedRepositories, repositoryValue)
	}
}

func verifyIntegrationWarnings(testInstance *testing.T, entries []observer.LoggedEntry, message string, expectedRepositories []string) {
	warningEntries := filterIntegrationLogs(entries, zapcore.WarnLevel, message)
	repositoryValues := make([]string, 0, len(warningEntries))
	for _, entry := range warningEntries {
		if repositoryValue, present := entry.ContextMap()[integrationRepositoryFieldNameConstant]; present {
			repositoryValues = append(repositoryValues, repositoryValue.(string))
		}
	}
	require.ElementsMatch(testInstance, expectedRepositories, repositoryValues)
}

func filterIntegrationLogs(entries []observer.LoggedEntry, level zapcore.Level, message string) []observer.LoggedEntry {
	matched := make([]observer.LoggedEntry, 0)
	for _, entry := range entries {
		if entry.Level == level && entry.Message == message {
			matched = append(matched, entry)
		}
	}
	return matched
}

func filterSuccessfulRepositories(allRepositories []string, failureRepositories []string) []string {
	if len(failureRepositories) == 0 {
		return append([]string{}, allRepositories...)
	}

	failureSet := make(map[string]struct{}, len(failureRepositories))
	for _, repositoryPath := range failureRepositories {
		failureSet[repositoryPath] = struct{}{}
	}

	successful := make([]string, 0, len(allRepositories))
	for _, repositoryPath := range allRepositories {
		if _, failed := failureSet[repositoryPath]; failed {
			continue
		}
		successful = append(successful, repositoryPath)
	}
	return successful
}
