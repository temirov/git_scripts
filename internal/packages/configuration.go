package packages

const (
	defaultTokenSourceValueConstant = "env:GITHUB_PACKAGES_TOKEN"
)

// Configuration aggregates settings for packages commands.
type Configuration struct {
	Purge PurgeConfiguration `mapstructure:"purge"`
}

// PurgeConfiguration stores options for purging container versions.
type PurgeConfiguration struct {
	PackageName string `mapstructure:"package"`
	DryRun      bool   `mapstructure:"dry_run"`
}

// DefaultConfiguration supplies baseline values for packages configuration.
func DefaultConfiguration() Configuration {
	return Configuration{
		Purge: PurgeConfiguration{},
	}
}
