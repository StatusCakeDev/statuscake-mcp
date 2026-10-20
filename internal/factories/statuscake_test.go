package factories_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StatusCakeDev/statuscake-mcp/internal/factories"
)

func valid() factories.ClientConfig {
	return factories.ClientConfig{
		APIToken:   "tok",
		RPS:        4,
		Retries:    3,
		MinBackoff: time.Second,
		MaxBackoff: 30 * time.Second,
	}
}

func TestCreateClientAcceptsAValidConfig(t *testing.T) {
	t.Parallel()

	client, err := factories.CreateClient(valid())
	if err != nil || client == nil {
		t.Fatalf("CreateClient() = %v, %v; want a client", client, err)
	}
}

func TestCreateClientRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mutate func(*factories.ClientConfig)
		want   string
	}{
		"no token":             {func(c *factories.ClientConfig) { c.APIToken = "" }, "API token is required"},
		"zero rate":            {func(c *factories.ClientConfig) { c.RPS = 0 }, "rps must be greater than zero"},
		"negative retries":     {func(c *factories.ClientConfig) { c.Retries = -1 }, "retries must not be negative"},
		"zero min backoff":     {func(c *factories.ClientConfig) { c.MinBackoff = 0 }, "min backoff must be greater than zero"},
		"max below min":        {func(c *factories.ClientConfig) { c.MaxBackoff = time.Millisecond }, "must not be less than min backoff"},
		"all problems at once": {func(c *factories.ClientConfig) { *c = factories.ClientConfig{} }, "API token is required"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := valid()
			tt.mutate(&cfg)

			_, err := factories.CreateClient(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("CreateClient() error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestCreateClientReportsEveryProblemTogether(t *testing.T) {
	t.Parallel()

	_, err := factories.CreateClient(factories.ClientConfig{})
	if err == nil {
		t.Fatal("CreateClient() succeeded")
	}
	for _, want := range []string{"API token", "rps", "min backoff"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
