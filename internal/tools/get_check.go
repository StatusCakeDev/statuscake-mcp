package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

var _ Tool[GetCheckInput, GetCheckOutput] = (*GetCheckHandler)(nil)

// CheckGetter is the port this tool needs: fetch one check, of whatever type,
// by reference. The result is a domain.Check whose Settings is a sum type, so
// the interface is not generic over SDK types and the handler never sees them.
type CheckGetter interface {
	GetCheck(ctx context.Context, ref domain.CheckRef) (domain.Check, error)
}

type GetCheckInput struct {
	Type string `json:"type" jsonschema:"the check's type, as returned by list_checks: uptime, pagespeed, ssl or heartbeat"`
	ID   string `json:"id" jsonschema:"the check's id, as returned by list_checks"`
}

// GetCheckOutput embeds the same summary a listing shows, so the model sees one
// consistent shape, and adds the settings that depend on the check's type.
type GetCheckOutput struct {
	CheckSummaryView

	Settings SettingsView `json:"settings" jsonschema:"configuration specific to the check's type. uptime: target, protocol, interval_seconds. pagespeed: target, region, interval_seconds. ssl: target, interval_seconds, alert_days. heartbeat: period_seconds"`
}

type GetCheckHandler struct {
	logger *slog.Logger
	checks CheckGetter
}

func NewGetCheckHandler(logger *slog.Logger, checks CheckGetter) *GetCheckHandler {
	return &GetCheckHandler{
		logger: logger,
		checks: checks,
	}
}

func (h *GetCheckHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name: "get_check",
		Description: `Get the configuration of one StatusCake check: its status,
		tags and the settings specific to its type. Both type and id are required
		and come from list_checks.`,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *GetCheckHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, in GetCheckInput) (*mcp.CallToolResult, GetCheckOutput, error) {
	ref, err := in.toRef()
	if err != nil {
		return nil, GetCheckOutput{}, err
	}

	h.logger.InfoContext(ctx, "get_check invoked",
		"type", ref.Type.String(),
		"id", ref.ID,
	)

	check, err := h.checks.GetCheck(ctx, ref)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil, GetCheckOutput{}, fmt.Errorf(
			"no %s check with id %q exists; use list_checks to find valid ids", ref.Type, ref.ID)
	case err != nil:
		h.logger.ErrorContext(ctx, "get_check failed", "error", err)
		return nil, GetCheckOutput{}, fmt.Errorf("could not get check: %w", err)
	}

	return nil, GetCheckOutput{
		CheckSummaryView: toSummaryView(check.CheckSummary),
		Settings:         toSettingsView(check.Settings),
	}, nil
}

func (in GetCheckInput) toRef() (domain.CheckRef, error) {
	return parseRef(in.Type, in.ID)
}
