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

// MkdirAll ensures a directory hierarchy exists with the provided permissions.
func (OSFileSystem) MkdirAll(path string, permissions fs.FileMode) error {
	return os.MkdirAll(path, permissions)
}

// ReadFile reads file contents.
func (OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to a file with the supplied permissions.
func (OSFileSystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	return os.WriteFile(path, data, permissions)
}
