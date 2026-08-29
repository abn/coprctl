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

// resolveFormat returns the effective output format, resolving the default
// ("auto" or empty) to table on a TTY and json off one so piped output stays
// machine-readable.
func resolveFormat(cmd *cobra.Command, format string) string {
	if format == "" || format == "auto" {
		if isTTY(cmd) {
			return "table"
		}
		return "json"
	}
	return format
}

// isHuman reports whether the effective output format is human (table or
// plain). The auto default is human only on a TTY.
func isHuman(cmd *cobra.Command, format string) bool {
	f, err := render.ParseFormat(resolveFormat(cmd, format))
	if err != nil {
		return false
	}
	return f == render.FormatTable || f == render.FormatPlain
}

// renderResult writes v using the requested format.
func renderResult(cmd *cobra.Command, o *outFlags, v any) error {
	f, err := render.ParseFormat(resolveFormat(cmd, o.format))
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

// renderHumanOr renders a human table when the output format is human, else
// renders v machine-readably. build returns the table; it is only called for
// human output.
func renderHumanOr(cmd *cobra.Command, o *outFlags, v any, build func() *render.Table) error {
	if isHuman(cmd, o.format) {
		return renderResult(cmd, o, build())
	}
	return renderResult(cmd, o, v)
}

// renderTableRows renders rows as a human table, or v machine-readably.
func renderTableRows(cmd *cobra.Command, o *outFlags, headers []string, rows [][]string, v any) error {
	return renderHumanOr(cmd, o, v, func() *render.Table {
		t := render.NewTable(headers...)
		for _, row := range rows {
			t.Add(row...)
		}
		return t
	})
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
