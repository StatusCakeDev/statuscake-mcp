package domain_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var epoch = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func entryAt(offset time.Duration) domain.CheckHistoryEntry {
	return domain.CheckHistoryEntry{At: epoch.Add(offset)}
}

func TestNewCheckHistory(t *testing.T) {
	t.Parallel()

	shuffled := []domain.CheckHistoryEntry{
		entryAt(-2 * time.Hour),
		entryAt(0),
		entryAt(-time.Hour),
	}

	tests := map[string]struct {
		entries  []domain.CheckHistoryEntry
		limit    int
		want     []domain.CheckHistoryEntry
		wantMore bool
	}{
		"orders newest first": {
			entries: shuffled,
			want:    []domain.CheckHistoryEntry{entryAt(0), entryAt(-time.Hour), entryAt(-2 * time.Hour)},
		},
		"truncation keeps the newest and reports more": {
			entries:  shuffled,
			limit:    2,
			want:     []domain.CheckHistoryEntry{entryAt(0), entryAt(-time.Hour)},
			wantMore: true,
		},
		"a limit equal to the entries reports no more": {
			entries: shuffled[:2],
			limit:   2,
			want:    []domain.CheckHistoryEntry{entryAt(0), entryAt(-2 * time.Hour)},
		},
		"nothing to list": {},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := domain.NewCheckHistory(tt.entries, tt.limit)

			if diff := cmp.Diff(tt.want, got.Entries, cmpOptions()); diff != "" {
				t.Errorf("entries mismatch (-want +got):\n%s", diff)
			}
			if got.More != tt.wantMore {
				t.Errorf("More = %v, want %v", got.More, tt.wantMore)
			}
		})
	}
}
