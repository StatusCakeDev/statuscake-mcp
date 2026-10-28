// Package tools implements the MCP Tool interface for each of the tools in the
// application. Each tool is a handler that the AI assistant may invoke. It
// declares its own name, description and JSON schema, reads what it needs
// through an interface that it owns and that speaks only domain types, and
// returns a view of the result shaped for the assistant to read.
package tools
