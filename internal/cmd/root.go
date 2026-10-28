package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	cliflag "github.com/tomasbasham/cli-runtime/flag"
	"github.com/tomasbasham/cli-runtime/iooption"
	"github.com/tomasbasham/cli-runtime/printer"

	"github.com/StatusCakeDev/statuscake-mcp/internal/adapter"
	"github.com/StatusCakeDev/statuscake-mcp/internal/cmd/flags"
	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

// PluginNotice is what the command says in place of its help text, and when it
// is run from a terminal. The server speaks to an MCP client over stdio, so
// there is nothing for a person to do with it directly.
const PluginNotice = `This binary is an MCP server. It is not meant to be executed directly.
Please configure the MCP client that consumes it, which will start it
automatically.`

// ErrNotInteractive reports that the server was started from a terminal rather
// than by an MCP client.
var ErrNotInteractive = errors.New("not started by an MCP client")

var (
	serverName = "statuscake"

	// Build information of the server, set at build time via ldflags.
	version = "dev"
	commit  = "none"
)

// nopWriteCloser is an io.WriteCloser with a trivial Close method.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

// RootOptions defines the options for the `statuscake-mcp` command.
type RootOptions struct {
	configFlags *flags.ConfigFlags

	logger *slog.Logger
	server *mcp.Server

	iooption.IOStreams
}

// NewRootOptions provides an initialised [RootOptions] instance.
func NewRootOptions(streams iooption.IOStreams) *RootOptions {
	logger := slog.New(slog.NewTextHandler(streams.ErrOut, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &RootOptions{
		configFlags: &flags.ConfigFlags{},
		logger:      logger,
		server: mcp.NewServer(
			&mcp.Implementation{Name: serverName, Version: version},
			&mcp.ServerOptions{Logger: logger},
		),
		IOStreams: streams,
	}
}

// NewRootCommand creates the `statuscake-mcp` command with default arguments.
func NewRootCommand() *cobra.Command {
	options := NewRootOptions(iooption.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})

	return NewRootCommandWithArgs(options)
}

// NewRootCommandWithArgs creates the `statuscake-mcp` command.
func NewRootCommandWithArgs(o *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "statuscake-mcp",
		Version:               versionInfo(),
		DisableFlagsInUseLine: true,
		Short:                 "StatusCake MCP server",
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.interactive() {
				return ErrNotInteractive
			}
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	// A person never runs the server by hand, so there is no help to give and
	// nothing to complete. The version is still reported.
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetHelpFunc(func(*cobra.Command, []string) { fmt.Fprintln(o.ErrOut, PluginNotice) })
	cmd.SetUsageFunc(func(*cobra.Command) error { return nil })

	printerOpts := printer.WarningPrinterOptions{Color: true}
	printer := printer.NewWarningPrinter(o.ErrOut, printerOpts)
	cmd.SetGlobalNormalizationFunc(cliflag.WarnWordSepNormalizeFunc(printer))

	pflags := cmd.PersistentFlags()
	pflags.String("config", "", "Config file (default is $HOME/.config/statuscake/mcp.yaml)")

	// Add root-specific flags.
	o.configFlags.AddFlags(pflags)

	// The globlal normalisation function ensures that all flags specified meet
	// the desired format, changing users' input if necessary.
	cmd.SetGlobalNormalizationFunc(cliflag.WordSepNormalizeFunc())

	return cmd
}

// interactive reports whether the server's input is a terminal, which means a
// person started it. An MCP client connects a pipe.
func (o *RootOptions) interactive() bool {
	in, ok := o.In.(*os.File)
	if !ok {
		return false
	}

	info, err := in.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (o *RootOptions) Complete(cmd *cobra.Command, args []string) error {
	return o.loadConfig(cmd)
}

func (o *RootOptions) Run() error {
	// The client terminates a stdio server by closing stdin or sending a signal.
	// Both cancel this context.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Version and Commit form part of the User-Agent header sent to StatusCake.
	// They will appear in our logs and help us identify the source of requests.
	client, err := adapter.New(o.logger, adapter.Config{
		Token:      o.configFlags.APIToken,
		RPS:        o.configFlags.RPS,
		Retries:    o.configFlags.Retries,
		MinBackoff: o.configFlags.MinBackoff,
		MaxBackoff: o.configFlags.MaxBackoff,
		Endpoint:   o.configFlags.Endpoint(),
		UserAgent:  userAgent(version, commit),
	})
	if err != nil {
		return fmt.Errorf("failed to create StatusCake client: %w", err)
	}

	// Register tools with the server.
	o.registerTools(client)

	o.logger.Info("starting MCP server", "name", serverName, "version", version, "transport", "stdio")

	// IOTransport reads JSON-RPC messages from stdin and writes responses to
	// stdout. Run blocks until the client disconnects or ctx is cancelled.
	transport := &mcp.IOTransport{
		Reader: io.NopCloser(o.In),
		Writer: nopWriteCloser{o.Out},
	}

	if err := o.server.Run(ctx, transport); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	o.logger.Info("MCP server stopped")
	return nil
}

func (o *RootOptions) loadConfig(cmd *cobra.Command) error {
	configFile, _ := cmd.Flags().GetString("config")

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("mcp")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("$HOME/.config/statuscake")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			viper.AutomaticEnv()
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := viper.Unmarshal(o.configFlags); err != nil {
		return fmt.Errorf("failed to unmarshal server config: %w", err)
	}

	return nil
}

// registerTools wires tool definitions onto the server, each built over the
// adapter that implements the port it consumes.
func (o *RootOptions) registerTools(client *adapter.Client) {
	tools.Register(o.server, tools.NewGetCheckHandler(o.logger, client))
	tools.Register(o.server, tools.NewListCheckHistoryHandler(o.logger, client))
	tools.Register(o.server, tools.NewListChecksHandler(o.logger, client))
	tools.Register(o.server, tools.NewListContactGroupsHandler(o.logger, client))
	tools.Register(o.server, tools.NewListIncidentsHandler(o.logger, client))
	tools.Register(o.server, tools.NewListMaintenanceWindowsHandler(o.logger, client))

	// The ping tool is a simple health check that can be used to verify that the
	// server is running and responsive.
	tools.Register(o.server, tools.NewPingHandler(o.logger))
}

func versionInfo() string {
	return fmt.Sprintf("%s (commit: %s)", version, commit)
}

func userAgent(version, commit string) string {
	if version == "" {
		version = "dev"
	}
	if commit != "" {
		version = fmt.Sprintf("%s (%s)", version, commit)
	}
	return fmt.Sprintf("statuscake-mcp/%s (%s; %s; %s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
