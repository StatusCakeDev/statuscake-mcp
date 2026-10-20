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

func TestListCheckHistoryTranslatesArgumentsIntoADomainFilter(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.ListCheckHistoryInput
		want domain.CheckHistoryFilter
	}{
		"no window": {
			in: tools.ListCheckHistoryInput{Type: "uptime", ID: "7"},
			want: domain.CheckHistoryFilter{
				CheckRef: domain.CheckRef{Type: domain.CheckTypeUptime, ID: "7"},
				Limit:    25,
			},
		},
		"a window, in any offset, becomes UTC-comparable instants": {
			in: tools.ListCheckHistoryInput{
				Type: "Uptime", ID: " 7 ",
				After:  "2026-10-01T10:00:00Z",
				Before: "2026-10-01T14:00:00+02:00",
				Limit:  500,
			},
			want: domain.CheckHistoryFilter{
				CheckRef: domain.CheckRef{Type: domain.CheckTypeUptime, ID: "7"},
				After:    gofp.Some(epoch.Add(-2 * time.Hour)),
				Before:   gofp.Some(epoch),
				Limit:    100,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeHistory{}
			h := tools.NewListCheckHistoryHandler(discardLogger(), fake)

			if _, _, err := h.Handle(t.Context(), nil, tt.in); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, fake.got, cmpOptions()); diff != "" {
				t.Errorf("filter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListCheckHistoryBuildsViews(t *testing.T) {
	t.Parallel()

	fake := &fakeHistory{result: domain.CheckHistory{
		Entries: []domain.CheckHistoryEntry{
			{
				At: epoch, Location: "LON",
				StatusCode:   gofp.Some(200),
				ResponseTime: gofp.Some(250 * time.Millisecond),
			},
			{At: epoch.Add(-time.Hour)}, // no answer at all
		},
		More: true,
	}}
	h := tools.NewListCheckHistoryHandler(discardLogger(), fake)

	_, got, err := h.Handle(t.Context(), nil, tools.ListCheckHistoryInput{Type: "uptime", ID: "7"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := tools.ListCheckHistoryOutput{
		Entries: []tools.CheckHistoryEntryView{
			{At: "2026-10-01T12:00:00Z", Location: "LON", StatusCode: 200, ResponseTimeMs: 250},
			{At: "2026-10-01T11:00:00Z"},
		},
		More: true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestListCheckHistoryRejectsBadArguments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.ListCheckHistoryInput
		want string
	}{
		"blank id":      {tools.ListCheckHistoryInput{Type: "uptime"}, "list_checks"},
		"unknown type":  {tools.ListCheckHistoryInput{Type: "dns", ID: "1"}, "uptime, pagespeed, ssl, heartbeat"},
		"bad before":    {tools.ListCheckHistoryInput{Type: "uptime", ID: "1", Before: "yesterday"}, "before: invalid time"},
		"bad after":     {tools.ListCheckHistoryInput{Type: "uptime", ID: "1", After: "2026-10-01"}, "after: invalid time"},
		"example given": {tools.ListCheckHistoryInput{Type: "uptime", ID: "1", After: "soon"}, "2026-10-01T09:30:00Z"},
		"empty window": {
			tools.ListCheckHistoryInput{Type: "uptime", ID: "1", After: "2026-10-02T00:00:00Z", Before: "2026-10-01T00:00:00Z"},
			"after must be earlier than before",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeHistory{}
			h := tools.NewListCheckHistoryHandler(discardLogger(), fake)

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

func TestListCheckHistoryExplainsPortFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")

	tests := map[string]struct {
		err  error
		want string
	}{
		"not found":   {domain.ErrNotFound, "list_checks"},
		"unsupported": {domain.ErrUnsupported, "only uptime checks"},
		"other":       {cause, "could not list check history"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := tools.NewListCheckHistoryHandler(discardLogger(), &fakeHistory{err: tt.err})

			_, _, err := h.Handle(t.Context(), nil, tools.ListCheckHistoryInput{Type: "ssl", ID: "1"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want one containing %q", err, tt.want)
			}
			if tt.err == cause && !errors.Is(err, cause) {
				t.Error("an unexpected failure should keep its cause")
			}
		})
	}
}
