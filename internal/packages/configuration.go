package packages

const (
	defaultTokenSourceValueConstant    = "env:GITHUB_PACKAGES_TOKEN"
	defaultServiceBaseURLValueConstant = ""
	defaultPageSizeValueConstant       = 0
)

// Configuration aggregates settings for packages commands.
type Configuration struct {
	Purge PurgeConfiguration `mapstructure:"purge"`
}

// PurgeConfiguration stores options for purging container versions.
type PurgeConfiguration struct {
	Owner          string `mapstructure:"owner"`
	PackageName    string `mapstructure:"package"`
	OwnerType      string `mapstructure:"owner_type"`
	TokenSource    string `mapstructure:"token_source"`
	DryRun         bool   `mapstructure:"dry_run"`
	ServiceBaseURL string `mapstructure:"service_base_url"`
	PageSize       int    `mapstructure:"page_size"`
}

// DefaultConfiguration supplies baseline values for packages configuration.
func DefaultConfiguration() Configuration {
	return Configuration{
		Purge: PurgeConfiguration{
			TokenSource:    defaultTokenSourceValueConstant,
			ServiceBaseURL: defaultServiceBaseURLValueConstant,
			PageSize:       defaultPageSizeValueConstant,
		},
	}
}
