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

func TestListIncidentsTranslatesArgumentsIntoADomainFilter(t *testing.T) {
	t.Parallel()

	fake := &fakeIncidents{}
	h := tools.NewListIncidentsHandler(discardLogger(), fake)

	_, _, err := h.Handle(t.Context(), nil, tools.ListIncidentsInput{
		Type: "uptime", ID: "7", After: "2026-10-01T00:00:00Z", Limit: 3,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := domain.IncidentFilter{
		CheckRef: domain.CheckRef{Type: domain.CheckTypeUptime, ID: "7"},
		After:    gofp.Some(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)),
		Limit:    3,
	}
	if diff := cmp.Diff(want, fake.got, cmpOptions()); diff != "" {
		t.Errorf("filter mismatch (-want +got):\n%s", diff)
	}
}

func TestListIncidentsBuildsViewsUsingTheClockForOngoingOnes(t *testing.T) {
	t.Parallel()

	fake := &fakeIncidents{result: domain.IncidentList{
		Incidents: []domain.Incident{
			{Started: epoch.Add(-30 * time.Minute)}, // still down
			{Started: epoch.Add(-5 * time.Hour), Ended: gofp.Some(epoch.Add(-4 * time.Hour))},
		},
		More: true,
	}}

	h := tools.NewListIncidentsHandler(discardLogger(), fake, tools.WithClock(func() time.Time { return epoch }))

	_, got, err := h.Handle(t.Context(), nil, tools.ListIncidentsInput{Type: "uptime", ID: "7"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := tools.ListIncidentsOutput{
		Incidents: []tools.IncidentView{
			{StartedAt: "2026-10-01T11:30:00Z", Ongoing: true, DurationSeconds: 1800},
			{StartedAt: "2026-10-01T07:00:00Z", EndedAt: "2026-10-01T08:00:00Z", DurationSeconds: 3600},
		},
		More: true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestListIncidentsWithNoneIsAnEmptyListNotNull(t *testing.T) {
	t.Parallel()

	h := tools.NewListIncidentsHandler(discardLogger(), &fakeIncidents{})

	_, got, err := h.Handle(t.Context(), nil, tools.ListIncidentsInput{Type: "uptime", ID: "7"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got.Incidents == nil || len(got.Incidents) != 0 {
		t.Errorf("Incidents = %#v, want a non-nil empty slice", got.Incidents)
	}
}

func TestListIncidentsRejectsBadArguments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.ListIncidentsInput
		want string
	}{
		"blank id": {
			in:   tools.ListIncidentsInput{Type: "uptime"},
			want: "list_checks",
		},
		"bad type": {
			in:   tools.ListIncidentsInput{Type: "x", ID: "1"},
			want: "invalid check type",
		},
		"bad before": {
			in:   tools.ListIncidentsInput{Type: "uptime", ID: "1", Before: "now"},
			want: "before: invalid time",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeIncidents{}
			h := tools.NewListIncidentsHandler(discardLogger(), fake)

			_, _, err := h.Handle(t.Context(), nil, tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want one containing %q", err, tt.want)
			}
			if fake.got.ID != "" {
				t.Error("the port was called despite invalid arguments")
			}
		})
	}
}

func TestListIncidentsExplainsPortFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")

	tests := map[string]struct {
		err  error
		want string
	}{
		"not found": {
			err:  domain.ErrNotFound,
			want: "list_checks",
		},
		"unsupported": {
			err:  domain.ErrUnsupported,
			want: "only uptime checks",
		},
		"other": {
			err:  cause,
			want: "could not list incidents",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := tools.NewListIncidentsHandler(discardLogger(), &fakeIncidents{err: tt.err})

			_, _, err := h.Handle(t.Context(), nil, tools.ListIncidentsInput{Type: "ssl", ID: "1"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}
