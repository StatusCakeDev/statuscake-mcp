package domain_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

func incident(start, end time.Duration) domain.Incident {
	return domain.Incident{
		Started: epoch.Add(start),
		Ended:   gofp.Some(epoch.Add(end)),
	}
}

func TestIncident(t *testing.T) {
	t.Parallel()

	now := epoch.Add(3 * time.Hour)

	tests := map[string]struct {
		incident     domain.Incident
		wantOngoing  bool
		wantDuration time.Duration
	}{
		"a resolved incident lasted from start to end": {
			incident:     incident(0, 10*time.Minute),
			wantDuration: 10 * time.Minute,
		},
		"an ongoing incident has lasted until now": {
			incident:     domain.Incident{Started: epoch.Add(2 * time.Hour), Ended: gofp.None[time.Time]()},
			wantOngoing:  true,
			wantDuration: time.Hour,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tt.incident.Ongoing(); got != tt.wantOngoing {
				t.Errorf("Ongoing() = %v, want %v", got, tt.wantOngoing)
			}
			if got := tt.incident.Duration(now); got != tt.wantDuration {
				t.Errorf("Duration() = %v, want %v", got, tt.wantDuration)
			}
		})
	}
}

func TestNewIncidentList(t *testing.T) {
	t.Parallel()

	incidents := []domain.Incident{
		incident(-3*time.Hour, -2*time.Hour),
		incident(0, time.Minute),
		incident(-time.Hour, -30*time.Minute),
	}
	newestFirst := []domain.Incident{incidents[1], incidents[2], incidents[0]}

	tests := map[string]struct {
		matches  []domain.Incident
		limit    int
		more     bool
		want     []domain.Incident
		wantMore bool
	}{
		"orders newest first": {
			matches: incidents,
			want:    newestFirst,
		},
		"truncation keeps the newest and reports more": {
			matches:  incidents,
			limit:    2,
			want:     newestFirst[:2],
			wantMore: true,
		},
		"a limit equal to the incidents reports no more": {
			matches: incidents,
			limit:   3,
			want:    newestFirst,
		},
		"more is passed through when the source held further pages": {
			matches:  incidents[:1],
			more:     true,
			want:     incidents[:1],
			wantMore: true,
		},
		"nothing to list": {},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := domain.NewIncidentList(tt.matches, tt.limit, tt.more)

			if diff := cmp.Diff(tt.want, got.Incidents, cmpOptions()); diff != "" {
				t.Errorf("incidents mismatch (-want +got):\n%s", diff)
			}
			if got.More != tt.wantMore {
				t.Errorf("More = %v, want %v", got.More, tt.wantMore)
			}
		})
	}
}
