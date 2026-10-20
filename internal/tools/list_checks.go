package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var _ Tool[ListChecksInput, ListChecksOutput] = (*ListChecksHandler)(nil)

// CheckLister is the port this tool needs: list checks of every type. It is
// defined here, where it is consumed, and speaks only domain types. The adapter
// that implements it hides how many SDK calls that takes.
type CheckLister interface {
	ListChecks(ctx context.Context, filter domain.CheckFilter) (domain.CheckList, error)
}

// ListChecksInput and ListChecksOutput double as the tool's JSON schema: the
// SDK infers the input and output schemas from these struct definitions.
//
// Every field is a plain Go value, not a gofp.Option. The SDK validates the
// arguments against a schema it infers by reflection, and it infers an object
// with no properties for an Option, which would reject the string a model
// sends. The empty string means "not supplied".
type ListChecksInput struct {
	Type   string `json:"type,omitempty" jsonschema:"only checks of this type: uptime, pagespeed, ssl or heartbeat"`
	Status string `json:"status,omitempty" jsonschema:"only checks in this state: up, down or paused"`
	Tag    string `json:"tag,omitempty" jsonschema:"only checks carrying this tag"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum checks to return; defaults to 25, capped at 100"`
}

type ListChecksOutput struct {
	Checks    []CheckSummaryView `json:"checks" jsonschema:"matching checks, down first, then paused, then up"`
	Total     int                `json:"total" jsonschema:"how many checks matched the filters, before the limit was applied"`
	Truncated bool               `json:"truncated" jsonschema:"true when more checks matched than are listed; narrow the filters to see the rest"`
}

type ListChecksHandler struct {
	logger *slog.Logger
	checks CheckLister
}

func NewListChecksHandler(logger *slog.Logger, checks CheckLister) *ListChecksHandler {
	return &ListChecksHandler{
		logger: logger,
		checks: checks,
	}
}

func (h *ListChecksHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name: "list_checks",
		Description: `List StatusCake checks of every type (uptime, pagespeed, ssl,
		heartbeat). Use it to answer "what is down?" or "what do we monitor?", and
		to find the type and id of a check before calling get_check. Results put
		down checks first. If truncated is true, narrow the filters instead of
		asking for more.`,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *ListChecksHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, in ListChecksInput) (*mcp.CallToolResult, ListChecksOutput, error) {
	filter, err := in.toFilter()
	if err != nil {
		return nil, ListChecksOutput{}, err
	}

	h.logger.InfoContext(ctx, "list_checks invoked",
		"type", filter.Type.String(),
		"status", filter.Status.String(),
		"tag", filter.Tag.String(),
		"limit", filter.Limit,
	)

	list, err := h.checks.ListChecks(ctx, filter)
	if err != nil {
		h.logger.ErrorContext(ctx, "list_checks failed", "error", err)
		return nil, ListChecksOutput{}, fmt.Errorf("could not list checks: %w", err)
	}

	out := ListChecksOutput{
		Checks:    make([]CheckSummaryView, len(list.Checks)),
		Total:     list.Total,
		Truncated: list.Truncated(),
	}
	for i, c := range list.Checks {
		out.Checks[i] = toSummaryView(c)
	}
	return nil, out, nil
}

// toFilter is the boundary where plain tool arguments become domain values.
// Invalid enumerations are rejected here, with a message that lists the valid
// values, since the inferred schema cannot express them.
func (in ListChecksInput) toFilter() (domain.CheckFilter, error) {
	typ, err := optional(in.Type, domain.ParseCheckType)
	if err != nil {
		return domain.CheckFilter{}, err
	}
	status, err := optional(in.Status, domain.ParseCheckStatus)
	if err != nil {
		return domain.CheckFilter{}, err
	}

	return domain.CheckFilter{
		Type:   typ,
		Status: status,
		Tag:    optionalString(in.Tag),
		Limit:  clampLimit(in.Limit),
	}, nil
}
