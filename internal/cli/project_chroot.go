package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newProjectChrootCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chroot",
		Short: "Manage project chroots",
	}
	cmd.AddCommand(
		newProjectChrootListCmd(app, out),
		newProjectChrootEnableCmd(app, out),
		newProjectChrootDisableCmd(app, out),
	)
	return cmd
}

func newProjectChrootListCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list REF",
		Short: "List the chroots enabled on a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Owner == "" {
				return fmt.Errorf("reference %q has no owner; use owner/project", args[0])
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			p, err := c.GetProject(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("CHROOT", "REPO")
				for name, repo := range p.ChrootRepos {
					t.Add(name, repo)
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, p.ChrootRepos)
		},
	}
	return cmd
}

func newProjectChrootEnableCmd(app *App, out *outFlags) *cobra.Command {
	var chroots []string
	cmd := &cobra.Command{
		Use:   "enable REF --chroot CHROOT...",
		Short: "Enable chroots on a project (additive)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Owner == "" {
				return fmt.Errorf("reference %q has no owner; use owner/project", args[0])
			}
			if len(chroots) == 0 {
				return cerr.Usage("--chroot is required")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			p, err := c.GetProject(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			// Additive: union the existing set with the new chroots.
			existing := make([]string, 0, len(p.ChrootRepos)+len(chroots))
			for name := range p.ChrootRepos {
				existing = append(existing, name)
			}
			seen := map[string]bool{}
			for _, name := range existing {
				seen[name] = true
			}
			for _, ch := range chroots {
				if !seen[ch] {
					existing = append(existing, ch)
				}
			}
			if err := c.EditProjectChroots(cmd.Context(), r.Owner, r.Project, existing); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"enabled": chroots, "chroots": existing})
		},
	}
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to enable")
	return cmd
}

func newProjectChrootDisableCmd(app *App, out *outFlags) *cobra.Command {
	var chroots []string
	var yes bool
	cmd := &cobra.Command{
		Use:   "disable REF --chroot CHROOT...",
		Short: "Disable chroots on a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Owner == "" {
				return fmt.Errorf("reference %q has no owner; use owner/project", args[0])
			}
			if len(chroots) == 0 {
				return cerr.Usage("--chroot is required")
			}
			if !yes {
				return confirmRequired("--yes")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			p, err := c.GetProject(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			// Disabling a chroot stops new builds but keeps existing repos.
			warnInactiveChroots(cmd, chroots, app)
			remaining := make([]string, 0, len(p.ChrootRepos))
			for name := range p.ChrootRepos {
				remaining = append(remaining, name)
			}
			drop := map[string]bool{}
			for _, ch := range chroots {
				drop[ch] = true
			}
			out2 := remaining[:0]
			for _, name := range remaining {
				if !drop[name] {
					out2 = append(out2, name)
				}
			}
			if len(out2) == len(remaining) {
				return renderResult(cmd, out, map[string]any{"disabled": []string{}, "chroots": remaining})
			}
			if err := c.EditProjectChroots(cmd.Context(), r.Owner, r.Project, out2); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"disabled": chroots, "chroots": out2})
		},
	}
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to disable")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the disable")
	return cmd
}
