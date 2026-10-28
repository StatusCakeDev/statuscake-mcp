package domain_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

func window(id string, state domain.MaintenanceState, start time.Duration) domain.MaintenanceWindow {
	return domain.MaintenanceWindow{
		ID:    id,
		State: state,
		Start: epoch.Add(start),
	}
}

func TestNewMaintenanceWindowList(t *testing.T) {
	t.Parallel()

	windows := []domain.MaintenanceWindow{
		window("paused", domain.MaintenanceStatePaused, 0),
		window("late", domain.MaintenanceStatePending, 48*time.Hour),
		window("now", domain.MaintenanceStateActive, -time.Hour),
		window("soon", domain.MaintenanceStatePending, time.Hour),
	}

	tests := map[string]struct {
		limit         int
		wantIDs       []string
		wantTotal     int
		wantTruncated bool
	}{
		"active first, then pending, then paused, each by start": {
			wantIDs:   []string{"now", "soon", "late", "paused"},
			wantTotal: 4,
		},
		"truncation keeps what matters now": {
			limit:         2,
			wantIDs:       []string{"now", "soon"},
			wantTotal:     4,
			wantTruncated: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			list := domain.NewMaintenanceWindowList(windows, tt.limit)

			ids := make([]string, len(list.Windows))
			for i, w := range list.Windows {
				ids[i] = w.ID
			}
			if diff := cmp.Diff(tt.wantIDs, ids); diff != "" {
				t.Errorf("ids mismatch (-want +got):\n%s", diff)
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

func TestParseMaintenanceState(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw     string
		want    domain.MaintenanceState
		wantErr string
	}{
		"active": {
			raw:  "active",
			want: domain.MaintenanceStateActive,
		},
		"normalises case": {
			raw:  " Pending ",
			want: domain.MaintenanceStatePending,
		},
		"paused": {
			raw:  "paused",
			want: domain.MaintenanceStatePaused,
		},
		"names the valid ones": {
			raw:     "over",
			wantErr: "active, pending, paused",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.ParseMaintenanceState(tt.raw)
			assertParse(t, got, err, tt.want, tt.wantErr)
		})
	}
}
