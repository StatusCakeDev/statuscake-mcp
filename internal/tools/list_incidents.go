package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var _ Tool[ListIncidentsInput, ListIncidentsOutput] = (*ListIncidentsHandler)(nil)

// IncidentLister is the port this tool needs: the periods a check was down.
type IncidentLister interface {
	ListIncidents(ctx context.Context, filter domain.IncidentFilter) (domain.IncidentList, error)
}

type ListIncidentsInput struct {
	Type   string `json:"type" jsonschema:"the check's type, as returned by list_checks. only uptime checks have incidents"`
	ID     string `json:"id" jsonschema:"the check's id, as returned by list_checks"`
	Before string `json:"before,omitempty" jsonschema:"only incidents before this RFC 3339 time, such as 2026-10-01T09:30:00Z"`
	After  string `json:"after,omitempty" jsonschema:"only incidents after this RFC 3339 time, such as 2026-10-01T09:30:00Z"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum incidents to return; defaults to 25, capped at 100"`
}

type ListIncidentsOutput struct {
	Incidents []IncidentView `json:"incidents" jsonschema:"periods of downtime, newest first"`
	More      bool           `json:"more" jsonschema:"true when older incidents may exist beyond those listed; narrow the time window to reach them"`
}

type ListIncidentsHandler struct {
	logger    *slog.Logger
	incidents IncidentLister
	now       func() time.Time
}

// IncidentsOption customises a [ListIncidentsHandler].
type IncidentsOption func(*ListIncidentsHandler)

// WithClock replaces the source of the current time, which is used to say how
// long an ongoing incident has lasted so far.
func WithClock(now func() time.Time) IncidentsOption {
	return func(h *ListIncidentsHandler) { h.now = now }
}

func NewListIncidentsHandler(logger *slog.Logger, incidents IncidentLister, opts ...IncidentsOption) *ListIncidentsHandler {
	h := &ListIncidentsHandler{
		logger:    logger,
		incidents: incidents,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *ListIncidentsHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name: "list_incidents",
		Description: `List the periods during which an uptime check was down: when
		each began, when it ended, and how long it lasted. Use it to answer "when
		did this last go down?" or "how often does it fail?". For individual test
		executions, use list_check_history instead. Both type and id are required
		and come from list_checks.`,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *ListIncidentsHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, in ListIncidentsInput) (*mcp.CallToolResult, ListIncidentsOutput, error) {
	filter, err := in.toFilter()
	if err != nil {
		return nil, ListIncidentsOutput{}, err
	}

	h.logger.InfoContext(ctx, "list_incidents invoked",
		"type", filter.Type.String(),
		"id", filter.ID,
		"before", filter.Before.String(),
		"after", filter.After.String(),
		"limit", filter.Limit,
	)

	list, err := h.incidents.ListIncidents(ctx, filter)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil, ListIncidentsOutput{}, fmt.Errorf(
			"no %s check with id %q exists; use list_checks to find valid ids", filter.Type, filter.ID)
	case errors.Is(err, domain.ErrUnsupported):
		return nil, ListIncidentsOutput{}, fmt.Errorf(
			"%s checks have no incidents; only uptime checks do", filter.Type)
	case err != nil:
		h.logger.ErrorContext(ctx, "list_incidents failed", "error", err)
		return nil, ListIncidentsOutput{}, fmt.Errorf("could not list incidents: %w", err)
	}

	now := h.now()
	out := ListIncidentsOutput{
		Incidents: make([]IncidentView, len(list.Incidents)),
		More:      list.More,
	}
	for i, inc := range list.Incidents {
		out.Incidents[i] = toIncidentView(inc, now)
	}
	return nil, out, nil
}

func (in ListIncidentsInput) toFilter() (domain.IncidentFilter, error) {
	ref, err := parseRef(in.Type, in.ID)
	if err != nil {
		return domain.IncidentFilter{}, err
	}
	before, after, err := optionalWindow(in.Before, in.After)
	if err != nil {
		return domain.IncidentFilter{}, err
	}

	return domain.IncidentFilter{
		CheckRef: ref,
		Before:   before,
		After:    after,
		Limit:    clampLimit(in.Limit),
	}, nil
}
