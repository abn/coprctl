package cli

import (
	"github.com/spf13/cobra"
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
			rows := make([][]string, 0, len(projects))
			for _, p := range projects {
				rows = append(rows, []string{p.FullName, truncate(p.Description, 40)})
			}
			return renderTableRows(cmd, out, []string{"FULL NAME", "DESCRIPTION"}, rows, projects)
		},
	}
	return cmd
}
