package discovery_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/repos/discovery"
)

const (
	developerDirectoryName             = "Dev"
	engineeringGroupDirectoryName      = "Group1"
	applicationRepositoryDirectoryName = "Repo1"
	serviceRepositoryDirectoryName     = "Repo2"
	toolsRepositoryDirectoryName       = "Repo3"
	gitMetadataDirectoryName           = ".git"
	singleRootSubtestTitle             = "discoversRepositoriesFromSingleRoot"
	combinedRootsSubtestTitle          = "discoversRepositoriesFromParentAndNestedRoots"
	repositoryDirectoryPermissions     = 0o755
)

type repositoryDefinition struct {
	directorySegments []string
}

func (definition repositoryDefinition) repositoryPath(rootDirectory string) string {
	segments := append([]string{rootDirectory}, definition.directorySegments...)
	return filepath.Join(segments...)
}

func (definition repositoryDefinition) gitMetadataPath(rootDirectory string) string {
	segments := append([]string{rootDirectory}, definition.directorySegments...)
	segments = append(segments, gitMetadataDirectoryName)
	return filepath.Join(segments...)
}

type filesystemDiscoveryTestScenario struct {
	title                      string
	rootDirectoriesConstructor func(string) []string
}

func (scenario filesystemDiscoveryTestScenario) execute(
	testFramework *testing.T,
	repositoryDefinitions []repositoryDefinition,
) {
	testFramework.Helper()

	temporaryRootDirectory := testFramework.TempDir()
	for _, repositoryDefinition := range repositoryDefinitions {
		gitMetadataDirectoryPath := repositoryDefinition.gitMetadataPath(temporaryRootDirectory)
		creationError := os.MkdirAll(gitMetadataDirectoryPath, repositoryDirectoryPermissions)
		require.NoError(testFramework, creationError)
	}

	repositoryDiscoverer := discovery.NewFilesystemRepositoryDiscoverer()
	discoveredRepositories, discoveryError := repositoryDiscoverer.DiscoverRepositories(
		scenario.rootDirectoriesConstructor(temporaryRootDirectory),
	)
	require.NoError(testFramework, discoveryError)

	expectedRepositories := make([]string, 0, len(repositoryDefinitions))
	for _, repositoryDefinition := range repositoryDefinitions {
		expectedRepositories = append(expectedRepositories, repositoryDefinition.repositoryPath(temporaryRootDirectory))
	}

	sort.Strings(expectedRepositories)
	sort.Strings(discoveredRepositories)
	require.Equal(testFramework, expectedRepositories, discoveredRepositories)
}

func TestFilesystemRepositoryDiscovererDiscoversNestedLayouts(testFramework *testing.T) {
	repositoryDefinitions := []repositoryDefinition{
		{directorySegments: []string{developerDirectoryName, engineeringGroupDirectoryName, applicationRepositoryDirectoryName}},
		{directorySegments: []string{developerDirectoryName, engineeringGroupDirectoryName, serviceRepositoryDirectoryName}},
		{directorySegments: []string{developerDirectoryName, toolsRepositoryDirectoryName}},
	}

	testScenarios := []filesystemDiscoveryTestScenario{
		{
			title: singleRootSubtestTitle,
			rootDirectoriesConstructor: func(rootDirectory string) []string {
				return []string{rootDirectory}
			},
		},
		{
			title: combinedRootsSubtestTitle,
			rootDirectoriesConstructor: func(rootDirectory string) []string {
				developerDirectoryPath := filepath.Join(rootDirectory, developerDirectoryName)
				engineeringGroupDirectoryPath := filepath.Join(developerDirectoryPath, engineeringGroupDirectoryName)
				return []string{rootDirectory, developerDirectoryPath, engineeringGroupDirectoryPath}
			},
		},
	}

	for _, testScenario := range testScenarios {
		testFramework.Run(testScenario.title, func(testFramework *testing.T) {
			testScenario.execute(testFramework, repositoryDefinitions)
		})
	}
}
