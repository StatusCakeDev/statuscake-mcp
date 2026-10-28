package tools_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

// The handlers depend on small ports, so the tests supply small fakes. Each
// fake records what the handler asked for and answers with what the test
// prepared, which is all a handler's behaviour consists of: translating a
// request into a domain query, and a domain answer into a view.

var epoch = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeChecks stands in for the adapter. It filters the way the adapter does and
// hands the matches to domain.NewCheckList, so tests exercise the real ordering
// and truncation policy.
type fakeChecks struct {
	checks []domain.Check
	err    error

	gotFilter domain.CheckFilter
	gotRef    domain.CheckRef
}

func (f *fakeChecks) ListChecks(_ context.Context, filter domain.CheckFilter) (domain.CheckList, error) {
	f.gotFilter = filter
	if f.err != nil {
		return domain.CheckList{}, f.err
	}

	var matches []domain.CheckSummary
	for _, c := range f.checks {
		if filter.Matches(c.CheckSummary) {
			matches = append(matches, c.CheckSummary)
		}
	}
	return domain.NewCheckList(matches, filter.Limit), nil
}

func (f *fakeChecks) GetCheck(_ context.Context, ref domain.CheckRef) (domain.Check, error) {
	f.gotRef = ref
	if f.err != nil {
		return domain.Check{}, f.err
	}
	for _, c := range f.checks {
		if c.CheckRef == ref {
			return c, nil
		}
	}
	return domain.Check{}, domain.ErrNotFound
}

type fakeHistory struct {
	result domain.CheckHistory
	err    error
	got    domain.CheckHistoryFilter
}

func (f *fakeHistory) ListCheckHistory(_ context.Context, filter domain.CheckHistoryFilter) (domain.CheckHistory, error) {
	f.got = filter
	return f.result, f.err
}

type fakeIncidents struct {
	result domain.IncidentList
	err    error
	got    domain.IncidentFilter
}

func (f *fakeIncidents) ListIncidents(_ context.Context, filter domain.IncidentFilter) (domain.IncidentList, error) {
	f.got = filter
	return f.result, f.err
}

type fakeContactGroups struct {
	groups []domain.ContactGroup
	err    error
	got    domain.ContactGroupFilter
}

func (f *fakeContactGroups) ListContactGroups(_ context.Context, filter domain.ContactGroupFilter) (domain.ContactGroupList, error) {
	f.got = filter
	if f.err != nil {
		return domain.ContactGroupList{}, f.err
	}
	return domain.NewContactGroupList(f.groups, filter.Limit), nil
}

type fakeWindows struct {
	windows []domain.MaintenanceWindow
	err     error
	got     domain.MaintenanceWindowFilter
}

func (f *fakeWindows) ListMaintenanceWindows(_ context.Context, filter domain.MaintenanceWindowFilter) (domain.MaintenanceWindowList, error) {
	f.got = filter
	if f.err != nil {
		return domain.MaintenanceWindowList{}, f.err
	}

	var matches []domain.MaintenanceWindow
	for _, w := range f.windows {
		if s, ok := filter.State.TryUnwrap(); ok && w.State != s {
			continue
		}
		matches = append(matches, w)
	}
	return domain.NewMaintenanceWindowList(matches, filter.Limit), nil
}

// fixture is a small account with one check of each type and one of each status
// that matters.
func fixture() *fakeChecks {
	return &fakeChecks{
		checks: []domain.Check{
			domain.NewCheck(
				domain.CheckSummary{
					CheckRef:      domain.CheckRef{ID: "101"},
					Name:          "Marketing site",
					Status:        domain.CheckStatusUp,
					Tags:          []string{"production", "web"},
					ContactGroups: []string{"1"},
				},
				domain.UptimeSettings{
					Target:   "https://example.com",
					Protocol: "HTTP",
					Interval: 5 * time.Minute,
				},
			),

			domain.NewCheck(
				domain.CheckSummary{
					CheckRef:      domain.CheckRef{ID: "102"},
					Name:          "Checkout API",
					Status:        domain.CheckStatusDown,
					Tags:          []string{"production", "payments"},
					ContactGroups: []string{"1"},
				},
				domain.UptimeSettings{
					Target:   "https://api.example.com/health",
					Protocol: "HTTP",
					Interval: time.Minute,
				},
			),

			domain.NewCheck(
				domain.CheckSummary{
					CheckRef:      domain.CheckRef{ID: "201"},
					Name:          "Homepage speed",
					Status:        domain.CheckStatusUnknown,
					ContactGroups: []string{"1"},
				},
				domain.PageSpeedSettings{
					Target:   "https://example.com",
					Region:   "UK",
					Interval: time.Hour,
				},
			),

			domain.NewCheck(
				domain.CheckSummary{
					CheckRef:      domain.CheckRef{ID: "301"},
					Name:          "example.com certificate",
					Status:        domain.CheckStatusUnknown,
					ContactGroups: []string{"1"},
				},
				domain.SSLSettings{
					Target:    "example.com",
					Interval:  24 * time.Hour,
					AlertDays: []int{30, 7, 1},
				},
			),

			domain.NewCheck(
				domain.CheckSummary{
					CheckRef:      domain.CheckRef{ID: "401"},
					Name:          "Nightly backup job",
					Status:        domain.CheckStatusPaused,
					ContactGroups: []string{"1"},
				},
				domain.HeartbeatSettings{
					Period: 25 * time.Hour,
				},
			),
		},
	}
}

// ---- a real MCP server and client, joined in memory ----------------------

// ports bundles the fakes behind every tool, so end-to-end tests can reach
// into whichever they need.
type ports struct {
	checks  *fakeChecks
	history *fakeHistory
	inc     *fakeIncidents
	groups  *fakeContactGroups
	windows *fakeWindows
}

func newPorts() *ports {
	return &ports{
		checks:  fixture(),
		history: &fakeHistory{},
		inc:     &fakeIncidents{},
		groups:  &fakeContactGroups{},
		windows: &fakeWindows{},
	}
}

// connect registers every tool on a real MCP server and returns a client
// session to it. That exercises what handler tests cannot: the JSON schemas the
// SDK infers from the structs, validation of arguments against them, and
// validation of the output against the output schema.
func connect(t *testing.T, p *ports) *mcp.ClientSession {
	t.Helper()

	logger := discardLogger()
	now := func() time.Time { return epoch }

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0"}, nil)
	tools.Register(server, tools.NewPingHandler(logger))
	tools.Register(server, tools.NewListChecksHandler(logger, p.checks))
	tools.Register(server, tools.NewGetCheckHandler(logger, p.checks))
	tools.Register(server, tools.NewListCheckHistoryHandler(logger, p.history))
	tools.Register(server, tools.NewListIncidentsHandler(logger, p.inc, tools.WithClock(now)))
	tools.Register(server, tools.NewListContactGroupsHandler(logger, p.groups))
	tools.Register(server, tools.NewListMaintenanceWindowsHandler(logger, p.windows))

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func call(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	res, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

// decode unmarshals the text of a successful result into v.
func decode(t *testing.T, res *mcp.CallToolResult, v any) {
	t.Helper()

	if res.IsError {
		t.Fatalf("unexpected tool error: %s", text(t, res))
	}
	if err := json.Unmarshal([]byte(text(t, res)), v); err != nil {
		t.Fatalf("decoding %q: %v", text(t, res), err)
	}
}

// cmpOptions teaches go-cmp to compare a gofp.Option, which holds only
// unexported fields but is comparable.
func cmpOptions() cmp.Option {
	return cmp.Options{
		cmpopts.EquateComparable(
			gofp.Option[domain.CheckType]{},
			gofp.Option[domain.CheckStatus]{},
			gofp.Option[domain.MaintenanceState]{},
			gofp.Option[string]{},
			gofp.Option[float64]{},
		),
		// Instants are equal whatever offset they were written with.
		cmp.Comparer(func(a, b gofp.Option[time.Time]) bool {
			at, aok := a.TryUnwrap()
			bt, bok := b.TryUnwrap()
			return aok == bok && at.Equal(bt)
		}),
		cmpopts.EquateEmpty(),
	}
}
