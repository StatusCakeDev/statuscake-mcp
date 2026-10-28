//go:build dev

package flags_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/StatusCakeDev/statuscake-mcp/internal/cmd/flags"
)

// These checks share viper's global state, so they do not run in parallel.

func TestDevelopmentBuildsReadTheEndpointFromTheEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("STATUSCAKE_CUSTOM_ENDPOINT", "http://localhost:1")

	var f flags.ConfigFlags
	f.AddFlags(pflag.NewFlagSet("check", pflag.ContinueOnError))
	if err := viper.Unmarshal(&f); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got, want := f.Endpoint(), "http://localhost:1"; got != want {
		t.Errorf("Endpoint() = %q, want %q", got, want)
	}
}
