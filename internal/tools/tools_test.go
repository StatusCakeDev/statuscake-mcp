package tools_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

// These tests run the registered tools behind a real MCP server and client
// joined by an in-memory transport. They cover what handler tests cannot: the
// schemas the SDK infers from the input and output structs, validation of
// arguments against them, and validation of each output against its schema. A
// view that marshals to something its own schema rejects fails here, not in
// front of a model.

func TestEndToEndEveryToolIsListedWithSchemas(t *testing.T) {
	t.Parallel()

	s := connect(t, newPorts())

	res, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)

		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Errorf("%s: missing schema (input %v, output %v)", tool.Name, tool.InputSchema != nil, tool.OutputSchema != nil)
		}
		if strings.TrimSpace(tool.Description) == "" {
			t.Errorf("%s: no description, so a model cannot tell when to use it", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: not annotated read-only", tool.Name)
		}
	}

	slices.Sort(names)
	want := []string{
		"get_check",
		"list_check_history",
		"list_checks",
		"list_contact_groups",
		"list_incidents",
		"list_maintenance_windows",
		"ping",
	}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("tools mismatch (-want +got):\n%s", diff)
	}
}

func TestEndToEndListChecksShowsDownChecksFirst(t *testing.T) {
	t.Parallel()

	s := connect(t, newPorts())

	var out tools.ListChecksOutput
	decode(t, call(t, s, "list_checks", map[string]any{"limit": 3}), &out)

	if out.Total != 5 || len(out.Checks) != 3 || !out.Truncated {
		t.Fatalf("total=%d listed=%d truncated=%v; want 5, 3, true", out.Total, len(out.Checks), out.Truncated)
	}
	if out.Checks[0].Name != "Checkout API" || out.Checks[1].Name != "Nightly backup job" {
		t.Errorf("order = %q, %q; want the down check, then the paused one", out.Checks[0].Name, out.Checks[1].Name)
	}
}

func TestEndToEndGetCheckPassesOutputValidation(t *testing.T) {
	t.Parallel()

	s := connect(t, newPorts())

	// The settings field is an interface, so the SDK infers an unrestricted
	// schema for it. If that were wrong, the SDK would reject the output here.
	var out map[string]any
	decode(t, call(t, s, "get_check", map[string]any{"type": "ssl", "id": "301"}), &out)

	settings, ok := out["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings = %#v, want an object", out["settings"])
	}
	if settings["target"] != "example.com" {
		t.Errorf("settings.target = %v", settings["target"])
	}
	if out["type"] != "ssl" || out["id"] != "301" {
		t.Errorf("summary fields are not promoted alongside settings: %v", out)
	}
}

func TestEndToEndInvalidArgumentsComeBackAsToolErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		tool string
		args map[string]any
		want string
	}{
		"bad status enum lists the valid values": {
			tool: "list_checks",
			args: map[string]any{"status": "dwon"},
			want: "up, down, paused",
		},
		"unknown check points at list_checks": {
			tool: "get_check",
			args: map[string]any{"type": "uptime", "id": "999"},
			want: "list_checks",
		},
		"bad time shows the expected format": {
			tool: "list_incidents",
			args: map[string]any{"type": "uptime", "id": "1", "after": "last week"},
			want: "RFC 3339",
		},
		"bad maintenance state lists the valid values": {
			tool: "list_maintenance_windows",
			args: map[string]any{"state": "over"},
			want: "active, pending, paused",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := connect(t, newPorts())

			res := call(t, s, tt.tool, tt.args)
			if !res.IsError {
				t.Fatalf("expected IsError, got: %s", text(t, res))
			}
			if !strings.Contains(text(t, res), tt.want) {
				t.Errorf("error %q does not contain %q", text(t, res), tt.want)
			}
		})
	}
}

func TestEndToEndMissingRequiredArgumentIsRejectedBySchema(t *testing.T) {
	t.Parallel()

	s := connect(t, newPorts())

	// "id" is required by the inferred schema, so the SDK refuses this before the
	// handler runs.
	for _, tool := range []string{"get_check", "list_check_history", "list_incidents"} {
		res := call(t, s, tool, map[string]any{"type": "uptime"})
		if !res.IsError {
			t.Errorf("%s: expected IsError, got: %s", tool, text(t, res))
		}
	}
}

func TestEndToEndListCheckHistoryPassesOutputValidation(t *testing.T) {
	t.Parallel()

	p := newPorts()
	p.history.result = domain.CheckHistory{
		Entries: []domain.CheckHistoryEntry{
			{At: epoch, Location: "LON", StatusCode: gofp.Some(200), ResponseTime: gofp.Some(120 * time.Millisecond)},
			{At: epoch.Add(-time.Hour)}, // optional fields absent
		},
		More: true,
	}
	s := connect(t, p)

	var out tools.ListCheckHistoryOutput
	decode(t, call(t, s, "list_check_history", map[string]any{
		"type": "uptime", "id": "7", "after": "2026-10-01T00:00:00Z",
	}), &out)

	want := tools.ListCheckHistoryOutput{
		Entries: []tools.CheckHistoryEntryView{
			{At: "2026-10-01T12:00:00Z", Location: "LON", StatusCode: 200, ResponseTimeMs: 120},
			{At: "2026-10-01T11:00:00Z"},
		},
		More: true,
	}
	if diff := cmp.Diff(want, out); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
	if !p.history.got.After.IsSome() {
		t.Error("the after argument did not reach the port")
	}
}

func TestEndToEndListIncidentsPassesOutputValidation(t *testing.T) {
	t.Parallel()

	p := newPorts()
	p.inc.result = domain.IncidentList{Incidents: []domain.Incident{
		{Started: epoch.Add(-time.Hour)},
		{Started: epoch.Add(-4 * time.Hour), Ended: gofp.Some(epoch.Add(-3 * time.Hour))},
	}}
	s := connect(t, p)

	var out tools.ListIncidentsOutput
	decode(t, call(t, s, "list_incidents", map[string]any{"type": "uptime", "id": "7"}), &out)

	want := tools.ListIncidentsOutput{Incidents: []tools.IncidentView{
		{StartedAt: "2026-10-01T11:00:00Z", Ongoing: true, DurationSeconds: 3600},
		{StartedAt: "2026-10-01T08:00:00Z", EndedAt: "2026-10-01T09:00:00Z", DurationSeconds: 3600},
	}}
	if diff := cmp.Diff(want, out); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestEndToEndContactGroupsNeverExposeRecipients(t *testing.T) {
	t.Parallel()

	p := newPorts()
	p.groups.groups = []domain.ContactGroup{{ID: "1", Name: "Ops", Emails: 2, Mobiles: 1}}
	s := connect(t, p)

	res := call(t, s, "list_contact_groups", nil)

	var out tools.ListContactGroupsOutput
	decode(t, res, &out)

	if diff := cmp.Diff([]tools.ContactGroupView{{ID: "1", Name: "Ops", Emails: 2, Mobiles: 1}}, out.ContactGroups); diff != "" {
		t.Errorf("groups mismatch (-want +got):\n%s", diff)
	}
	for _, leaked := range []string{"@", "+44", "ping_url", "http"} {
		if strings.Contains(text(t, res), leaked) {
			t.Errorf("output contains %q: %s", leaked, text(t, res))
		}
	}
}

func TestEndToEndListMaintenanceWindowsPassesOutputValidation(t *testing.T) {
	t.Parallel()

	p := newPorts()
	p.windows.windows = windows()
	s := connect(t, p)

	var out tools.ListMaintenanceWindowsOutput
	decode(t, call(t, s, "list_maintenance_windows", map[string]any{"state": "active"}), &out)

	if out.Total != 1 || len(out.MaintenanceWindows) != 1 || out.MaintenanceWindows[0].Name != "Deploy" {
		t.Errorf("got %+v, want only the active window", out)
	}
}

func TestEndToEndPing(t *testing.T) {
	t.Parallel()

	s := connect(t, newPorts())

	var out tools.PingOutput
	decode(t, call(t, s, "ping", nil), &out)

	if out.Message != "pong" {
		t.Errorf("Message = %q", out.Message)
	}
}
