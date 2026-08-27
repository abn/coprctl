package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newBuildCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Manage Copr builds",
	}
	out.bind(cmd)
	cmd.AddCommand(
		newBuildGetCmd(app, &out),
		newBuildListCmd(app, &out),
	)
	return cmd
}

func newBuildGetCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get BUILD_ID",
		Short: "Show a build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Kind != ref.KindBuild {
				return fmt.Errorf("expected a build id, got %q", args[0])
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			b, err := c.GetBuild(cmd.Context(), r.BuildID)
			if err != nil {
				return err
			}
			if out.format == "auto" || out.format == "table" || out.format == "plain" {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("ID", fmt.Sprintf("%d", b.ID))
				t.Add("Project", b.OwnerName+"/"+b.ProjectName)
				t.Add("Package", b.PackageName)
				t.Add("State", b.State)
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, b)
		},
	}
	return cmd
}

func newBuildListCmd(app *App, out *outFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list REF",
		Short: "List builds for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			builds, err := c.ListBuilds(cmd.Context(), r.Owner, r.Project, "", limit)
			if err != nil {
				return err
			}
			if out.format == "auto" || out.format == "table" || out.format == "plain" {
				t := render.NewTable("ID", "PACKAGE", "STATE")
				for _, b := range builds {
					t.Add(fmt.Sprintf("%d", b.ID), b.PackageName, b.State)
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, builds)
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "number of builds to list")
	return cmd
}
