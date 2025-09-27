package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OSFileSystem implements FileSystem using the operating system primitives.
type OSFileSystem struct{}

// Stat retrieves file metadata.
func (OSFileSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// Rename renames a path.
func (OSFileSystem) Rename(oldPath string, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// Abs resolves an absolute path.
func (OSFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}
