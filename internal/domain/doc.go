// Package domain holds the MCP's own view of what StatusCake reports: the read
// models that adapters return and that the server renders, and the errors by
// which adapters report what went wrong.
//
// It is deliberately small and owned by the MCP alone. The other StatusCake
// applications see the same API through models of their own, which lets each
// shape the data to its needs and keeps them from depending on one another.
package domain
