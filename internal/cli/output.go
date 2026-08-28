package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/ref"
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
	// Color the human formats only on an interactive terminal, and never when
	// NO_COLOR is set or TERM is dumb.
	useColor := f == render.FormatTable || f == render.FormatPlain
	if useColor && (os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb") {
		useColor = false
	}
	if useColor && !isTTY(cmd) {
		useColor = false
	}
	return render.RenderColored(cmd.OutOrStdout(), f, v, useColor)
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

// parseRef parses a project reference requiring an owner.
func parseRef(s string) (ref.Ref, error) {
	r, err := ref.Parse(s, nil)
	if err != nil {
		return r, err
	}
	if r.Owner == "" {
		return r, cerr.Usage(fmt.Sprintf("reference %q has no owner; use owner/project", s))
	}
	return r, nil
}

// confirmRequired returns a usage error naming the confirmation flag that a
// non-interactive destructive command needs.
func confirmRequired(flag string) error {
	return cerr.Usage(fmt.Sprintf("this is a destructive operation; pass %s to confirm", flag))
}
