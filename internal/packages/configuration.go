package packages

import "strings"

const (
	defaultTokenSourceValueConstant = "env:GITHUB_PACKAGES_TOKEN"
)

// Configuration aggregates settings for packages commands.
type Configuration struct {
	Purge PurgeConfiguration `mapstructure:"purge"`
}

// PurgeConfiguration stores options for purging container versions.
type PurgeConfiguration struct {
	PackageName     string   `mapstructure:"package"`
	DryRun          bool     `mapstructure:"dry_run"`
	RepositoryRoots []string `mapstructure:"roots"`
}

// DefaultConfiguration supplies baseline values for packages configuration.
func DefaultConfiguration() Configuration {
	return Configuration{
		Purge: PurgeConfiguration{},
	}
}

// sanitize trims configured values and removes empty entries.
func (configuration Configuration) sanitize() Configuration {
	sanitized := configuration
	sanitized.Purge = configuration.Purge.sanitize()
	return sanitized
}

func (configuration PurgeConfiguration) sanitize() PurgeConfiguration {
	sanitized := configuration
	sanitized.RepositoryRoots = sanitizeRoots(configuration.RepositoryRoots)
	return sanitized
}

func sanitizeRoots(candidateRoots []string) []string {
	sanitizedRoots := make([]string, 0, len(candidateRoots))
	for _, rootCandidate := range candidateRoots {
		trimmedRoot := strings.TrimSpace(rootCandidate)
		if len(trimmedRoot) == 0 {
			continue
		}
		sanitizedRoots = append(sanitizedRoots, trimmedRoot)
	}
	if len(sanitizedRoots) == 0 {
		return nil
	}
	return sanitizedRoots
}
