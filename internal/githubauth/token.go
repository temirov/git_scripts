package githubauth

import (
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
