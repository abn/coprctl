package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newProjectCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage Copr projects",
	}
	out.bind(cmd)

	cmd.AddCommand(
		newProjectListCmd(app, &out),
		newProjectGetCmd(app, &out),
	)
	return cmd
}

func newProjectListCmd(app *App, out *outFlags) *cobra.Command {
	var mine, all bool
	cmd := &cobra.Command{
		Use:   "list [OWNER]",
		Short: "List projects",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.Client()
			if err != nil {
				return err
			}
			owner := ""
			if len(args) == 1 {
				owner = args[0]
			}
			limit := 100
			if all {
				limit = 0
			}
			projects, err := c.ListProjects(cmd.Context(), owner, limit)
			if err != nil {
				return err
			}
			if out.format == "auto" || out.format == "table" || out.format == "plain" {
				t := render.NewTable("FULL NAME", "DESCRIPTION")
				for _, p := range projects {
					t.Add(p.FullName, truncate(p.Description, 40))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, projects)
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "only my projects")
	cmd.Flags().BoolVar(&all, "all", false, "paginate through all projects")
	return cmd
}

func newProjectGetCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get REF",
		Short: "Show a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Owner == "" {
				return fmt.Errorf("reference %q has no owner; use owner/project", args[0])
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			p, err := c.GetProject(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			if out.format == "auto" || out.format == "table" || out.format == "plain" {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("Full name", p.FullName)
				t.Add("Description", p.Description)
				t.Add("Devel mode", fmt.Sprintf("%v", p.DevelMode))
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, p)
		},
	}
	return cmd
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
