package pathutils

import "strings"

const (
	booleanLiteralTrueValueConstant  = "true"
	booleanLiteralFalseValueConstant = "false"
)

// RepositoryPathSanitizerConfiguration controls repository path sanitization behavior.
type RepositoryPathSanitizerConfiguration struct {
	// ExcludeBooleanLiteralCandidates removes arguments that represent boolean literals.
	ExcludeBooleanLiteralCandidates bool
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

	return sanitizedPaths
}

func isBooleanLiteral(candidate string) bool {
	loweredCandidate := strings.ToLower(candidate)
	return loweredCandidate == booleanLiteralTrueValueConstant || loweredCandidate == booleanLiteralFalseValueConstant
}
