package adapters_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-go"

	"github.com/StatusCakeDev/statuscake-mcp/internal/adapters"
	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

// The adapter must satisfy every port the tools consume.
var (
	_ tools.CheckLister             = (*adapters.Client)(nil)
	_ tools.CheckGetter             = (*adapters.Client)(nil)
	_ tools.CheckHistoryLister      = (*adapters.Client)(nil)
	_ tools.IncidentLister          = (*adapters.Client)(nil)
	_ tools.ContactGroupLister      = (*adapters.Client)(nil)
	_ tools.MaintenanceWindowLister = (*adapters.Client)(nil)
)

// cmpOptions teaches go-cmp to compare a gofp.Option, which holds only
// unexported fields but is comparable.
func cmpOptions() cmp.Option {
	return cmp.Options{
		cmpopts.EquateComparable(
			gofp.Option[int]{},
			gofp.Option[time.Duration]{},
			gofp.Option[time.Time]{},
		),
		cmpopts.EquateEmpty(),
	}
}

func ref(typ domain.CheckType, id string) domain.CheckRef {
	return domain.CheckRef{Type: typ, ID: id}
}

// serveEveryFamily declares a listing for each type of check.
func serveEveryFamily(api *fakeAPI) {
	api.serve("GET /v1/uptime", http.StatusOK, list(1,
		uptimeOverview("1", "shop", "up", false, "production"),
		uptimeOverview("2", "checkout", "down", false, "production", "payments"),
		uptimeOverview("3", "legacy", "up", true), // paused trumps up
	))
	api.serve("GET /v1/heartbeat", http.StatusOK, list(1,
		heartbeatOverview("4", "backup", "down", false, "production"),
	))
	api.serve("GET /v1/pagespeed", http.StatusOK, list(1,
		pagespeed("5", "homepage", false),
		pagespeed("6", "blog", true),
	))
	api.serve("GET /v1/ssl", http.StatusOK, list(1,
		ssl("7", "example.com", false),
	))
}

func TestListChecks(t *testing.T) {
	t.Parallel()

	summary := func(typ domain.CheckType, id, name string, status domain.CheckStatus, tags ...string) domain.CheckSummary {
		return domain.CheckSummary{CheckRef: ref(typ, id), Name: name, Status: status, Tags: tags}
	}

	var (
		checkout = summary(domain.CheckTypeUptime, "2", "checkout", domain.CheckStatusDown, "production", "payments")
		backup   = summary(domain.CheckTypeHeartbeat, "4", "backup", domain.CheckStatusDown, "production")
		blog     = summary(domain.CheckTypePagespeed, "6", "blog", domain.CheckStatusPaused)
		legacy   = summary(domain.CheckTypeUptime, "3", "legacy", domain.CheckStatusPaused)
		homepage = summary(domain.CheckTypePagespeed, "5", "homepage", domain.CheckStatusUnknown)
		shop     = summary(domain.CheckTypeUptime, "1", "shop", domain.CheckStatusUp, "production")
		cert     = summary(domain.CheckTypeSSL, "7", "https://example.com", domain.CheckStatusUnknown)
	)

	tests := map[string]struct {
		filter    domain.CheckFilter
		want      []domain.CheckSummary
		wantTotal int
		wantPaths []string
	}{
		"gathers every type and orders for triage": {
			want: []domain.CheckSummary{
				backup, checkout, // down
				blog, legacy, // paused
				homepage, cert, shop, // the rest, alphabetically
			},
			wantTotal: 7,
			wantPaths: []string{"/v1/heartbeat", "/v1/pagespeed", "/v1/ssl", "/v1/uptime"},
		},
		"a type filter only asks the API about that type": {
			filter:    domain.CheckFilter{Type: gofp.Some(domain.CheckTypeSSL)},
			want:      []domain.CheckSummary{cert},
			wantTotal: 1,
			wantPaths: []string{"/v1/ssl"},
		},
		"down is derived, so it finds heartbeat and uptime checks alike": {
			filter:    domain.CheckFilter{Status: gofp.Some(domain.CheckStatusDown)},
			want:      []domain.CheckSummary{backup, checkout},
			wantTotal: 2,
			wantPaths: []string{"/v1/heartbeat", "/v1/pagespeed", "/v1/ssl", "/v1/uptime"},
		},
		"paused covers types that report nothing but paused": {
			filter:    domain.CheckFilter{Status: gofp.Some(domain.CheckStatusPaused)},
			want:      []domain.CheckSummary{blog, legacy},
			wantTotal: 2,
			wantPaths: []string{"/v1/heartbeat", "/v1/pagespeed", "/v1/ssl", "/v1/uptime"},
		},
		"up does not claim health the API never reported": {
			filter:    domain.CheckFilter{Status: gofp.Some(domain.CheckStatusUp)},
			want:      []domain.CheckSummary{shop},
			wantTotal: 1,
			wantPaths: []string{"/v1/heartbeat", "/v1/pagespeed", "/v1/ssl", "/v1/uptime"},
		},
		"a tag filter excludes types that have no tags": {
			filter:    domain.CheckFilter{Tag: gofp.Some("production")},
			want:      []domain.CheckSummary{backup, checkout, shop},
			wantTotal: 3,
			wantPaths: []string{"/v1/heartbeat", "/v1/pagespeed", "/v1/ssl", "/v1/uptime"},
		},
		"filters combine": {
			filter: domain.CheckFilter{
				Type: gofp.Some(domain.CheckTypeUptime),
				Tag:  gofp.Some("payments"),
			},
			want:      []domain.CheckSummary{checkout},
			wantTotal: 1,
			wantPaths: []string{"/v1/uptime"},
		},
		"the limit keeps the most urgent and the total counts every match": {
			filter:    domain.CheckFilter{Limit: 3},
			want:      []domain.CheckSummary{backup, checkout, blog},
			wantTotal: 7,
			wantPaths: []string{"/v1/heartbeat", "/v1/pagespeed", "/v1/ssl", "/v1/uptime"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			api, client := newAPI(t)
			serveEveryFamily(api)

			got, err := client.ListChecks(t.Context(), tt.filter)
			if err != nil {
				t.Fatalf("ListChecks() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, got.Checks, cmpOptions()); diff != "" {
				t.Errorf("checks mismatch (-want +got):\n%s", diff)
			}
			if got.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", got.Total, tt.wantTotal)
			}
			if diff := cmp.Diff(tt.wantPaths, api.paths()); diff != "" {
				t.Errorf("requested paths mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListChecksReadsEveryPage(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serveFunc("GET /v1/uptime", func(r *http.Request) (int, any) {
		switch r.URL.Query().Get("page") {
		case "1":
			return http.StatusOK, list(3, uptimeOverview("1", "a", "up", false))
		case "2":
			return http.StatusOK, list(3, uptimeOverview("2", "b", "up", false))
		default:
			return http.StatusOK, list(3, uptimeOverview("3", "c", "down", false))
		}
	})

	got, err := client.ListChecks(t.Context(), domain.CheckFilter{Type: gofp.Some(domain.CheckTypeUptime)})
	if err != nil {
		t.Fatalf("ListChecks() error = %v", err)
	}

	var ids []string
	for _, c := range got.Checks {
		ids = append(ids, c.ID)
	}
	if diff := cmp.Diff([]string{"3", "1", "2"}, ids); diff != "" {
		t.Errorf("ids mismatch (-want +got):\n%s", diff)
	}

	var pages []string
	for _, r := range api.requests() {
		pages = append(pages, r.Query.Get("page"))
	}
	if diff := cmp.Diff([]string{"1", "2", "3"}, pages); diff != "" {
		t.Errorf("pages requested mismatch (-want +got):\n%s", diff)
	}
}

func TestListChecksAsksForWhatItNeeds(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime", http.StatusOK, list(1))

	if _, err := client.ListChecks(t.Context(), domain.CheckFilter{Type: gofp.Some(domain.CheckTypeUptime)}); err != nil {
		t.Fatalf("ListChecks() error = %v", err)
	}

	want := url.Values{
		"page":     {"1"},
		"limit":    {"100"}, // the API's maximum page size, not the caller's limit
		"nouptime": {"true"},
	}
	if diff := cmp.Diff(want, api.only(t).Query); diff != "" {
		t.Errorf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestListChecksFailsWholeRatherThanReturningPartialResults(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	serveEveryFamily(api)
	api.serve("GET /v1/ssl", http.StatusInternalServerError, map[string]any{"message": "boom"})

	got, err := client.ListChecks(t.Context(), domain.CheckFilter{})
	if err == nil {
		t.Fatalf("ListChecks() = %+v, want an error", got)
	}

	var apiErr statuscake.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusInternalServerError {
		t.Errorf("error = %v, want it to wrap an APIError with status 500", err)
	}
	if errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a server error must not be reported as not found: %v", err)
	}
	if len(got.Checks) != 0 {
		t.Errorf("got %d checks alongside the error, want none", len(got.Checks))
	}
}

func TestListChecksHonoursCancellation(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	serveEveryFamily(api)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := client.ListChecks(ctx, domain.CheckFilter{}); err == nil {
		t.Fatal("ListChecks() with a cancelled context succeeded, want an error")
	}
}

func TestGetCheck(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ref  domain.CheckRef
		path string
		body any
		want domain.Check
	}{
		"uptime": {
			ref:  ref(domain.CheckTypeUptime, "11"),
			path: "GET /v1/uptime/{id}",
			body: item(uptimeDetail("11")),
			want: domain.Check{
				CheckSummary: domain.CheckSummary{
					CheckRef: ref(domain.CheckTypeUptime, "11"),
					Name:     "Checkout API",
					Status:   domain.CheckStatusDown,
					Tags:     []string{"production", "payments"},
				},
				Settings: domain.UptimeSettings{
					Target:   "https://api.example.com/health?token=SECRET",
					Protocol: "HTTP",
					Interval: time.Minute,
				},
			},
		},
		"heartbeat omits the url that a job pings": {
			ref:  ref(domain.CheckTypeHeartbeat, "12"),
			path: "GET /v1/heartbeat/{id}",
			body: item(map[string]any{
				"id": "12", "name": "backup", "period": 90000, "paused": false,
				"status": "up", "tags": []string{"nightly"}, "contact_groups": []string{},
				"url": "https://heartbeat.statuscake.com/SECRET-TOKEN", "uptime": 100,
			}),
			want: domain.Check{
				CheckSummary: domain.CheckSummary{
					CheckRef: ref(domain.CheckTypeHeartbeat, "12"),
					Name:     "backup",
					Status:   domain.CheckStatusUp,
					Tags:     []string{"nightly"},
				},
				Settings: domain.HeartbeatSettings{Period: 25 * time.Hour},
			},
		},
		"pagespeed": {
			ref:  ref(domain.CheckTypePagespeed, "13"),
			path: "GET /v1/pagespeed/{id}",
			body: item(pagespeed("13", "homepage", false)),
			want: domain.Check{
				CheckSummary: domain.CheckSummary{
					CheckRef: ref(domain.CheckTypePagespeed, "13"),
					Name:     "homepage",
					Status:   domain.CheckStatusUnknown,
				},
				Settings: domain.PageSpeedSettings{
					Target:   "https://homepage.example.com",
					Region:   "UK",
					Interval: time.Hour,
				},
			},
		},
		"ssl": {
			ref:  ref(domain.CheckTypeSSL, "14"),
			path: "GET /v1/ssl/{id}",
			body: item(ssl("14", "example.com", true)),
			want: domain.Check{
				CheckSummary: domain.CheckSummary{
					CheckRef: ref(domain.CheckTypeSSL, "14"),
					Name:     "https://example.com",
					Status:   domain.CheckStatusPaused,
				},
				Settings: domain.SSLSettings{
					Target:    "https://example.com",
					Interval:  24 * time.Hour,
					AlertDays: []int{30, 7, 1},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			api, client := newAPI(t)
			api.serve(tt.path, http.StatusOK, tt.body)

			got, err := client.GetCheck(t.Context(), tt.ref)
			if err != nil {
				t.Fatalf("GetCheck() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, got, cmpOptions()); diff != "" {
				t.Errorf("check mismatch (-want +got):\n%s", diff)
			}
			if got.Type != tt.ref.Type {
				t.Errorf("Type = %q, want %q: a check must carry the type it was fetched as", got.Type, tt.ref.Type)
			}
		})
	}
}

func TestGetCheckRequestsTheRightResource(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime/{id}", http.StatusOK, item(uptimeDetail("a/b c")))

	if _, err := client.GetCheck(t.Context(), ref(domain.CheckTypeUptime, "a/b c")); err != nil {
		t.Fatalf("GetCheck() error = %v", err)
	}

	// The id is escaped, so it cannot be used to reach a different route.
	if got := api.only(t).Path; got != "/v1/uptime/a/b c" {
		t.Errorf("path = %q", got)
	}
}

func TestGetCheckReportsMissingChecksAsNotFound(t *testing.T) {
	t.Parallel()

	for _, typ := range []domain.CheckType{
		domain.CheckTypeUptime,
		domain.CheckTypeHeartbeat,
		domain.CheckTypePagespeed,
		domain.CheckTypeSSL,
	} {
		t.Run(typ.String(), func(t *testing.T) {
			t.Parallel()

			_, client := newAPI(t) // serves nothing, so everything is a 404

			_, err := client.GetCheck(t.Context(), ref(typ, "404"))
			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("error = %v, want one matching domain.ErrNotFound", err)
			}

			// The cause is kept for logs.
			var apiErr statuscake.APIError
			if !errors.As(err, &apiErr) {
				t.Errorf("error = %v, want it to wrap the SDK's APIError", err)
			}
		})
	}
}

func TestGetCheckDoesNotMistakeOtherFailuresForNotFound(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			t.Parallel()

			api, client := newAPI(t)
			api.serve("GET /v1/uptime/{id}", status, map[string]any{"message": "no"})

			_, err := client.GetCheck(t.Context(), ref(domain.CheckTypeUptime, "1"))
			if err == nil || errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("error = %v, want a failure that is not ErrNotFound", err)
			}
		})
	}
}

func TestGetCheckRejectsATypeItDoesNotKnow(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)

	_, err := client.GetCheck(t.Context(), ref("carrier-pigeon", "1"))
	if !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("error = %v, want one matching domain.ErrUnsupported", err)
	}
	if got := api.requests(); len(got) != 0 {
		t.Errorf("made %d requests, want none", len(got))
	}
}

var at = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestListCheckHistory(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime/{id}/history", http.StatusOK, unlinked(
		map[string]any{"created_at": "2026-10-01T11:00:00Z", "location": "LON", "performance": 250, "status_code": 200},
		map[string]any{"created_at": "2026-10-01T12:00:00Z", "location": "NYC", "performance": 0},
		map[string]any{"created_at": "2026-10-01T10:00:00Z"},
	))

	got, err := client.ListCheckHistory(t.Context(), domain.CheckHistoryFilter{
		CheckRef: ref(domain.CheckTypeUptime, "7"),
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("ListCheckHistory() error = %v", err)
	}

	want := domain.CheckHistory{Entries: []domain.CheckHistoryEntry{
		{At: at, Location: "NYC", ResponseTime: gofp.Some(time.Duration(0))},
		{
			At: at.Add(-time.Hour), Location: "LON",
			StatusCode:   gofp.Some(200),
			ResponseTime: gofp.Some(250 * time.Millisecond),
		},
		{At: at.Add(-2 * time.Hour)},
	}}
	if diff := cmp.Diff(want, got, cmpOptions()); diff != "" {
		t.Errorf("history mismatch (-want +got):\n%s", diff)
	}
}

func TestListCheckHistoryAsksForTheWindowAndOneSpare(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime/{id}/history", http.StatusOK, unlinked())

	_, err := client.ListCheckHistory(t.Context(), domain.CheckHistoryFilter{
		CheckRef: ref(domain.CheckTypeUptime, "7"),
		Before:   gofp.Some(at),
		After:    gofp.Some(at.Add(-24 * time.Hour)),
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListCheckHistory() error = %v", err)
	}

	want := url.Values{
		"limit":  {"11"}, // one spare, to learn whether there is more
		"before": {strconv.FormatInt(at.Unix(), 10)},
		"after":  {strconv.FormatInt(at.Add(-24*time.Hour).Unix(), 10)},
	}
	req := api.only(t)
	if diff := cmp.Diff(want, req.Query); diff != "" {
		t.Errorf("query mismatch (-want +got):\n%s", diff)
	}
	if req.Path != "/v1/uptime/7/history" {
		t.Errorf("path = %q", req.Path)
	}
}

func TestListCheckHistoryReportsWhenThereIsMore(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime/{id}/history", http.StatusOK, unlinked(
		map[string]any{"created_at": "2026-10-01T12:00:00Z"},
		map[string]any{"created_at": "2026-10-01T11:00:00Z"},
		map[string]any{"created_at": "2026-10-01T10:00:00Z"},
	))

	got, err := client.ListCheckHistory(t.Context(), domain.CheckHistoryFilter{
		CheckRef: ref(domain.CheckTypeUptime, "7"),
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("ListCheckHistory() error = %v", err)
	}

	if len(got.Entries) != 2 || !got.More {
		t.Errorf("got %d entries, More = %v; want 2 entries and More", len(got.Entries), got.More)
	}
}

func TestListCheckHistoryAndIncidentsAreOnlyForUptimeChecks(t *testing.T) {
	t.Parallel()

	for _, typ := range []domain.CheckType{
		domain.CheckTypeHeartbeat,
		domain.CheckTypePagespeed,
		domain.CheckTypeSSL,
	} {
		t.Run(typ.String(), func(t *testing.T) {
			t.Parallel()

			api, client := newAPI(t)

			_, err := client.ListCheckHistory(t.Context(), domain.CheckHistoryFilter{CheckRef: ref(typ, "1")})
			if !errors.Is(err, domain.ErrUnsupported) {
				t.Errorf("ListCheckHistory() error = %v, want ErrUnsupported", err)
			}

			_, err = client.ListIncidents(t.Context(), domain.IncidentFilter{CheckRef: ref(typ, "1")})
			if !errors.Is(err, domain.ErrUnsupported) {
				t.Errorf("ListIncidents() error = %v, want ErrUnsupported", err)
			}

			if got := api.requests(); len(got) != 0 {
				t.Errorf("made %d requests, want none", len(got))
			}
		})
	}
}

func TestListCheckHistoryAndIncidentsReportMissingChecksAsNotFound(t *testing.T) {
	t.Parallel()

	_, client := newAPI(t)
	uptime := ref(domain.CheckTypeUptime, "404")

	if _, err := client.ListCheckHistory(t.Context(), domain.CheckHistoryFilter{CheckRef: uptime}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ListCheckHistory() error = %v, want ErrNotFound", err)
	}
	if _, err := client.ListIncidents(t.Context(), domain.IncidentFilter{CheckRef: uptime}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ListIncidents() error = %v, want ErrNotFound", err)
	}
}

func TestListIncidents(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime/{id}/periods", http.StatusOK, unlinked(
		map[string]any{"created_at": "2026-10-01T08:00:00Z", "status": "up", "duration": 7200},
		map[string]any{"created_at": "2026-10-01T06:00:00Z", "ended_at": "2026-10-01T08:00:00Z", "status": "down", "duration": 7200},
		map[string]any{"created_at": "2026-10-01T12:00:00Z", "status": "down"}, // ongoing
		map[string]any{"created_at": "2026-10-01T05:00:00Z", "status": "up"},
	))

	got, err := client.ListIncidents(t.Context(), domain.IncidentFilter{
		CheckRef: ref(domain.CheckTypeUptime, "7"),
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListIncidents() error = %v", err)
	}

	want := domain.IncidentList{Incidents: []domain.Incident{
		{Started: at, Ended: gofp.None[time.Time]()},
		{Started: at.Add(-6 * time.Hour), Ended: gofp.Some(at.Add(-4 * time.Hour))},
	}}
	if diff := cmp.Diff(want, got, cmpOptions()); diff != "" {
		t.Errorf("incidents mismatch (-want +got):\n%s", diff)
	}
}

func TestListIncidentsPassesTheWindow(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/uptime/{id}/periods", http.StatusOK, unlinked())

	_, err := client.ListIncidents(t.Context(), domain.IncidentFilter{
		CheckRef: ref(domain.CheckTypeUptime, "7"),
		Before:   gofp.Some(at),
		After:    gofp.Some(at.Add(-24 * time.Hour)),
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListIncidents() error = %v", err)
	}

	want := url.Values{
		"limit":  {"100"},
		"before": {strconv.FormatInt(at.Unix(), 10)},
		"after":  {strconv.FormatInt(at.Add(-24*time.Hour).Unix(), 10)},
	}
	req := api.only(t)
	if diff := cmp.Diff(want, req.Query); diff != "" {
		t.Errorf("query mismatch (-want +got):\n%s", diff)
	}
	if req.Path != "/v1/uptime/7/periods" {
		t.Errorf("path = %q", req.Path)
	}
}

func TestListIncidentsReportsMoreWhenTheWindowWasNotReadInFull(t *testing.T) {
	t.Parallel()

	fullPage := make([]any, 100)
	for i := range fullPage {
		fullPage[i] = map[string]any{
			"created_at": at.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			"status":     "up",
		}
	}

	tests := map[string]struct {
		periods  []any
		limit    int
		wantMore bool
	}{
		"a full page of periods may have a successor": {periods: fullPage, limit: 10, wantMore: true},
		"a short page is everything":                  {periods: fullPage[:3], limit: 10},
		"more incidents than the limit": {
			periods: []any{
				map[string]any{"created_at": "2026-10-01T10:00:00Z", "status": "down"},
				map[string]any{"created_at": "2026-10-01T11:00:00Z", "status": "down"},
			},
			limit:    1,
			wantMore: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			api, client := newAPI(t)
			api.serve("GET /v1/uptime/{id}/periods", http.StatusOK, unlinked(tt.periods...))

			got, err := client.ListIncidents(t.Context(), domain.IncidentFilter{
				CheckRef: ref(domain.CheckTypeUptime, "7"),
				Limit:    tt.limit,
			})
			if err != nil {
				t.Fatalf("ListIncidents() error = %v", err)
			}
			if got.More != tt.wantMore {
				t.Errorf("More = %v, want %v", got.More, tt.wantMore)
			}
		})
	}
}

func TestListContactGroups(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serveFunc("GET /v1/contact-groups", func(r *http.Request) (int, any) {
		if r.URL.Query().Get("page") == "2" {
			return http.StatusOK, list(2, contactGroup("3", "Billing", 0, 1, 0))
		}
		return http.StatusOK, list(2,
			contactGroup("1", "Ops", 3, 2, 1),
			contactGroup("2", "Platform", 1, 0, 2),
		)
	})

	got, err := client.ListContactGroups(t.Context(), domain.ContactGroupFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListContactGroups() error = %v", err)
	}

	// Only the size of each kind of recipient survives: the domain type has
	// nowhere to put an address, a number or a hook URL.
	want := domain.ContactGroupList{
		Groups: []domain.ContactGroup{
			{ID: "3", Name: "Billing", Mobiles: 1},
			{ID: "1", Name: "Ops", Emails: 3, Mobiles: 2, Integrations: 1},
		},
		Total: 3,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("contact groups mismatch (-want +got):\n%s", diff)
	}
}

func TestListContactGroupsFailsOnAPIError(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/contact-groups", http.StatusUnauthorized, map[string]any{"message": "bad token"})

	if _, err := client.ListContactGroups(t.Context(), domain.ContactGroupFilter{}); err == nil {
		t.Fatal("ListContactGroups() succeeded, want an error")
	}
}

func TestListMaintenanceWindows(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/maintenance-windows", http.StatusOK, list(1,
		maintenanceWindow("2", "Later", "pending", "1w", nil, []string{"1", "2"}),
		maintenanceWindow("1", "Now", "active", "never", []string{"production"}, nil),
		maintenanceWindow("3", "Odd", "paused", "2w", nil, nil),
	))

	got, err := client.ListMaintenanceWindows(t.Context(), domain.MaintenanceWindowFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListMaintenanceWindows() error = %v", err)
	}

	start := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	want := domain.MaintenanceWindowList{
		Windows: []domain.MaintenanceWindow{
			{
				ID: "1", Name: "Now", State: domain.MaintenanceStateActive,
				Start: start, End: start.Add(2 * time.Hour),
				Repeat: "never", Timezone: "Europe/London", Tags: []string{"production"},
			},
			{
				ID: "2", Name: "Later", State: domain.MaintenanceStatePending,
				Start: start, End: start.Add(2 * time.Hour),
				Repeat: "weekly", Timezone: "Europe/London", Checks: 2,
			},
		},
		Total: 3,
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty(), cmpopts.EquateApproxTime(0)); diff != "" {
		t.Errorf("maintenance windows mismatch (-want +got):\n%s", diff)
	}
}

func TestListMaintenanceWindowsFiltersByStateAtTheSource(t *testing.T) {
	t.Parallel()

	api, client := newAPI(t)
	api.serve("GET /v1/maintenance-windows", http.StatusOK, list(1))

	_, err := client.ListMaintenanceWindows(t.Context(), domain.MaintenanceWindowFilter{
		State: gofp.Some(domain.MaintenanceStateActive),
	})
	if err != nil {
		t.Fatalf("ListMaintenanceWindows() error = %v", err)
	}

	want := url.Values{"page": {"1"}, "limit": {"100"}, "state": {"active"}}
	if diff := cmp.Diff(want, api.only(t).Query); diff != "" {
		t.Errorf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestRepeatIntervalsAreNamedForHumans(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"never": "never",
		"1d":    "daily",
		"1w":    "weekly",
		"2w":    "fortnightly",
		"1m":    "monthly",
	}

	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			api, client := newAPI(t)
			api.serve("GET /v1/maintenance-windows", http.StatusOK, list(1,
				maintenanceWindow("1", "w", "active", raw, nil, nil),
			))

			got, err := client.ListMaintenanceWindows(t.Context(), domain.MaintenanceWindowFilter{})
			if err != nil {
				t.Fatalf("ListMaintenanceWindows() error = %v", err)
			}
			if len(got.Windows) != 1 || got.Windows[0].Repeat != want {
				t.Errorf("Repeat = %v, want %q", got.Windows, want)
			}
		})
	}
}
