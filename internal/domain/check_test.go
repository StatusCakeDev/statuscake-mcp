package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

func summary(id, name string, status domain.CheckStatus) domain.CheckSummary {
	return domain.CheckSummary{
		CheckRef: domain.CheckRef{Type: domain.CheckTypeUptime, ID: id},
		Name:     name,
		Status:   status,
	}
}

func names(list domain.CheckList) []string {
	out := make([]string, len(list.Checks))
	for i, c := range list.Checks {
		out[i] = c.Name
	}
	return out
}

func TestNewCheckList(t *testing.T) {
	t.Parallel()

	mixed := []domain.CheckSummary{
		summary("1", "zeta", domain.CheckStatusUp),
		summary("2", "Beta", domain.CheckStatusDown),
		summary("3", "gamma", domain.CheckStatusPaused),
		summary("4", "alpha", domain.CheckStatusDown),
		summary("5", "delta", domain.CheckStatusUp),
		summary("6", "eta", domain.CheckStatusUnknown),
	}

	tests := map[string]struct {
		matches       []domain.CheckSummary
		limit         int
		wantNames     []string
		wantTotal     int
		wantTruncated bool
	}{
		"orders down, then paused, then the rest, alphabetically and ignoring case": {
			matches:   mixed,
			wantNames: []string{"alpha", "Beta", "gamma", "delta", "eta", "zeta"},
			wantTotal: 6,
		},
		"truncation keeps the most urgent checks": {
			matches:       mixed,
			limit:         3,
			wantNames:     []string{"alpha", "Beta", "gamma"},
			wantTotal:     6,
			wantTruncated: true,
		},
		"a limit equal to the matches is not truncation": {
			matches:   mixed[:2],
			limit:     2,
			wantNames: []string{"Beta", "zeta"},
			wantTotal: 2,
		},
		"a limit of zero keeps everything": {
			matches:   mixed[:2],
			wantNames: []string{"Beta", "zeta"},
			wantTotal: 2,
		},
		"no matches": {
			wantNames: []string{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			list := domain.NewCheckList(tt.matches, tt.limit)

			if diff := cmp.Diff(tt.wantNames, names(list)); diff != "" {
				t.Errorf("names mismatch (-want +got):\n%s", diff)
			}
			if list.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", list.Total, tt.wantTotal)
			}
			if got := list.Truncated(); got != tt.wantTruncated {
				t.Errorf("Truncated() = %v, want %v", got, tt.wantTruncated)
			}
		})
	}
}

func TestNewCheckListDoesNotReorderItsInput(t *testing.T) {
	t.Parallel()

	matches := []domain.CheckSummary{
		summary("1", "up", domain.CheckStatusUp),
		summary("2", "down", domain.CheckStatusDown),
	}
	want := append([]domain.CheckSummary(nil), matches...)

	domain.NewCheckList(matches, 0)

	if diff := cmp.Diff(want, matches); diff != "" {
		t.Errorf("input was modified (-want +got):\n%s", diff)
	}
}

func TestCheckFilterMatches(t *testing.T) {
	t.Parallel()

	check := domain.CheckSummary{
		CheckRef: domain.CheckRef{Type: domain.CheckTypeHeartbeat, ID: "1"},
		Status:   domain.CheckStatusDown,
		Tags:     []string{"production", "payments"},
	}

	tests := map[string]struct {
		filter domain.CheckFilter
		want   bool
	}{
		"an empty filter matches everything": {
			want: true,
		},
		"the limit is not a criterion": {
			filter: domain.CheckFilter{Limit: 1},
			want:   true,
		},
		"matching type": {
			filter: domain.CheckFilter{Type: gofp.Some(domain.CheckTypeHeartbeat)},
			want:   true,
		},
		"other type": {
			filter: domain.CheckFilter{Type: gofp.Some(domain.CheckTypeUptime)},
		},
		"matching status": {
			filter: domain.CheckFilter{Status: gofp.Some(domain.CheckStatusDown)},
			want:   true,
		},
		"other status": {
			filter: domain.CheckFilter{Status: gofp.Some(domain.CheckStatusUp)},
		},
		"carried tag": {
			filter: domain.CheckFilter{Tag: gofp.Some("payments")},
			want:   true,
		},
		"missing tag": {
			filter: domain.CheckFilter{Tag: gofp.Some("staging")},
		},
		"every criterion must hold": {
			filter: domain.CheckFilter{
				Type:   gofp.Some(domain.CheckTypeHeartbeat),
				Status: gofp.Some(domain.CheckStatusUp),
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tt.filter.Matches(check); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCheckType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw     string
		want    domain.CheckType
		wantErr string
	}{
		"exact":                {raw: "ssl", want: domain.CheckTypeSSL},
		"normalises case":      {raw: " Uptime ", want: domain.CheckTypeUptime},
		"pagespeed":            {raw: "pagespeed", want: domain.CheckTypePagespeed},
		"heartbeat":            {raw: "heartbeat", want: domain.CheckTypeHeartbeat},
		"names the valid ones": {raw: "test", wantErr: "uptime, pagespeed, ssl, heartbeat"},
		"empty":                {raw: "", wantErr: "invalid check type"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.ParseCheckType(tt.raw)
			assertParse(t, got, err, tt.want, tt.wantErr)
		})
	}
}

func TestParseCheckStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw     string
		want    domain.CheckStatus
		wantErr string
	}{
		"up":                   {raw: "up", want: domain.CheckStatusUp},
		"normalises case":      {raw: " DOWN", want: domain.CheckStatusDown},
		"paused":               {raw: "paused", want: domain.CheckStatusPaused},
		"names the valid ones": {raw: "dwon", wantErr: "up, down, paused"},
		"unknown is output only": {
			raw:     "unknown",
			wantErr: "invalid check status",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.ParseCheckStatus(tt.raw)
			assertParse(t, got, err, tt.want, tt.wantErr)
		})
	}
}

func TestNewCheckTakesItsTypeFromTheSettings(t *testing.T) {
	t.Parallel()

	// The summary claims "uptime"; the settings are SSL. The settings win, so a
	// check cannot disagree with its own configuration.
	s := summary("9", "cert", domain.CheckStatusUp)

	check := domain.NewCheck(s, domain.SSLSettings{Target: "example.com"})

	if check.Type != domain.CheckTypeSSL {
		t.Fatalf("Type = %q, want %q", check.Type, domain.CheckTypeSSL)
	}
	if check.ID != "9" || check.Name != "cert" {
		t.Errorf("summary was not preserved: %+v", check.CheckSummary)
	}
}

func TestFoldSettingsSelectsTheMatchingCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		settings domain.CheckSettings
		want     string
	}{
		{domain.UptimeSettings{}, "uptime"},
		{domain.PageSpeedSettings{}, "pagespeed"},
		{domain.SSLSettings{}, "ssl"},
		{domain.HeartbeatSettings{Period: time.Hour}, "heartbeat"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			got := domain.FoldSettings(tt.settings,
				func(domain.UptimeSettings) string { return "uptime" },
				func(domain.PageSpeedSettings) string { return "pagespeed" },
				func(domain.SSLSettings) string { return "ssl" },
				func(domain.HeartbeatSettings) string { return "heartbeat" },
			)
			if got != tt.want {
				t.Errorf("FoldSettings() = %q, want %q", got, tt.want)
			}
			if got := tt.settings.Type().String(); got != tt.want {
				t.Errorf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

// assertParse is shared by the Parse* tests: either the value matches, or the
// error contains the expected text.
func assertParse[T comparable](t *testing.T, got T, err error, want T, wantErr string) {
	t.Helper()

	if wantErr != "" {
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("error = %v, want one containing %q", err, wantErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
