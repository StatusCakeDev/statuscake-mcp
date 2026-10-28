package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var _ Tool[ListContactGroupsInput, ListContactGroupsOutput] = (*ListContactGroupsHandler)(nil)

// ContactGroupLister is the port this tool needs: who is alerted.
type ContactGroupLister interface {
	ListContactGroups(ctx context.Context, filter domain.ContactGroupFilter) (domain.ContactGroupList, error)
}

type ListContactGroupsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum groups to return; defaults to 25, capped at 100"`
}

type ListContactGroupsOutput struct {
	ContactGroups []ContactGroupView `json:"contact_groups" jsonschema:"groups ordered by name"`
	Total         int                `json:"total" jsonschema:"how many groups exist, before the limit was applied"`
	Truncated     bool               `json:"truncated" jsonschema:"true when more groups exist than are listed"`
}

type ListContactGroupsHandler struct {
	logger *slog.Logger
	groups ContactGroupLister
}

func NewListContactGroupsHandler(logger *slog.Logger, groups ContactGroupLister) *ListContactGroupsHandler {
	return &ListContactGroupsHandler{
		logger: logger,
		groups: groups,
	}
}

func (h *ListContactGroupsHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name: "list_contact_groups",
		Description: `List the contact groups that StatusCake alerts when a check
		fails, with how many email addresses, mobile numbers and integrations each
		holds. Individual addresses and numbers are never shown.`,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *ListContactGroupsHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, in ListContactGroupsInput) (*mcp.CallToolResult, ListContactGroupsOutput, error) {
	filter := domain.ContactGroupFilter{Limit: clampLimit(in.Limit)}

	h.logger.InfoContext(ctx, "list_contact_groups invoked", "limit", filter.Limit)

	list, err := h.groups.ListContactGroups(ctx, filter)
	if err != nil {
		h.logger.ErrorContext(ctx, "list_contact_groups failed", "error", err)
		return nil, ListContactGroupsOutput{}, fmt.Errorf("could not list contact groups: %w", err)
	}

	out := ListContactGroupsOutput{
		ContactGroups: make([]ContactGroupView, len(list.Groups)),
		Total:         list.Total,
		Truncated:     list.Truncated(),
	}
	for i, g := range list.Groups {
		out.ContactGroups[i] = toContactGroupView(g)
	}
	return nil, out, nil
}
