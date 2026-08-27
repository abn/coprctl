package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newMonitorCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "monitor REF",
		Short: "Show a package-by-chroot state matrix for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			// Monitor is a real /api_3 endpoint: package x chroot states.
			rows, err := c.Monitor(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("PACKAGE", "CHROOT", "STATE", "VERSION")
				for _, row := range rows {
					for ch, info := range row.Chroots {
						t.Add(row.Name, ch, info.State, info.PkgVersion)
					}
				}
				return renderResult(cmd, &out, t)
			}
			return renderResult(cmd, &out, rows)
		},
	}
	out.bind(cmd)
	return cmd
}

func newStatusCmd(app *App) *cobra.Command {
	var out outFlags
	var quiet bool
	cmd := &cobra.Command{
		Use:   "status REF",
		Short: "One-shot project health summary; exits 4 on failed builds",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			// Check the latest build of each package for failures.
			pkgs, err := c.ListPackages(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			unhealthy := 0
			summary := []map[string]any{}
			for _, p := range pkgs {
				builds, err := c.ListBuilds(cmd.Context(), r.Owner, r.Project, p.Name, 1)
				if err != nil {
					continue
				}
				if len(builds) == 0 {
					continue
				}
				state := builds[0].State
				if state == "failed" || state == "canceled" {
					unhealthy++
				}
				summary = append(summary, map[string]any{
					"package": p.Name, "state": state, "build_id": builds[0].ID,
				})
			}
			if !quiet {
				if isHuman(out.format) {
					t := render.NewTable("PACKAGE", "STATE", "BUILD")
					for _, s := range summary {
						t.Add(s["package"].(string), s["state"].(string), fmt.Sprintf("%v", s["build_id"]))
					}
					if err := renderResult(cmd, &out, t); err != nil {
						return err
					}
				} else {
					if err := renderResult(cmd, &out, summary); err != nil {
						return err
					}
				}
			}
			if unhealthy > 0 {
				return cerr.New("unhealthy", cerr.ExitBuildFailed, "one or more packages have failing builds")
			}
			return nil
		},
	}
	out.bind(cmd)
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress output, keep exit code")
	return cmd
}
