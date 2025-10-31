package githubauth

import (
	"errors"
	"os"
	"strings"
)

// Environment variable names used by GitHub authentication helpers.
const (
	EnvGitHubCLIToken = "GH_TOKEN"
	EnvGitHubToken    = "GITHUB_TOKEN"
	EnvGitHubAPIToken = "GITHUB_API_TOKEN"
)

var tokenPreference = []string{
	EnvGitHubCLIToken,
	EnvGitHubToken,
	EnvGitHubAPIToken,
}

const tokenMissingMessage = "missing GitHub authentication token; set GH_TOKEN, GITHUB_TOKEN, or GITHUB_API_TOKEN"

// ResolveToken returns the first non-empty GitHub authentication token observed
// in the provided environment map or the process environment.
func ResolveToken(environment map[string]string) (string, bool) {
	for _, key := range tokenPreference {
		if value, ok := lookup(environment, key); ok {
			return value, true
		}
	}
	for _, key := range tokenPreference {
		if value, ok := os.LookupEnv(key); ok {
			value = strings.TrimSpace(value)
			if len(value) > 0 {
				return value, true
			}
		}
	}
	return "", false
}

func lookup(environment map[string]string, key string) (string, bool) {
	if environment == nil {
		return "", false
	}
	value, exists := environment[key]
	if !exists {
		return "", false
	}
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return "", false
	}
	return value, true
}

// TokenRequirement describes the token validation strategy for GitHub commands.
type TokenRequirement int

const (
	TokenRequired TokenRequirement = iota
	TokenOptional
)

// MissingTokenError surfaces missing GitHub authentication tokens.
type MissingTokenError struct {
	Operation string
	Critical  bool
}

// Error returns the canonical missing-token message.
func (err MissingTokenError) Error() string {
	return tokenMissingMessage
}

// Is enables errors.Is checks against MissingTokenError sentinels.
func (err MissingTokenError) Is(target error) bool {
	switch typed := target.(type) {
	case MissingTokenError:
		return err.Critical == typed.Critical
	case *MissingTokenError:
		return err.Critical == typed.Critical
	default:
		return false
	}
}

// Critical reports whether the missing token should be treated as fatal.
func (err MissingTokenError) CriticalRequirement() bool {
	return err.Critical
}

// ErrTokenMissing denotes a critical missing token.
var ErrTokenMissing = MissingTokenError{Critical: true}

// ErrTokenMissingOptional denotes a missing token for an optional GitHub operation.
var ErrTokenMissingOptional = MissingTokenError{Critical: false}

// NewMissingTokenError constructs a MissingTokenError for the given operation.
func NewMissingTokenError(operation string, critical bool) MissingTokenError {
	return MissingTokenError{Operation: operation, Critical: critical}
}

// IsMissingTokenError returns the underlying MissingTokenError when present.
func IsMissingTokenError(err error) (MissingTokenError, bool) {
	var missing MissingTokenError
	if errors.As(err, &missing) {
		return missing, true
	}
	return MissingTokenError{}, false
}
