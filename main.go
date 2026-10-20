// Command statuscake-mcp is a local MCP server that speaks JSON-RPC over
// stdio. Stdout is the protocol channel and must carry nothing else, so all
// diagnostics go to stderr via slog.
package main

import (
	"os"

	"github.com/StatusCakeDev/statuscake-mcp/internal/cmd"
	cliruntime "github.com/tomasbasham/cli-runtime"
)

func main() {
	command := cmd.NewRootCommand()
	if code := cliruntime.Run(command); code != 0 {
		os.Exit(code)
	}
}
