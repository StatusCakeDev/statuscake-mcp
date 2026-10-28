package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var _ Tool[ListCheckHistoryInput, ListCheckHistoryOutput] = (*ListCheckHistoryHandler)(nil)

// CheckHistoryLister is the port this tool needs: the tests a check has run.
type CheckHistoryLister interface {
	ListCheckHistory(ctx context.Context, filter domain.CheckHistoryFilter) (domain.CheckHistory, error)
}

type ListCheckHistoryInput struct {
	Type   string `json:"type" jsonschema:"the check's type, as returned by list_checks. Only uptime checks have history"`
	ID     string `json:"id" jsonschema:"the check's id, as returned by list_checks"`
	Before string `json:"before,omitempty" jsonschema:"only tests before this RFC 3339 time, such as 2026-10-01T09:30:00Z"`
	After  string `json:"after,omitempty" jsonschema:"only tests after this RFC 3339 time, such as 2026-10-01T09:30:00Z"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum tests to return; defaults to 25, capped at 100"`
}

type ListCheckHistoryOutput struct {
	Entries []CheckHistoryEntryView `json:"entries" jsonschema:"individual test executions, newest first"`
	More    bool                    `json:"more" jsonschema:"true when older tests exist beyond those listed; narrow the time window to reach them"`
}

type ListCheckHistoryHandler struct {
	logger  *slog.Logger
	history CheckHistoryLister
}

func NewListCheckHistoryHandler(logger *slog.Logger, history CheckHistoryLister) *ListCheckHistoryHandler {
	return &ListCheckHistoryHandler{
		logger:  logger,
		history: history,
	}
}

func (h *ListCheckHistoryHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name: "list_check_history",
		Description: `List the individual test executions an uptime check has run:
		when, from where, what the target answered and how quickly. Use it to
		investigate how a check has behaved over a window of time. For periods of
		downtime, use list_incidents instead. Both type and id are required and come
		from list_checks.`,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *ListCheckHistoryHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, in ListCheckHistoryInput) (*mcp.CallToolResult, ListCheckHistoryOutput, error) {
	filter, err := in.toFilter()
	if err != nil {
		return nil, ListCheckHistoryOutput{}, err
	}

	h.logger.InfoContext(ctx, "list_check_history invoked",
		"type", filter.Type.String(),
		"id", filter.ID,
		"before", filter.Before.String(),
		"after", filter.After.String(),
		"limit", filter.Limit,
	)

	history, err := h.history.ListCheckHistory(ctx, filter)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil, ListCheckHistoryOutput{}, fmt.Errorf(
			"no %s check with id %q exists; use list_checks to find valid ids", filter.Type, filter.ID)
	case errors.Is(err, domain.ErrUnsupported):
		return nil, ListCheckHistoryOutput{}, fmt.Errorf(
			"%s checks have no test history; only uptime checks do", filter.Type)
	case err != nil:
		h.logger.ErrorContext(ctx, "list_check_history failed", "error", err)
		return nil, ListCheckHistoryOutput{}, fmt.Errorf("could not list check history: %w", err)
	}

	out := ListCheckHistoryOutput{
		Entries: make([]CheckHistoryEntryView, len(history.Entries)),
		More:    history.More,
	}
	for i, e := range history.Entries {
		out.Entries[i] = toHistoryEntryView(e)
	}
	return nil, out, nil
}

func (in ListCheckHistoryInput) toFilter() (domain.CheckHistoryFilter, error) {
	ref, err := parseRef(in.Type, in.ID)
	if err != nil {
		return domain.CheckHistoryFilter{}, err
	}
	before, after, err := optionalWindow(in.Before, in.After)
	if err != nil {
		return domain.CheckHistoryFilter{}, err
	}

	return domain.CheckHistoryFilter{
		CheckRef: ref,
		Before:   before,
		After:    after,
		Limit:    clampLimit(in.Limit),
	}, nil
}
