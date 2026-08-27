package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/copr"
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
		newBuildSubmitCmd(app, &out),
		newBuildCancelCmd(app, &out),
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
			if isHuman(out.format) {
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

func newBuildSubmitCmd(app *App, out *outFlags) *cobra.Command {
	var src sourceFlags
	var chroots []string
	var dir string
	cmd := &cobra.Command{
		Use:   "submit REF --source TYPE [flags]",
		Short: "Submit a build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			st, sm, err := src.sourceMap()
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			b, err := c.SubmitBuild(cmd.Context(), copr.BuildSubmit{
				Owner: r.Owner, Project: r.Project,
				SourceType: st, Source: sm, Chroots: chroots, Dir: dir,
			})
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("ID", fmt.Sprintf("%d", b.ID))
				t.Add("State", b.State)
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, b)
		},
	}
	src.bind(cmd)
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to build in (globs allowed)")
	cmd.Flags().StringVar(&dir, "dir", "", "side repo / project directory")
	return cmd
}

func newBuildCancelCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel BUILD_ID",
		Short: "Cancel a build",
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
			if err := c.CancelBuild(cmd.Context(), r.BuildID); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"canceled": r.BuildID})
		},
	}
	return cmd
}
