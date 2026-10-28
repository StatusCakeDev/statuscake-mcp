// Command statuscake-mcp is a local MCP server that speaks JSON-RPC over
// stdio. Stdout is the protocol channel and must carry nothing else, so all
// diagnostics go to stderr via slog.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/StatusCakeDev/statuscake-mcp/internal/cmd"
	cliruntime "github.com/tomasbasham/cli-runtime"
)

func main() {
	err := cliruntime.RunNoErrOutput(cmd.NewRootCommand())
	if err == nil {
		return
	}

	if errors.Is(err, cmd.ErrNotInteractive) {
		fmt.Fprintln(os.Stderr, cmd.PluginNotice)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(1)
}
