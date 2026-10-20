package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var _ Tool[ListMaintenanceWindowsInput, ListMaintenanceWindowsOutput] = (*ListMaintenanceWindowsHandler)(nil)

// MaintenanceWindowLister is the port this tool needs: when alerts are
// deliberately suppressed.
type MaintenanceWindowLister interface {
	ListMaintenanceWindows(ctx context.Context, filter domain.MaintenanceWindowFilter) (domain.MaintenanceWindowList, error)
}

type ListMaintenanceWindowsInput struct {
	State string `json:"state,omitempty" jsonschema:"only windows in this state: active, pending or paused"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum windows to return; defaults to 25, capped at 100"`
}

type ListMaintenanceWindowsOutput struct {
	MaintenanceWindows []MaintenanceWindowView `json:"maintenance_windows" jsonschema:"active windows first, then pending, then paused"`
	Total              int                     `json:"total" jsonschema:"how many windows matched, before the limit was applied"`
	Truncated          bool                    `json:"truncated" jsonschema:"true when more windows matched than are listed; narrow the state filter to see the rest"`
}

type ListMaintenanceWindowsHandler struct {
	logger  *slog.Logger
	windows MaintenanceWindowLister
}

func NewListMaintenanceWindowsHandler(logger *slog.Logger, windows MaintenanceWindowLister) *ListMaintenanceWindowsHandler {
	return &ListMaintenanceWindowsHandler{
		logger:  logger,
		windows: windows,
	}
}

func (h *ListMaintenanceWindowsHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name: "list_maintenance_windows",
		Description: `List the maintenance windows during which StatusCake
		suppresses alerts. Use it to explain why a check that looks broken did not
		alert, or to see what is planned. Filter by state active to see what is
		suppressed right now.`,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *ListMaintenanceWindowsHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, in ListMaintenanceWindowsInput) (*mcp.CallToolResult, ListMaintenanceWindowsOutput, error) {
	state, err := optional(in.State, domain.ParseMaintenanceState)
	if err != nil {
		return nil, ListMaintenanceWindowsOutput{}, err
	}
	filter := domain.MaintenanceWindowFilter{State: state, Limit: clampLimit(in.Limit)}

	h.logger.InfoContext(ctx, "list_maintenance_windows invoked",
		"state", filter.State.String(),
		"limit", filter.Limit,
	)

	list, err := h.windows.ListMaintenanceWindows(ctx, filter)
	if err != nil {
		h.logger.ErrorContext(ctx, "list_maintenance_windows failed", "error", err)
		return nil, ListMaintenanceWindowsOutput{}, fmt.Errorf("could not list maintenance windows: %w", err)
	}

	out := ListMaintenanceWindowsOutput{
		MaintenanceWindows: make([]MaintenanceWindowView, len(list.Windows)),
		Total:              list.Total,
		Truncated:          list.Truncated(),
	}
	for i, w := range list.Windows {
		out.MaintenanceWindows[i] = toMaintenanceWindowView(w)
	}
	return nil, out, nil
}
