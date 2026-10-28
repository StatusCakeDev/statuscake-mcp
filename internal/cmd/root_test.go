package cmd_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/tomasbasham/cli-runtime/iooption"

	"github.com/StatusCakeDev/statuscake-mcp/internal/cmd"
)

// These tests do not run in parallel, because the commands share the global
// state of viper.
func TestHelpIsReplacedByTheNotice(t *testing.T) {
	tests := map[string][]string{
		"long flag":  {"--help"},
		"short flag": {"-h"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			command := newCommand(&out, &errOut)
			command.SetArgs(args)

			if err := command.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := strings.TrimSpace(errOut.String()); got != cmd.PluginNotice {
				t.Errorf("stderr = %q, want %q", got, cmd.PluginNotice)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", out.String())
			}
		})
	}
}

func TestVersionIsStillReported(t *testing.T) {
	var out, errOut bytes.Buffer
	command := newCommand(&out, &errOut)
	command.SetArgs([]string{"--version"})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "dev") {
		t.Errorf("stdout = %q, want it to contain the version", out.String())
	}
}

func TestThereAreNoShellCompletions(t *testing.T) {
	command := newCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.InitDefaultCompletionCmd()

	for _, sub := range command.Commands() {
		if sub.Name() == "completion" {
			t.Error("the completion command exists")
		}
	}
}

func TestRefusesToServeATerminal(t *testing.T) {
	// A character device, as a terminal is, stands in for one.
	terminal, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = terminal.Close() })

	options := cmd.NewRootOptions(iooption.IOStreams{In: terminal, Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}})
	command := cmd.NewRootCommandWithArgs(options)
	command.SetArgs(nil)

	if err := command.Execute(); !errors.Is(err, cmd.ErrNotInteractive) {
		t.Errorf("Execute() error = %v, want %v", err, cmd.ErrNotInteractive)
	}
}

func newCommand(out, errOut *bytes.Buffer) *cobra.Command {
	options := cmd.NewRootOptions(iooption.IOStreams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	command := cmd.NewRootCommandWithArgs(options)
	command.SetOut(out)
	command.SetErr(errOut)
	return command
}
