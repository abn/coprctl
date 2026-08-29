package cli

import (
	"context"
	"sort"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/chroot"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/render"
)

func newProjectChrootCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chroot",
		Short: "Manage project chroots",
	}
	cmd.AddCommand(
		newProjectChrootListCmd(app, out),
		newProjectChrootGetCmd(app, out),
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
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			p, err := c.GetProject(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(p.ChrootRepos))
			for name := range p.ChrootRepos {
				names = append(names, name)
			}
			sort.Strings(names)
			return renderHumanOr(cmd, out, p.ChrootRepos, func() *render.Table {
				t := render.NewTable("CHROOT", "STATE", "REPO")
				for _, name := range names {
					t.Add(name, string(chroot.Classify(name, true)), p.ChrootRepos[name])
				}
				return t
			})
		},
	}
	return cmd
}

// currentChroots returns the enabled chroot names of a project, sorted.
func currentChroots(c *copr.Client, ctx context.Context, owner, project string) ([]string, error) {
	p, err := c.GetProject(ctx, owner, project)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(p.ChrootRepos))
	for name := range p.ChrootRepos {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// setChroots replaces the enabled chroot set on a project.
func setChroots(c *copr.Client, ctx context.Context, owner, project string, chroots []string) error {
	return c.EditProject(ctx, copr.ProjectEdit{
		Owner: owner, Project: project, Chroots: &chroots,
	})
}

func newProjectChrootEnableCmd(app *App, out *outFlags) *cobra.Command {
	var chroots []string
	cmd := &cobra.Command{
		Use:   "enable REF --chroot CHROOT...",
		Short: "Enable chroots on a project (additive)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			if len(chroots) == 0 {
				return cerr.Usage("--chroot is required")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			existing, err := currentChroots(c, cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			seen := map[string]bool{}
			for _, name := range existing {
				seen[name] = true
			}
			next := existing
			for _, ch := range chroots {
				if !seen[ch] {
					next = append(next, ch)
				}
			}
			sort.Strings(next)
			if err := setChroots(c, cmd.Context(), r.Owner, r.Project, next); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"enabled": chroots, "chroots": next})
		},
	}
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to enable")
	bindChrootCompletion(app, cmd, "chroot")
	bindRefCompletion(app, cmd)
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
			r, err := parseRef(args[0])
			if err != nil {
				return err
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
			existing, err := currentChroots(c, cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			drop := map[string]bool{}
			for _, ch := range chroots {
				drop[ch] = true
			}
			remaining := make([]string, 0, len(existing))
			for _, name := range existing {
				if !drop[name] {
					remaining = append(remaining, name)
				}
			}
			if len(remaining) == len(existing) {
				return renderResult(cmd, out, map[string]any{"disabled": []string{}, "chroots": existing})
			}
			if err := setChroots(c, cmd.Context(), r.Owner, r.Project, remaining); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"disabled": chroots, "chroots": remaining})
		},
	}
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to disable")
	bindChrootCompletion(app, cmd, "chroot")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the disable")
	return cmd
}
