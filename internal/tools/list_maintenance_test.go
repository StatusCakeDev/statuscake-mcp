package tools_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

func windows() []domain.MaintenanceWindow {
	return []domain.MaintenanceWindow{
		{
			ID: "2", Name: "Database upgrade", State: domain.MaintenanceStatePending,
			Start: epoch.Add(48 * time.Hour), End: epoch.Add(50 * time.Hour),
			Repeat: "never", Timezone: "Europe/London", Checks: 4,
		},
		{
			ID: "1", Name: "Deploy", State: domain.MaintenanceStateActive,
			Start: epoch.Add(-time.Hour), End: epoch.Add(time.Hour),
			Repeat: "weekly", Timezone: "UTC", Tags: []string{"production"},
		},
	}
}

func TestListMaintenanceWindowsBuildsViews(t *testing.T) {
	t.Parallel()

	fake := &fakeWindows{windows: windows()}
	h := tools.NewListMaintenanceWindowsHandler(discardLogger(), fake)

	_, got, err := h.Handle(t.Context(), nil, tools.ListMaintenanceWindowsInput{Limit: 1})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := tools.ListMaintenanceWindowsOutput{
		MaintenanceWindows: []tools.MaintenanceWindowView{{
			ID: "1", Name: "Deploy", State: "active",
			Start: "2026-10-01T11:00:00Z", End: "2026-10-01T13:00:00Z",
			Repeat: "weekly", Timezone: "UTC", Tags: []string{"production"},
		}},
		Total:     2,
		Truncated: true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestListMaintenanceWindowsMarshalsMissingTagsAsAnEmptyList(t *testing.T) {
	t.Parallel()

	h := tools.NewListMaintenanceWindowsHandler(discardLogger(), &fakeWindows{windows: windows()})

	_, got, err := h.Handle(t.Context(), nil, tools.ListMaintenanceWindowsInput{State: "pending"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(got.MaintenanceWindows) != 1 || got.MaintenanceWindows[0].Tags == nil {
		t.Errorf("got %+v, want one window with non-nil tags", got.MaintenanceWindows)
	}
}

func TestListMaintenanceWindowsTranslatesTheStateFilter(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want domain.MaintenanceWindowFilter
	}{
		"absent": {want: domain.MaintenanceWindowFilter{Limit: 25}},
		"parsed": {
			in:   " ACTIVE ",
			want: domain.MaintenanceWindowFilter{State: gofp.Some(domain.MaintenanceStateActive), Limit: 25},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeWindows{}
			h := tools.NewListMaintenanceWindowsHandler(discardLogger(), fake)

			if _, _, err := h.Handle(t.Context(), nil, tools.ListMaintenanceWindowsInput{State: tt.in}); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, fake.got, cmpOptions()); diff != "" {
				t.Errorf("filter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListMaintenanceWindowsRejectsAnInvalidStateWithTheValidValues(t *testing.T) {
	t.Parallel()

	fake := &fakeWindows{}
	h := tools.NewListMaintenanceWindowsHandler(discardLogger(), fake)

	_, _, err := h.Handle(t.Context(), nil, tools.ListMaintenanceWindowsInput{State: "over"})
	if err == nil || !strings.Contains(err.Error(), "active, pending, paused") {
		t.Fatalf("error = %v, want one listing the valid states", err)
	}
}

func TestListMaintenanceWindowsWrapsPortFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	h := tools.NewListMaintenanceWindowsHandler(discardLogger(), &fakeWindows{err: cause})

	_, _, err := h.Handle(t.Context(), nil, tools.ListMaintenanceWindowsInput{})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "could not list maintenance windows") {
		t.Fatalf("error = %v, want a wrapped failure", err)
	}
}
