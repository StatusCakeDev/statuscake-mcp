package factories

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"golang.org/x/time/rate"

	"github.com/StatusCakeDev/statuscake-go"
	"github.com/StatusCakeDev/statuscake-go/backoff"
	"github.com/StatusCakeDev/statuscake-go/credentials"
	"github.com/StatusCakeDev/statuscake-go/throttle"
)

// ClientConfig is everything needed to build a StatusCake client.
type ClientConfig struct {
	APIToken       string
	RPS            int
	Retries        int
	MinBackoff     time.Duration
	MaxBackoff     time.Duration
	CustomEndpoint string

	// Version and Commit identify this build of statuscake-mcp. They form the
	// User-Agent sent to StatusCake, which helps identify the source of requests
	// in its logs.
	Version string
	Commit  string
}

// validate rejects settings that would not fail until the first request, or
// that would quietly misbehave: a rate of zero never lets a second request
// through, and a negative retry count is meaningless.
func (c ClientConfig) validate() error {
	var errs []error
	if c.APIToken == "" {
		errs = append(errs, errors.New("API token is required: set STATUSCAKE_API_TOKEN or pass --api-token"))
	}
	if c.RPS <= 0 {
		errs = append(errs, fmt.Errorf("rps must be greater than zero, got %d", c.RPS))
	}
	if c.Retries < 0 {
		errs = append(errs, fmt.Errorf("retries must not be negative, got %d", c.Retries))
	}
	if c.MinBackoff <= 0 {
		errs = append(errs, fmt.Errorf("min backoff must be greater than zero, got %s", c.MinBackoff))
	}
	if c.MaxBackoff < c.MinBackoff {
		errs = append(errs, fmt.Errorf("max backoff (%s) must not be less than min backoff (%s)", c.MaxBackoff, c.MinBackoff))
	}
	return errors.Join(errs...)
}

func (c ClientConfig) userAgent() string {
	version := c.Version
	if version == "" {
		version = "dev"
	}
	if c.Commit != "" {
		version = fmt.Sprintf("%s (%s)", version, c.Commit)
	}
	return fmt.Sprintf("statuscake-mcp/%s (%s; %s; %s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// CreateClient builds a StatusCake client that is rate limited and retries
// with exponential backoff.
func CreateClient(cfg ClientConfig) (*statuscake.Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	bearer := credentials.NewBearerWithStaticToken(cfg.APIToken)
	opts := []statuscake.Option{
		statuscake.WithBackoff(backoff.Exponential{
			BaseDelay:  cfg.MinBackoff,
			Multiplier: 2.0,
			Jitter:     0.2,
			MaxDelay:   cfg.MaxBackoff,
		}),
		statuscake.WithHTTPClient(&http.Client{
			Transport: throttle.NewWithDefaultTransport(
				rate.NewLimiter(rate.Limit(cfg.RPS), 1),
			),
		}),
		statuscake.WithMaxRetries(cfg.Retries),
		statuscake.WithRequestCredentials(bearer),
		statuscake.WithUserAgent(cfg.userAgent()),
	}

	if cfg.CustomEndpoint != "" {
		opts = append(opts, statuscake.WithHost(cfg.CustomEndpoint))
	}

	return statuscake.NewClient(opts...), nil
}
