package tools_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

func TestListChecksTranslatesArgumentsIntoADomainFilter(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.ListChecksInput
		want domain.CheckFilter
	}{
		"nothing supplied leaves every filter absent": {
			want: domain.CheckFilter{Limit: 25},
		},
		"everything supplied is parsed and trimmed": {
			in: tools.ListChecksInput{Type: "UPTIME", Status: "down", Tag: " production ", Limit: 10},
			want: domain.CheckFilter{
				Type:   gofp.Some(domain.CheckTypeUptime),
				Status: gofp.Some(domain.CheckStatusDown),
				Tag:    gofp.Some("production"),
				Limit:  10,
			},
		},
		"blank strings are absent, not empty values": {
			in:   tools.ListChecksInput{Type: "  ", Status: "", Tag: "\t"},
			want: domain.CheckFilter{Limit: 25},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := fixture()
			h := tools.NewListChecksHandler(discardLogger(), fake)

			if _, _, err := h.Handle(t.Context(), nil, tt.in); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, fake.gotFilter, cmpOptions()); diff != "" {
				t.Errorf("filter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListChecksAppliesTheLimitPolicy(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ in, want int }{
		"unset gets the default":    {0, 25},
		"negative gets the default": {-5, 25},
		"in range is kept":          {10, 10},
		"the cap itself is kept":    {100, 100},
		"above the cap is capped":   {5000, 100},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := fixture()
			h := tools.NewListChecksHandler(discardLogger(), fake)

			if _, _, err := h.Handle(t.Context(), nil, tools.ListChecksInput{Limit: tt.in}); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if fake.gotFilter.Limit != tt.want {
				t.Errorf("limit %d became %d, want %d", tt.in, fake.gotFilter.Limit, tt.want)
			}
		})
	}
}

func TestListChecksReportsTruncationAndKeepsTheMostUrgent(t *testing.T) {
	t.Parallel()

	h := tools.NewListChecksHandler(discardLogger(), fixture())

	_, out, err := h.Handle(t.Context(), nil, tools.ListChecksInput{Limit: 2})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := tools.ListChecksOutput{
		Checks: []tools.CheckSummaryView{
			{ID: "102", Type: "uptime", Name: "Checkout API", Status: "down", Tags: []string{"production", "payments"}},
			{ID: "401", Type: "heartbeat", Name: "Nightly backup job", Status: "paused", Tags: []string{}},
		},
		Total:     5,
		Truncated: true,
	}
	if diff := cmp.Diff(want, out); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestListChecksIsNotTruncatedWhenEverythingFits(t *testing.T) {
	t.Parallel()

	h := tools.NewListChecksHandler(discardLogger(), fixture())

	_, out, err := h.Handle(t.Context(), nil, tools.ListChecksInput{Limit: 5})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if out.Truncated || out.Total != 5 || len(out.Checks) != 5 {
		t.Errorf("total=%d listed=%d truncated=%v; want 5, 5, false", out.Total, len(out.Checks), out.Truncated)
	}
}

func TestListChecksRejectsAnInvalidEnumWithTheValidValues(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   tools.ListChecksInput
		want string
	}{
		"status": {tools.ListChecksInput{Status: "dwon"}, "up, down, paused"},
		"type":   {tools.ListChecksInput{Type: "ping"}, "uptime, pagespeed, ssl, heartbeat"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := fixture()
			h := tools.NewListChecksHandler(discardLogger(), fake)

			_, _, err := h.Handle(t.Context(), nil, tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want one listing %q", err, tt.want)
			}
			if fake.gotFilter != (domain.CheckFilter{}) {
				t.Error("the port was called despite invalid arguments")
			}
		})
	}
}

func TestListChecksWrapsAdapterFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	fake := fixture()
	fake.err = cause
	h := tools.NewListChecksHandler(discardLogger(), fake)

	_, _, err := h.Handle(t.Context(), nil, tools.ListChecksInput{})
	if err == nil || !strings.Contains(err.Error(), "could not list checks") || !errors.Is(err, cause) {
		t.Fatalf("error = %v, want a wrapped adapter failure", err)
	}
}

func TestListChecksMarshalsEmptyTagsAsAnArrayAndOmitsAbsentTimes(t *testing.T) {
	t.Parallel()

	h := tools.NewListChecksHandler(discardLogger(), fixture())

	_, out, err := h.Handle(t.Context(), nil, tools.ListChecksInput{Type: "heartbeat"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	raw, err := json.Marshal(out.Checks[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "last_tested_at") {
		t.Errorf("an unset last_tested_at should be omitted: %s", raw)
	}
	if !strings.Contains(string(raw), `"tags":[]`) {
		t.Errorf("tags should marshal as an empty array: %s", raw)
	}
}

func TestListChecksDefinitionIsReadOnly(t *testing.T) {
	t.Parallel()

	def := tools.NewListChecksHandler(discardLogger(), fixture()).Definition()

	if def.Name != "list_checks" {
		t.Errorf("Name = %q", def.Name)
	}
	if def.Annotations == nil || !def.Annotations.ReadOnlyHint {
		t.Error("a listing tool must be annotated read-only")
	}
}
