package cli

import (
	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/render"
)

func newProjectSearchCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search projects by name or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			projects, err := c.SearchProjects(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("FULL NAME", "DESCRIPTION")
				for _, p := range projects {
					t.Add(p.FullName, truncate(p.Description, 40))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, projects)
		},
	}
	return cmd
}
