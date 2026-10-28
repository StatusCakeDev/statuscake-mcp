//go:build dev

package flags

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// developmentFlags are the settings that exist only in development builds,
// which are made with `go build -tags dev`.
type developmentFlags struct {
	CustomEndpoint string `mapstructure:"custom_endpoint"`
}

func (f *ConfigFlags) addDevelopmentFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.CustomEndpoint, "custom-endpoint", "", "Custom endpoint to which requests will be made. Only available in development builds")
	if err := viper.BindPFlag("custom_endpoint", flags.Lookup("custom-endpoint")); err != nil {
		panic(err)
	}
	viper.MustBindEnv("custom_endpoint", "STATUSCAKE_CUSTOM_ENDPOINT")
}

// Endpoint returns the API host to use in place of StatusCake's, or an empty
// string for the default.
func (f *ConfigFlags) Endpoint() string { return f.CustomEndpoint }
