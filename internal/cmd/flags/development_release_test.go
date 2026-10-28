//go:build !dev

package flags_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/StatusCakeDev/statuscake-mcp/internal/cmd/flags"
)

// These checks share viper's global state, so they do not run in parallel.

func TestReleaseBuildsHaveNoEndpointFlag(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	var f flags.ConfigFlags
	set := pflag.NewFlagSet("check", pflag.ContinueOnError)
	f.AddFlags(set)

	if set.Lookup("custom-endpoint") != nil {
		t.Error("a release build offers --custom-endpoint")
	}
}

func TestReleaseBuildsIgnoreEndpointFromConfigAndEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("STATUSCAKE_CUSTOM_ENDPOINT", "http://localhost:1")

	var f flags.ConfigFlags
	f.AddFlags(pflag.NewFlagSet("check", pflag.ContinueOnError))
	viper.AutomaticEnv()
	viper.Set("custom_endpoint", "http://localhost:2")

	if err := viper.Unmarshal(&f); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := f.Endpoint(); got != "" {
		t.Errorf("Endpoint() = %q, want it empty in a release build", got)
	}
}
