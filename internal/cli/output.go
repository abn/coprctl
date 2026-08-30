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

// resolveRefOwner fills in an empty owner on a project-family ref from the
// current profile's username, so a bare project name means "the current user".
// Build refs have no owner and are left untouched.
func resolveRefOwner(app *App, r ref.Ref) ref.Ref {
	if r.Owner != "" {
		return r
	}
	switch r.Kind {
	case ref.KindProject, ref.KindPackage, ref.KindProjectChroot:
	default:
		return r
	}
	if app.Cfg != nil {
		if prof, err := app.Cfg.Profile(app.profile); err == nil && prof.Username != "" {
			r.Owner = prof.Username
		}
	}
	return r
}

// parseRef parses a project-family reference. A bare project name defaults to
// the current profile's username; build refs are passed through untouched.
func parseRef(app *App, s string) (ref.Ref, error) {
	r, err := ref.Parse(s, nil)
	if err != nil {
		return r, err
	}
	return resolveRefOwner(app, r), nil
}

// parseBuildRef parses a reference and requires it to be a build.
func parseBuildRef(args []string) (ref.Ref, error) {
	r, err := ref.Parse(args[0], nil)
	if err != nil {
		return r, err
	}
	if r.Kind != ref.KindBuild {
		return r, cerr.Usage(fmt.Sprintf("expected a build id, got %q", args[0]))
	}
	return r, nil
}

// parseBuildChrootRef parses a reference and requires it to be a build chroot.
func parseBuildChrootRef(args []string) (ref.Ref, error) {
	r, err := ref.Parse(args[0], nil)
	if err != nil {
		return r, err
	}
	if r.Kind != ref.KindBuildChroot {
		return r, cerr.Usage(fmt.Sprintf("expected a build/chroot reference, got %q", args[0]))
	}
	return r, nil
}

// parseBuildID parses a build reference and returns its integer id.
func parseBuildID(args []string) (int, error) {
	r, err := parseBuildRef(args)
	if err != nil {
		return 0, err
	}
	return r.BuildID, nil
}

// parseBuildIDs parses build references and returns their integer ids.
func parseBuildIDs(args []string) ([]int, error) {
	ids := make([]int, 0, len(args))
	for _, a := range args {
		id, err := parseBuildID([]string{a})
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePackageRef parses a reference requiring ForcePackage and defaults a bare
// owner to the current profile's username.
func parsePackageRef(app *App, args []string) (ref.Ref, error) {
	r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
	if err != nil {
		return r, err
	}
	return resolveRefOwner(app, r), nil
}

// confirmRequired returns a usage error naming the confirmation flag that a
// non-interactive destructive command needs.
func confirmRequired(flag string) error {
	return cerr.Usage(fmt.Sprintf("this is a destructive operation; pass %s to confirm", flag))
}
