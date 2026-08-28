package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/copr"
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
		newProjectCreateCmd(app, &out),
		newProjectEditCmd(app, &out),
		newProjectRegenCmd(app, &out),
		newProjectDeleteCmd(app, &out),
		newProjectForkCmd(app, &out),
		newProjectChrootCmd(app, &out),
	)
	return cmd
}

func newProjectEditCmd(app *App, out *outFlags) *cobra.Command {
	var description, homepage, contact, instructions, githubRepo string
	var develMode bool
	var develSet bool
	var enableNet bool
	var enableNetSet bool
	cmd := &cobra.Command{
		Use:   "edit REF [settings...]",
		Short: "Edit project settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			inst, err := resolveInstructions(instructions)
			if err != nil {
				return err
			}
			// Derive homepage and contact from a linked GitHub repo when the
			// user did not set them explicitly.
			if githubRepo != "" {
				repoHome, repoContact := githubRepoLinks(githubRepo)
				if homepage == "" {
					homepage = repoHome
				}
				if contact == "" {
					contact = repoContact
				}
			}
			var devel *bool
			if develSet {
				devel = &develMode
			}
			var enableNetPtr *bool
			if enableNetSet {
				enableNetPtr = &enableNet
			}
			if err := c.EditProject(cmd.Context(), copr.ProjectEdit{
				Owner: r.Owner, Project: r.Project,
				Description: description, Homepage: homepage, Contact: contact,
				Instructions: inst,
				DevelMode:    devel,
				EnableNet:    enableNetPtr,
			}); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"edited": r.String()})
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "project description")
	cmd.Flags().StringVar(&instructions, "instructions", "", "installation instructions (inline or a markdown file path)")
	cmd.Flags().StringVar(&homepage, "homepage", "", "project homepage")
	cmd.Flags().StringVar(&contact, "contact", "", "project contact")
	cmd.Flags().StringVar(&githubRepo, "github-repo", "", "linked GitHub repo OWNER/REPO; derives homepage and issues contact when unset")
	cmd.Flags().BoolVar(&develMode, "devel-mode", false, "enable devel mode")
	cmd.Flags().BoolVar(&develSet, "devel-mode-set", false, "set devel mode (use with --devel-mode)")
	cmd.Flags().BoolVar(&enableNet, "enable-net", false, "enable network access during builds (needed to fetch sources)")
	cmd.Flags().BoolVar(&enableNetSet, "enable-net-set", false, "set network access (use with --enable-net)")
	return cmd
}

// githubRepoLinks derives the homepage and issues contact for a GitHub repo.
func githubRepoLinks(repo string) (homepage, contact string) {
	home := "https://github.com/" + strings.TrimPrefix(repo, "https://github.com/")
	return strings.TrimSuffix(home, "/"), home + "/issues"
}

// resolveInstructions returns the installation instructions. If value names an
// existing file, its contents are read (so a markdown file can be passed);
// otherwise the value is used as inline text.
func resolveInstructions(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if fi, err := os.Stat(value); err == nil && !fi.IsDir() {
		data, err := os.ReadFile(value)
		if err != nil {
			return "", cerr.Config("cannot read instructions file").Wrap(err)
		}
		return string(data), nil
	}
	return value, nil
}

func newProjectRegenCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "regenerate-repos REF",
		Short: "Regenerate repository metadata for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			if err := c.RegenerateRepos(cmd.Context(), r.Owner, r.Project); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"regenerated": r.String()})
		},
	}
	return cmd
}

func newProjectListCmd(app *App, out *outFlags) *cobra.Command {
	var mine, all bool
	cmd := &cobra.Command{
		Use:   "list [OWNER]",
		Short: "List projects",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			owner := resolveListOwner(app, args)
			limit := 100
			if all {
				limit = 0
			}
			projects, err := c.ListProjects(cmd.Context(), owner, limit)
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
	cmd.Flags().BoolVar(&mine, "mine", false, "only my projects (default with no owner)")
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
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			p, err := c.GetProject(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("Full name", p.FullName)
				t.Add("Description", p.Description)
				t.Add("Devel mode", fmt.Sprintf("%v", p.DevelMode))
				t.Add("Enable net", fmt.Sprintf("%v", p.EnableNet))
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, p)
		},
	}
	bindRefCompletion(app, cmd)
	return cmd
}

func newProjectCreateCmd(app *App, out *outFlags) *cobra.Command {
	var chroots []string
	var description, instructions, homepage, contact, githubRepo string
	var ifNotExists, develMode, enableNet bool
	cmd := &cobra.Command{
		Use:   "create REF",
		Short: "Create a project",
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
			if ifNotExists {
				// Idempotent create: if the project already exists, report
				// success without creating it.
				if existing, gerr := c.GetProject(cmd.Context(), r.Owner, r.Project); gerr == nil && existing != nil {
					return renderResult(cmd, out, map[string]any{"created": r.String(), "existed": true})
				}
			}
			inst, err := resolveInstructions(instructions)
			if err != nil {
				return err
			}
			// Derive homepage and contact from a linked GitHub repo when the
			// user did not set them explicitly.
			if githubRepo != "" {
				repoHome, repoContact := githubRepoLinks(githubRepo)
				if homepage == "" {
					homepage = repoHome
				}
				if contact == "" {
					contact = repoContact
				}
			}
			err = c.CreateProject(cmd.Context(), copr.ProjectCreate{
				Owner:        r.Owner,
				Name:         r.Project,
				Chroots:      chroots,
				Description:  description,
				Instructions: inst,
				Homepage:     homepage,
				Contact:      contact,
				DevelMode:    develMode,
				EnableNet:    enableNet,
			}, ifNotExists)
			if err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{
				"created": r.String(),
			})
		},
	}
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to enable (repeatable)")
	bindChrootCompletion(app, cmd, "chroot")
	cmd.Flags().StringVar(&description, "description", "", "project description")
	cmd.Flags().StringVar(&instructions, "instructions", "", "installation instructions (inline or a markdown file path)")
	cmd.Flags().StringVar(&homepage, "homepage", "", "project homepage")
	cmd.Flags().StringVar(&contact, "contact", "", "project contact")
	cmd.Flags().StringVar(&githubRepo, "github-repo", "", "linked GitHub repo OWNER/REPO; derives homepage and issues contact when unset")
	cmd.Flags().BoolVar(&ifNotExists, "if-not-exists", false, "do not fail if the project exists")
	cmd.Flags().BoolVar(&develMode, "devel-mode", false, "enable devel mode")
	cmd.Flags().BoolVar(&enableNet, "enable-net", false, "enable network access during builds (needed to fetch sources)")
	return cmd
}

func newProjectDeleteCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete REF",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if !yes {
				return confirmRequired("--yes")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			if err := c.DeleteProject(cmd.Context(), r.Owner, r.Project); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"deleted": r.String()})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}

func newProjectForkCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "fork SRC DST",
		Short: "Fork a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			dst, err := ref.Parse(args[1], nil)
			if err != nil {
				return err
			}
			if !yes {
				return confirmRequired("--yes")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			if err := c.ForkProject(cmd.Context(), src.Owner, src.Project, dst.Owner, dst.Project); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"forked": src.String() + " -> " + dst.String()})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func isHuman(format string) bool {
	return format == "" || format == "auto" || format == "table" || format == "plain"
}

// resolveListOwner picks the owner for project list. An explicit argument
// wins; otherwise it defaults to the authenticated user's projects so the
// command does not page through the whole instance.
func resolveListOwner(app *App, args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	if app.Cfg != nil {
		if prof, err := app.Cfg.Profile(app.profile); err == nil && prof.Username != "" {
			return prof.Username
		}
	}
	return ""
}
