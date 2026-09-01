package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/abn/coprctl/internal/render"
	"github.com/abn/coprctl/internal/tui"
)

// newUICmd renders a project dashboard. On an interactive TTY it opens the
// Bubble Tea dashboard; off a TTY it degrades to a plain table so automation
// is never blocked.
func newUICmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "ui [REF]",
		Short: "Interactive project dashboard (degrades to plain output off a TTY)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			if ref == "" {
				return fmt.Errorf("a project reference is required")
			}
			r, err := parseRef(app, ref)
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			// Interactive TTY path: the Bubble Tea dashboard, unless the user
			// asked for machine output or the environment forbids a UI.
			if term.IsTerminal(int(os.Stdout.Fd())) && !isNoUI() && !cmd.Flags().Changed("output") {
				return tui.Run(cmd.Context(), c, r.Owner, r.Project, r.Dir)
			}
			// Degraded path: plain table.
			rows, err := c.Monitor(cmd.Context(), r.Owner, r.Project, r.Dir)
			if err != nil {
				return err
			}
			t := render.NewTable("PACKAGE", "CHROOT", "STATE", "BUILD", "VERSION")
			for _, row := range rows {
				for ch, info := range row.Chroots {
					t.Add(row.Name, ch, info.State, monitorBuildID(info), info.PkgVersion)
				}
			}
			return renderResult(cmd, &out, t)
		},
	}
	out.bind(cmd)
	return cmd
}

func isNoUI() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" || os.Getenv("CI") != ""
}
