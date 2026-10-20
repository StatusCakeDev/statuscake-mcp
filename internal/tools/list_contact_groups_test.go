package tools_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

func TestListContactGroupsBuildsViewsAndReportsTruncation(t *testing.T) {
	t.Parallel()

	fake := &fakeContactGroups{groups: []domain.ContactGroup{
		{ID: "2", Name: "Ops", Emails: 3, Mobiles: 2, Integrations: 1},
		{ID: "1", Name: "Billing", Emails: 1},
		{ID: "3", Name: "Platform"},
	}}
	h := tools.NewListContactGroupsHandler(discardLogger(), fake)

	_, got, err := h.Handle(t.Context(), nil, tools.ListContactGroupsInput{Limit: 2})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := tools.ListContactGroupsOutput{
		ContactGroups: []tools.ContactGroupView{
			{ID: "1", Name: "Billing", Emails: 1},
			{ID: "2", Name: "Ops", Emails: 3, Mobiles: 2, Integrations: 1},
		},
		Total:     3,
		Truncated: true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
	if fake.got.Limit != 2 {
		t.Errorf("limit passed to the port = %d, want 2", fake.got.Limit)
	}
}

func TestListContactGroupsAppliesTheDefaultLimit(t *testing.T) {
	t.Parallel()

	fake := &fakeContactGroups{}
	h := tools.NewListContactGroupsHandler(discardLogger(), fake)

	_, got, err := h.Handle(t.Context(), nil, tools.ListContactGroupsInput{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if fake.got.Limit != 25 {
		t.Errorf("limit = %d, want 25", fake.got.Limit)
	}
	if got.ContactGroups == nil {
		t.Error("ContactGroups is nil, want an empty list that marshals as []")
	}
}

func TestListContactGroupsWrapsPortFailures(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	h := tools.NewListContactGroupsHandler(discardLogger(), &fakeContactGroups{err: cause})

	_, _, err := h.Handle(t.Context(), nil, tools.ListContactGroupsInput{})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "could not list contact groups") {
		t.Fatalf("error = %v, want a wrapped failure", err)
	}
}
