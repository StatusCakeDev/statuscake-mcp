package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool[In, Out any] interface {
	Definition() *mcp.Tool
	Handle(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error)
}

func Register[In, Out any](server *mcp.Server, t Tool[In, Out]) {
	mcp.AddTool(server, t.Definition(), t.Handle)
}
