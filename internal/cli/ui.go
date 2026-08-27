package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/render"
)

// newUICmd is a minimal full-view command. It renders a project's monitor
// matrix; when stdout is not a TTY it degrades to plain table output rather
// than entering an interactive view. The richer Bubble Tea dashboard is
// deferred; this command keeps the TUI-degradation invariant real.
func newUICmd(app *App) *cobra.Command {
	var out outFlags
	var ref string
	cmd := &cobra.Command{
		Use:   "ui [REF]",
		Short: "Project dashboard view (degrades to plain output off a TTY)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				ref = args[0]
			}
			if ref == "" {
				return fmt.Errorf("a project reference is required")
			}
			r, err := parseRef(ref)
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			rows, err := c.Monitor(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			t := render.NewTable("PACKAGE", "CHROOT", "STATE", "VERSION")
			for _, row := range rows {
				for ch, info := range row.Chroots {
					t.Add(row.Name, ch, info.State, info.PkgVersion)
				}
			}
			return renderResult(cmd, &out, t)
		},
	}
	out.bind(cmd)
	return cmd
}
