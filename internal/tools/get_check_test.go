package tools_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

func TestGetCheckBuildsTheSettingsViewForEveryType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.GetCheckInput
		want tools.GetCheckOutput
	}{
		"uptime": {
			in: tools.GetCheckInput{Type: "uptime", ID: "101"},
			want: tools.GetCheckOutput{
				CheckSummaryView: tools.CheckSummaryView{
					ID:            "101",
					Type:          "uptime",
					Name:          "Marketing site",
					Status:        "up",
					Tags:          []string{"production", "web"},
					ContactGroups: []string{"1"},
				},
				Settings: tools.UptimeSettingsView{
					Target:          "https://example.com",
					Protocol:        "HTTP",
					IntervalSeconds: 300,
				},
			},
		},
		"pagespeed": {
			in: tools.GetCheckInput{Type: "pagespeed", ID: "201"},
			want: tools.GetCheckOutput{
				CheckSummaryView: tools.CheckSummaryView{
					ID:            "201",
					Type:          "pagespeed",
					Name:          "Homepage speed",
					Status:        "unknown",
					Tags:          []string{},
					ContactGroups: []string{"1"},
				},
				Settings: tools.PageSpeedSettingsView{
					Target:          "https://example.com",
					Region:          "UK",
					IntervalSeconds: 3600,
				},
			},
		},
		"ssl": {
			in: tools.GetCheckInput{Type: "ssl", ID: "301"},
			want: tools.GetCheckOutput{
				CheckSummaryView: tools.CheckSummaryView{
					ID:            "301",
					Type:          "ssl",
					Name:          "example.com certificate",
					Status:        "unknown",
					Tags:          []string{},
					ContactGroups: []string{"1"},
				},
				Settings: tools.SSLSettingsView{
					Target:          "example.com",
					IntervalSeconds: 86400,
					AlertDays:       []int{30, 7, 1},
				},
			},
		},
		"heartbeat": {
			in: tools.GetCheckInput{Type: "heartbeat", ID: "401"},
			want: tools.GetCheckOutput{
				CheckSummaryView: tools.CheckSummaryView{
					ID:            "401",
					Type:          "heartbeat",
					Name:          "Nightly backup job",
					Status:        "paused",
					Tags:          []string{},
					ContactGroups: []string{"1"},
				},
				Settings: tools.HeartbeatSettingsView{
					PeriodSeconds: 90000,
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := tools.NewGetCheckHandler(discardLogger(), fixture())

			_, got, err := h.Handle(t.Context(), nil, tt.in)
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetCheckPassesAParsedReferenceToThePort(t *testing.T) {
	t.Parallel()

	fake := fixture()
	h := tools.NewGetCheckHandler(discardLogger(), fake)

	if _, _, err := h.Handle(t.Context(), nil, tools.GetCheckInput{Type: " SSL ", ID: " 301 "}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := domain.CheckRef{Type: domain.CheckTypeSSL, ID: "301"}
	if fake.gotRef != want {
		t.Errorf("ref = %+v, want %+v", fake.gotRef, want)
	}
}

func TestGetCheckExplainsHowToRecoverFromNotFound(t *testing.T) {
	t.Parallel()

	h := tools.NewGetCheckHandler(discardLogger(), fixture())

	_, _, err := h.Handle(t.Context(), nil, tools.GetCheckInput{Type: "uptime", ID: "999"})
	if err == nil || !strings.Contains(err.Error(), "list_checks") {
		t.Fatalf("error = %v, want one that points the model at list_checks", err)
	}
}

func TestGetCheckDistinguishesTypesThatShareAnID(t *testing.T) {
	t.Parallel()

	// "101" exists as an uptime check but not as an ssl check.
	h := tools.NewGetCheckHandler(discardLogger(), fixture())

	if _, _, err := h.Handle(t.Context(), nil, tools.GetCheckInput{Type: "ssl", ID: "101"}); err == nil {
		t.Fatal("Handle() succeeded: the id exists, but not for that type")
	}
}

func TestGetCheckRejectsBadArguments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.GetCheckInput
		want string
	}{
		"blank id": {
			in:   tools.GetCheckInput{Type: "uptime", ID: "  "},
			want: "list_checks",
		},
		"unknown type": {
			in:   tools.GetCheckInput{Type: "dns", ID: "1"},
			want: "uptime, pagespeed, ssl, heartbeat",
		},
		"no type": {
			in:   tools.GetCheckInput{ID: "1"},
			want: "invalid check type",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := fixture()
			h := tools.NewGetCheckHandler(discardLogger(), fake)

			_, _, err := h.Handle(t.Context(), nil, tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want one containing %q", err, tt.want)
			}
			if fake.gotRef != (domain.CheckRef{}) {
				t.Error("the port was called despite invalid arguments")
			}
		})
	}
}

func TestGetCheckWrapsAdapterFailuresWithoutCallingThemNotFound(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	fake := fixture()
	fake.err = cause
	h := tools.NewGetCheckHandler(discardLogger(), fake)

	_, _, err := h.Handle(t.Context(), nil, tools.GetCheckInput{Type: "uptime", ID: "101"})
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "list_checks") {
		t.Fatalf("error = %v, want the failure wrapped, not mistaken for a missing check", err)
	}
}
