package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/render"
)

// common output flag state.
type outFlags struct {
	format string
	fields string
}

func (o *outFlags) bind(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&o.format, "output", "o", "auto", "output format: table, plain, json, jsonl, yaml")
	cmd.PersistentFlags().StringVar(&o.fields, "fields", "", "comma-separated fields to show")
}

// renderResult writes v using the requested format.
func renderResult(cmd *cobra.Command, o *outFlags, v any) error {
	format := o.format
	if format == "" || format == "auto" {
		if isTTY(cmd) {
			format = "table"
		} else {
			format = "json"
		}
	}
	f, err := render.ParseFormat(format)
	if err != nil {
		return cerr.Usage(err.Error())
	}
	return render.Render(cmd.OutOrStdout(), f, v)
}

// isTTY reports whether stdout is an interactive terminal. Output defaults to
// json when not a TTY, which is the agent-friendly default.
func isTTY(cmd *cobra.Command) bool {
	f := cmd.OutOrStdout()
	if f != os.Stdout {
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func requireOne(args []string, usage string) (string, error) {
	if len(args) < 1 {
		return "", cerr.Usage(fmt.Sprintf("expected %s", usage))
	}
	return args[0], nil
}
