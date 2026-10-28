package domain_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

func TestNewContactGroupList(t *testing.T) {
	t.Parallel()

	groups := []domain.ContactGroup{
		{ID: "3", Name: "platform"},
		{ID: "1", Name: "Billing"},
		{ID: "2", Name: "billing"},
		{ID: "4", Name: "Ops"},
	}

	tests := map[string]struct {
		limit         int
		wantIDs       []string
		wantTotal     int
		wantTruncated bool
	}{
		"orders by name ignoring case, then by id": {
			wantIDs:   []string{"1", "2", "4", "3"},
			wantTotal: 4,
		},
		"truncation keeps the first by name": {
			limit:         2,
			wantIDs:       []string{"1", "2"},
			wantTotal:     4,
			wantTruncated: true,
		},
		"a limit equal to the groups is not truncation": {
			limit:     4,
			wantIDs:   []string{"1", "2", "4", "3"},
			wantTotal: 4,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			list := domain.NewContactGroupList(groups, tt.limit)

			ids := make([]string, len(list.Groups))
			for i, g := range list.Groups {
				ids[i] = g.ID
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
