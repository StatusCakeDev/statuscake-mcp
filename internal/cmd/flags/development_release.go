//go:build !dev

package flags

import "github.com/spf13/pflag"

// developmentFlags are the settings that exist only in development builds. A
// release build has none, so users cannot point the server at anything but the
// StatusCake API.
type developmentFlags struct{}

func (f *ConfigFlags) addDevelopmentFlags(*pflag.FlagSet) {}

// Endpoint returns the API host to use in place of StatusCake's, or an empty
// string for the default. A release build always returns the default.
func (f *ConfigFlags) Endpoint() string { return "" }
