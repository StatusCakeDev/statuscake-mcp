// Package flags defines the configuration flags for the StatusCake MCP server,
// allowing users to set options via command-line flags, environment variables,
// or configuration files. It provides a structured way to manage server
// settings such as API tokens, request limits, retry policies, and custom
// endpoints.
package flags

import (
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ConfigFlags is the configuration of the server. Each field can be set by a
// flag, an environment variable or a key in the config file; where more than
// one is set the order of precedence is that of the list, so a flag beats an
// environment variable, which beats the config file, which beats the default.
//
// The mapstructure tags are the config file keys, and the viper keys that the
// flags and environment variables are bound to.
type ConfigFlags struct {
	APIToken       string        `mapstructure:"api_token"`
	RPS            int           `mapstructure:"rps"`
	Retries        int           `mapstructure:"retries"`
	MinBackoff     time.Duration `mapstructure:"min_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
	CustomEndpoint string        `mapstructure:"custom_endpoint"`
}

// AddFlags registers the flags and binds each to viper, together with the
// environment variable and config file key that can also set it.
func (f *ConfigFlags) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.APIToken, "api-token", "", "The API token used to authenticate with the StatusCake API")
	if err := viper.BindPFlag("api_token", flags.Lookup("api-token")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("api_token", "STATUSCAKE_API_TOKEN")

	flags.IntVar(&f.RPS, "rps", 4, "Maximum requests per second to make to the API")
	if err := viper.BindPFlag("rps", flags.Lookup("rps")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("rps", "STATUSCAKE_RPS")

	flags.IntVar(&f.Retries, "retries", 3, "Maximum number of retries to perform when an API request fails")
	if err := viper.BindPFlag("retries", flags.Lookup("retries")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("retries", "STATUSCAKE_RETRIES")

	flags.DurationVar(&f.MinBackoff, "min-backoff", time.Second, "Minimum wait after a failed API call, with a unit such as 500ms or 2s")
	if err := viper.BindPFlag("min_backoff", flags.Lookup("min-backoff")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("min_backoff", "STATUSCAKE_MIN_BACKOFF")

	flags.DurationVar(&f.MaxBackoff, "max-backoff", 30*time.Second, "Maximum wait after a failed API call, with a unit such as 30s or 1m")
	if err := viper.BindPFlag("max_backoff", flags.Lookup("max-backoff")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("max_backoff", "STATUSCAKE_MAX_BACKOFF")

	flags.StringVar(&f.CustomEndpoint, "custom-endpoint", "", "Custom endpoint to which requests will be made")
	if err := viper.BindPFlag("custom_endpoint", flags.Lookup("custom-endpoint")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("custom_endpoint", "STATUSCAKE_CUSTOM_ENDPOINT")
}
