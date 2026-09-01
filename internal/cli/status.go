package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/render"
)

func newMonitorCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "monitor REF",
		Short: "Show a package-by-chroot state matrix for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			// Monitor is a real /api_3 endpoint: package x chroot states.
			rows, err := c.Monitor(cmd.Context(), r.Owner, r.Project, r.Dir)
			if err != nil {
				return wrapGroupNotFoundHint(r.Owner, instanceBase(app), err)
			}
			trows := make([][]string, 0)
			for _, row := range rows {
				for ch, info := range row.Chroots {
					build := "-"
					if info.BuildID != 0 {
						build = fmt.Sprintf("%d", info.BuildID)
					}
					log := "-"
					if info.URLBuildLog != "" {
						log = elideLogURL(info.URLBuildLog)
					}
					trows = append(trows, []string{row.Name, ch, info.State, build, info.PkgVersion, log})
				}
			}
			return renderTableRows(cmd, &out, []string{"PACKAGE", "CHROOT", "STATE", "BUILD", "VERSION", "LOG"}, trows, rows)
		},
	}
	out.bind(cmd)
	return cmd
}

// elideLogURL shortens a live-log URL to its host and final two path segments
// (the build dir and log filename), so the human monitor table stays narrow
// and every chroot keeps its own signal; the full URL is the JSON output's
// job. Short URLs and degenerate inputs pass through unchanged.
func elideLogURL(u string) string {
	const maxLen = 48
	if len(u) <= maxLen {
		return u
	}
	head := u
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.Index(u[i+3:], "/"); j >= 0 {
			head = u[:i+3+j]
		}
	}
	segs := strings.Split(u, "/")
	nonEmpty := make([]string, 0, len(segs))
	for _, s := range segs {
		if s != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	tail := ""
	if n := len(nonEmpty); n >= 2 {
		tail = nonEmpty[n-2] + "/" + nonEmpty[n-1]
	} else if n == 1 {
		tail = nonEmpty[0]
	}
	// Elide only when the middle actually shrinks; a host-only URL or a
	// trailing slash would otherwise grow rather than shorten.
	if len(head)+len("/.../")+len(tail) < len(u) {
		return head + "/.../" + tail
	}
	return u
}

func newStatusCmd(app *App) *cobra.Command {
	var out outFlags
	var quiet bool
	cmd := &cobra.Command{
		Use:   "status REF",
		Short: "One-shot project health summary; exits 4 on failed builds",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
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
				// A total cap of 1 fetches only the latest build, in one request.
				builds, err := c.ListBuilds(cmd.Context(), r.Owner, r.Project, p.Name, 1)
				if err != nil {
					unhealthy++
					summary = append(summary, map[string]any{
						"package": p.Name, "state": "error", "error": err.Error(),
					})
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
				if err := renderHumanOr(cmd, &out, summary, func() *render.Table {
					t := render.NewTable("PACKAGE", "STATE", "BUILD")
					for _, s := range summary {
						build := "-"
						if bid, ok := s["build_id"]; ok {
							build = fmt.Sprintf("%v", bid)
						} else if msg, ok := s["error"]; ok {
							build = fmt.Sprintf("%v", msg)
						}
						t.Add(s["package"].(string), s["state"].(string), build)
					}
					return t
				}); err != nil {
					return err
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
