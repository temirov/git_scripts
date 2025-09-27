package packages

const (
	cliConfigurationRootKeyConstant      = "cli"
	packagesConfigurationRootKeyConstant = cliConfigurationRootKeyConstant + ".packages"
	purgeConfigurationKeyConstant        = packagesConfigurationRootKeyConstant + ".purge"
	purgeOwnerKeyConstant                = purgeConfigurationKeyConstant + ".owner"
	purgePackageNameKeyConstant          = purgeConfigurationKeyConstant + ".package"
	purgeOwnerTypeKeyConstant            = purgeConfigurationKeyConstant + ".owner_type"
	purgeTokenSourceKeyConstant          = purgeConfigurationKeyConstant + ".token_source"
	purgeDryRunKeyConstant               = purgeConfigurationKeyConstant + ".dry_run"
	purgeServiceBaseURLKeyConstant       = purgeConfigurationKeyConstant + ".service_base_url"
	purgePageSizeKeyConstant             = purgeConfigurationKeyConstant + ".page_size"
	defaultTokenSourceValueConstant      = "env:GITHUB_PACKAGES_TOKEN"
	defaultServiceBaseURLValueConstant   = ""
	defaultPageSizeValueConstant         = 0
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

// DefaultConfigurationValues provides Viper defaults for packages settings.
func DefaultConfigurationValues() map[string]any {
	return map[string]any{
		purgeTokenSourceKeyConstant:    defaultTokenSourceValueConstant,
		purgeDryRunKeyConstant:         false,
		purgeServiceBaseURLKeyConstant: defaultServiceBaseURLValueConstant,
		purgePageSizeKeyConstant:       defaultPageSizeValueConstant,
	}
}
