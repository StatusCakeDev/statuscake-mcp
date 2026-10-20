package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var _ Tool[PingInput, PingOutput] = (*PingHandler)(nil)

// PingInput and PingOutput double as the tool's JSON schema: the SDK infers the
// input and output schemas from these struct definitions.
type PingInput struct {
}

type PingOutput struct {
	Message string `json:"message" jsonschema:"fixed response confirming the server is alive"`
}

type PingHandler struct {
	logger *slog.Logger
}

func NewPingHandler(logger *slog.Logger) *PingHandler {
	return &PingHandler{
		logger: logger,
	}
}

func (h *PingHandler) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "ping",
		Description: "Check that the StatusCake MCP server is running. Returns a fixed response.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

func (h *PingHandler) Handle(ctx context.Context, _ *mcp.CallToolRequest, _ PingInput) (*mcp.CallToolResult, PingOutput, error) {
	h.logger.InfoContext(ctx, "ping invoked")
	return nil, PingOutput{Message: "pong"}, nil
}
