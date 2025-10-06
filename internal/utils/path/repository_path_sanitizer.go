package pathutils

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	booleanLiteralTrueValueConstant  = "true"
	booleanLiteralFalseValueConstant = "false"
)

// RepositoryPathSanitizerConfiguration controls repository path sanitization behavior.
type RepositoryPathSanitizerConfiguration struct {
	// ExcludeBooleanLiteralCandidates removes arguments that represent boolean literals.
	ExcludeBooleanLiteralCandidates bool
	// PruneNestedPaths removes repository paths that are nested within other provided paths.
	PruneNestedPaths bool
}

// RepositoryPathSanitizer normalizes repository path inputs consistently across commands.
type RepositoryPathSanitizer struct {
	homeExpander  *HomeExpander
	configuration RepositoryPathSanitizerConfiguration
}

// NewRepositoryPathSanitizer constructs a RepositoryPathSanitizer with default behavior.
func NewRepositoryPathSanitizer() *RepositoryPathSanitizer {
	return NewRepositoryPathSanitizerWithConfiguration(nil, RepositoryPathSanitizerConfiguration{})
}

// NewRepositoryPathSanitizerWithConfiguration constructs a RepositoryPathSanitizer using the provided expander and configuration.
func NewRepositoryPathSanitizerWithConfiguration(homeExpander *HomeExpander, configuration RepositoryPathSanitizerConfiguration) *RepositoryPathSanitizer {
	resolvedExpander := homeExpander
	if resolvedExpander == nil {
		resolvedExpander = NewHomeExpander()
	}

	return &RepositoryPathSanitizer{
		homeExpander:  resolvedExpander,
		configuration: configuration,
	}
}

// Sanitize trims whitespace, expands the user's home directory, and removes disallowed values.
func (sanitizer *RepositoryPathSanitizer) Sanitize(candidatePaths []string) []string {
	if sanitizer == nil {
		return sanitizePathsWithExpander(NewHomeExpander(), RepositoryPathSanitizerConfiguration{}, candidatePaths)
	}

	return sanitizePathsWithExpander(sanitizer.homeExpander, sanitizer.configuration, candidatePaths)
}

func sanitizePathsWithExpander(expander *HomeExpander, configuration RepositoryPathSanitizerConfiguration, candidatePaths []string) []string {
	sanitizedPaths := make([]string, 0, len(candidatePaths))
	for candidateIndex := range candidatePaths {
		trimmedCandidate := strings.TrimSpace(candidatePaths[candidateIndex])
		if len(trimmedCandidate) == 0 {
			continue
		}

		if configuration.ExcludeBooleanLiteralCandidates && isBooleanLiteral(trimmedCandidate) {
			continue
		}

		expandedPath := expander.Expand(trimmedCandidate)
		if len(expandedPath) == 0 {
			continue
		}

		sanitizedPaths = append(sanitizedPaths, expandedPath)
	}

	if len(sanitizedPaths) == 0 {
		return nil
	}

	if configuration.PruneNestedPaths {
		return pruneNestedPaths(sanitizedPaths)
	}

	return sanitizedPaths
}

func isBooleanLiteral(candidate string) bool {
	loweredCandidate := strings.ToLower(candidate)
	return loweredCandidate == booleanLiteralTrueValueConstant || loweredCandidate == booleanLiteralFalseValueConstant
}

func pruneNestedPaths(candidatePaths []string) []string {
	if len(candidatePaths) == 0 {
		return nil
	}

	type pathDetails struct {
		originalIndex int
		value         string
		canonical     string
		comparison    string
	}

	paths := make([]pathDetails, 0, len(candidatePaths))
	for index := range candidatePaths {
		canonicalPath := canonicalizePath(candidatePaths[index])
		comparisonPath := comparisonPath(canonicalPath)
		paths = append(paths, pathDetails{
			originalIndex: index,
			value:         candidatePaths[index],
			canonical:     canonicalPath,
			comparison:    comparisonPath,
		})
	}

	sort.SliceStable(paths, func(first int, second int) bool {
		firstLength := len(paths[first].comparison)
		secondLength := len(paths[second].comparison)
		if firstLength == secondLength {
			return paths[first].comparison < paths[second].comparison
		}
		return firstLength < secondLength
	})

	selected := make([]pathDetails, 0, len(paths))
	for _, candidate := range paths {
		skip := false
		for _, existing := range selected {
			if candidate.comparison == existing.comparison {
				skip = true
				break
			}
			if isNestedPath(existing.canonical, candidate.canonical) {
				skip = true
				break
			}
		}
		if !skip {
			selected = append(selected, candidate)
		}
	}

	sort.SliceStable(selected, func(first int, second int) bool {
		return selected[first].originalIndex < selected[second].originalIndex
	})

	pruned := make([]string, 0, len(selected))
	for _, candidate := range selected {
		pruned = append(pruned, candidate.value)
	}

	return pruned
}

func canonicalizePath(path string) string {
	cleanedPath := filepath.Clean(path)
	absolutePath, absoluteError := filepath.Abs(cleanedPath)
	if absoluteError == nil {
		return filepath.Clean(absolutePath)
	}
	return cleanedPath
}

func comparisonPath(path string) string {
	comparison := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		comparison = strings.ToLower(comparison)
	}
	return comparison
}

func isNestedPath(parent string, candidate string) bool {
	parentClean := comparisonPath(parent)
	candidateClean := comparisonPath(candidate)

	if candidateClean == parentClean {
		return true
	}

	if len(candidateClean) <= len(parentClean) {
		return false
	}

	if !strings.HasPrefix(candidateClean, parentClean) {
		return false
	}

	parentEndsWithSeparator := parentClean[len(parentClean)-1] == os.PathSeparator
	if parentEndsWithSeparator {
		return true
	}

	return candidateClean[len(parentClean)] == os.PathSeparator
}
