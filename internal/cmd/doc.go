// Package cmd wires the application together and serves it over standard input
// and output, which is how an AI client runs the server once it has been
// registered with it. The root command creates the API adapter, registers the
// tools and runs until the client disconnects or is interrupted.
package cmd
