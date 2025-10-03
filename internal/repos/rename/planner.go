package rename

import (
	"path/filepath"
	"strings"
)

const (
	ownerRepositorySeparatorConstant = "/"
)

// DirectoryPlan describes the desired folder arrangement for a repository rename.
type DirectoryPlan struct {
	FolderName        string
	OwnerSegment      string
	RepositorySegment string
	IncludeOwner      bool
}

// DirectoryPlanner computes desired directory plans based on rename preferences.
type DirectoryPlanner struct{}

// NewDirectoryPlanner constructs a planner instance for deriving rename targets.
func NewDirectoryPlanner() DirectoryPlanner {
	return DirectoryPlanner{}
}

// Plan evaluates the desired directory layout for a repository.
func (planner DirectoryPlanner) Plan(includeOwner bool, finalOwnerRepository string, defaultFolderName string) DirectoryPlan {
	trimmedDefaultFolderName := strings.TrimSpace(defaultFolderName)
	plan := DirectoryPlan{
		FolderName:        trimmedDefaultFolderName,
		RepositorySegment: trimmedDefaultFolderName,
	}

	if !includeOwner {
		return plan
	}

	ownerSegment, repositorySegment, parseSucceeded := splitOwnerRepository(finalOwnerRepository)
	if !parseSucceeded {
		return plan
	}

	plan.IncludeOwner = true
	plan.OwnerSegment = ownerSegment
	plan.RepositorySegment = repositorySegment
	plan.FolderName = filepath.Join(ownerSegment, repositorySegment)

	return plan
}

// IsNoop determines whether the repository already resides at the desired location.
func (plan DirectoryPlan) IsNoop(repositoryPath string, currentFolderName string) bool {
	trimmedTarget := strings.TrimSpace(plan.FolderName)
	if len(trimmedTarget) == 0 {
		return true
	}

	if plan.IncludeOwner {
		cleanedRepositoryPath := filepath.Clean(repositoryPath)
		expectedSuffix := filepath.Clean(trimmedTarget)
		return strings.HasSuffix(cleanedRepositoryPath, expectedSuffix)
	}

	return trimmedTarget == strings.TrimSpace(currentFolderName)
}

func splitOwnerRepository(ownerRepository string) (string, string, bool) {
	trimmedOwnerRepository := strings.TrimSpace(ownerRepository)
	if len(trimmedOwnerRepository) == 0 {
		return "", "", false
	}

	segments := strings.Split(trimmedOwnerRepository, ownerRepositorySeparatorConstant)
	if len(segments) != 2 {
		return "", "", false
	}

	ownerSegment := strings.TrimSpace(segments[0])
	repositorySegment := strings.TrimSpace(segments[1])
	if len(ownerSegment) == 0 || len(repositorySegment) == 0 {
		return "", "", false
	}

	return ownerSegment, repositorySegment, true
}
