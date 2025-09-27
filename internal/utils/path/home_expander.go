package pathutils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	tildeSymbolConstant             = "~"
	tildeForwardSlashPrefixConstant = "~/"
)

var tildeWithPathSeparatorPrefix = tildeSymbolConstant + string(os.PathSeparator)

// HomeDirectoryProvider resolves the current user's home directory path.
type HomeDirectoryProvider func() (string, error)

// HomeExpander converts user home shortcuts to absolute paths.
type HomeExpander struct {
	homeDirectoryProvider HomeDirectoryProvider
	homeDirectory         string
	homeDirectoryError    error
	initializationGuard   sync.Once
}

// NewHomeExpander constructs a HomeExpander using the operating system lookup.
func NewHomeExpander() *HomeExpander {
	return NewHomeExpanderWithProvider(os.UserHomeDir)
}

// NewHomeExpanderWithProvider constructs a HomeExpander with a custom provider.
func NewHomeExpanderWithProvider(provider HomeDirectoryProvider) *HomeExpander {
	if provider == nil {
		provider = os.UserHomeDir
	}
	return &HomeExpander{homeDirectoryProvider: provider}
}

// Expand resolves leading tilde prefixes to the user's home directory.
func (expander *HomeExpander) Expand(candidatePath string) string {
	if expander == nil {
		return candidatePath
	}
	if len(candidatePath) == 0 {
		return candidatePath
	}
	if !strings.HasPrefix(candidatePath, tildeSymbolConstant) {
		return candidatePath
	}

	resolvedHomeDirectory := expander.resolveHomeDirectory()
	if len(resolvedHomeDirectory) == 0 {
		return candidatePath
	}

	if candidatePath == tildeSymbolConstant {
		return resolvedHomeDirectory
	}

	if strings.HasPrefix(candidatePath, tildeForwardSlashPrefixConstant) {
		relativePath := strings.TrimPrefix(candidatePath, tildeForwardSlashPrefixConstant)
		return filepath.Join(resolvedHomeDirectory, relativePath)
	}

	if tildeWithPathSeparatorPrefix != tildeForwardSlashPrefixConstant && strings.HasPrefix(candidatePath, tildeWithPathSeparatorPrefix) {
		relativePath := strings.TrimPrefix(candidatePath, tildeWithPathSeparatorPrefix)
		return filepath.Join(resolvedHomeDirectory, relativePath)
	}

	return candidatePath
}

func (expander *HomeExpander) resolveHomeDirectory() string {
	expander.initializationGuard.Do(func() {
		expander.homeDirectory, expander.homeDirectoryError = expander.homeDirectoryProvider()
	})
	if expander.homeDirectoryError != nil {
		return ""
	}
	return expander.homeDirectory
}
