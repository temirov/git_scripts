package discovery

import (
	"io/fs"
	"path/filepath"
	"sort"
)

const gitMetadataDirectoryNameConstant = ".git"

// FilesystemRepositoryDiscoverer locates git repositories on disk.
type FilesystemRepositoryDiscoverer struct{}

// NewFilesystemRepositoryDiscoverer constructs a repository discoverer backed by filepath.WalkDir.
func NewFilesystemRepositoryDiscoverer() *FilesystemRepositoryDiscoverer {
	return &FilesystemRepositoryDiscoverer{}
}

// DiscoverRepositories walks the provided roots and returns directories containing a .git entry.
func (discoverer *FilesystemRepositoryDiscoverer) DiscoverRepositories(roots []string) ([]string, error) {
	seen := make(map[string]struct{})
	var repositories []string

	for _, root := range roots {
		walkError := filepath.WalkDir(root, func(path string, directoryEntry fs.DirEntry, walkError error) error {
			if walkError != nil {
				return nil
			}

			if directoryEntry.Name() != gitMetadataDirectoryNameConstant {
				return nil
			}

			repositoryPath := filepath.Dir(path)
			if _, alreadySeen := seen[repositoryPath]; alreadySeen {
				if directoryEntry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			seen[repositoryPath] = struct{}{}
			repositories = append(repositories, repositoryPath)

			if directoryEntry.IsDir() {
				return fs.SkipDir
			}
			return nil
		})
		if walkError != nil {
			return nil, walkError
		}
	}

	sort.Strings(repositories)
	return repositories, nil
}
