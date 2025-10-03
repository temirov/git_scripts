package rename_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temirov/gix/internal/repos/rename"
)

const (
	plannerOwnerRepositoryConstant          = "owner/example"
	plannerAlternateOwnerRepositoryConstant = "alternate/sample"
	plannerDefaultFolderNameConstant        = "example"
	plannerRepositoryPathConstant           = "/tmp/example"
	plannerOwnerRepositoryPathConstant      = "/tmp/owner/example"
)

func TestDirectoryPlannerPlan(testInstance *testing.T) {
	testCases := []struct {
		name              string
		includeOwner      bool
		finalOwnerRepo    string
		defaultFolderName string
		expectedPlan      rename.DirectoryPlan
	}{
		{
			name:              "without_owner_uses_default",
			includeOwner:      false,
			finalOwnerRepo:    plannerOwnerRepositoryConstant,
			defaultFolderName: plannerDefaultFolderNameConstant,
			expectedPlan: rename.DirectoryPlan{
				FolderName:        plannerDefaultFolderNameConstant,
				RepositorySegment: plannerDefaultFolderNameConstant,
				IncludeOwner:      false,
			},
		},
		{
			name:              "with_owner_builds_nested_folder",
			includeOwner:      true,
			finalOwnerRepo:    plannerOwnerRepositoryConstant,
			defaultFolderName: plannerDefaultFolderNameConstant,
			expectedPlan: rename.DirectoryPlan{
				FolderName:        filepath.Join("owner", "example"),
				OwnerSegment:      "owner",
				RepositorySegment: "example",
				IncludeOwner:      true,
			},
		},
		{
			name:              "with_owner_missing_identifier_uses_default",
			includeOwner:      true,
			finalOwnerRepo:    "",
			defaultFolderName: plannerDefaultFolderNameConstant,
			expectedPlan: rename.DirectoryPlan{
				FolderName:        plannerDefaultFolderNameConstant,
				RepositorySegment: plannerDefaultFolderNameConstant,
				IncludeOwner:      false,
			},
		},
	}

	planner := rename.NewDirectoryPlanner()
	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			plan := planner.Plan(testCase.includeOwner, testCase.finalOwnerRepo, testCase.defaultFolderName)
			require.Equal(subtest, testCase.expectedPlan, plan)
		})
	}
}

func TestDirectoryPlanIsNoop(testInstance *testing.T) {
	planner := rename.NewDirectoryPlanner()
	testCases := []struct {
		name           string
		plan           rename.DirectoryPlan
		repositoryPath string
		currentFolder  string
		expectedIsNoop bool
	}{
		{
			name:           "empty_target_skips",
			plan:           rename.DirectoryPlan{FolderName: ""},
			repositoryPath: plannerRepositoryPathConstant,
			currentFolder:  plannerDefaultFolderNameConstant,
			expectedIsNoop: true,
		},
		{
			name:           "matching_folder_without_owner",
			plan:           planner.Plan(false, plannerOwnerRepositoryConstant, plannerDefaultFolderNameConstant),
			repositoryPath: plannerRepositoryPathConstant,
			currentFolder:  plannerDefaultFolderNameConstant,
			expectedIsNoop: true,
		},
		{
			name:           "mismatched_folder_without_owner",
			plan:           planner.Plan(false, plannerOwnerRepositoryConstant, plannerDefaultFolderNameConstant),
			repositoryPath: plannerRepositoryPathConstant,
			currentFolder:  "legacy",
			expectedIsNoop: false,
		},
		{
			name:           "matching_owner_repository_suffix",
			plan:           planner.Plan(true, plannerOwnerRepositoryConstant, plannerDefaultFolderNameConstant),
			repositoryPath: plannerOwnerRepositoryPathConstant,
			currentFolder:  plannerDefaultFolderNameConstant,
			expectedIsNoop: true,
		},
		{
			name:           "different_owner_repository_suffix",
			plan:           planner.Plan(true, plannerAlternateOwnerRepositoryConstant, plannerDefaultFolderNameConstant),
			repositoryPath: plannerOwnerRepositoryPathConstant,
			currentFolder:  plannerDefaultFolderNameConstant,
			expectedIsNoop: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testInstance.Run(testCase.name, func(subtest *testing.T) {
			isNoop := testCase.plan.IsNoop(testCase.repositoryPath, testCase.currentFolder)
			require.Equal(subtest, testCase.expectedIsNoop, isNoop)
		})
	}
}
