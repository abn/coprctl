package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/manifest"
	"github.com/abn/coprctl/internal/render"
)

func newApplyCmd(app *App) *cobra.Command {
	var out outFlags
	var file string
	var dryRun, prune bool
	cmd := &cobra.Command{
		Use:   "apply -f copr.yaml",
		Short: "Reconcile a project to match the manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadManifest(file)
			if err != nil {
				return err
			}
			if issues := m.Validate(); len(issues) > 0 {
				return cerr.Config("manifest failed validation").WithHint(formatIssues(issues))
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			if dryRun {
				diffs, err := m.DiffAgainst(cmd.Context(), c)
				if err != nil {
					return err
				}
				return renderResult(cmd, &out, map[string]any{"dry_run": true, "diffs": diffs})
			}
			// Create or update the project. Create tolerates an existing
			// project; updates apply the manifest's settings via edit.
			owner, project := m.Metadata.Owner, m.Metadata.Name
			_, gerr := c.GetProject(cmd.Context(), owner, project)
			exists := gerr == nil
			if exists {
				var devel *bool
				dv := m.Spec.Settings.DevelMode
				devel = &dv
				if err := c.EditProject(cmd.Context(), copr.ProjectEdit{
					Owner: owner, Project: project,
					Description: m.Spec.Description,
					Homepage:    m.Spec.Homepage,
					Contact:     m.Spec.Contact,
					DevelMode:   devel,
				}); err != nil {
					return err
				}
			} else {
				if err := c.CreateProject(cmd.Context(), copr.ProjectCreate{
					Owner: owner, Name: project,
					Chroots:     m.Spec.Chroots.Enabled,
					Description: m.Spec.Description,
					Homepage:    m.Spec.Homepage,
					Contact:     m.Spec.Contact,
					DevelMode:   m.Spec.Settings.DevelMode,
				}, true); err != nil {
					return err
				}
			}
			// Apply chroot config.
			for ch, cfg := range m.Spec.Chroots.Config {
				if err := c.EditProjectChroot(cmd.Context(), copr.MockChrootEdit{
					Owner: owner, Project: project, Chroot: ch,
					AdditionalPackages: cfg.AdditionalPackages,
					AdditionalRepos:    cfg.AdditionalRepos,
					AdditionalModules:  cfg.Modules,
					WithOpts:           cfg.With,
					WithoutOpts:        cfg.Without,
					Isolation:          cfg.Isolation,
				}); err != nil {
					return err
				}
			}
			// Create packages, tolerating existing ones (re-runnable).
			for _, p := range m.Spec.Packages {
				src, st, err := packageToSource(p)
				if err != nil {
					return err
				}
				if err := c.UpsertPackage(cmd.Context(), copr.PackageCreate{
					Owner: owner, Project: project, Name: p.Name,
					SourceType: st, Source: src, AutoRebuild: p.AutoRebuild,
				}); err != nil {
					return err
				}
			}
			return renderResult(cmd, &out, map[string]any{"applied": owner + "/" + project})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change")
	cmd.Flags().BoolVar(&prune, "prune", false, "remove state absent from the manifest")
	out.bind(cmd)
	_ = prune
	return cmd
}

func newExportCmd(app *App) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "export REF",
		Short: "Generate a manifest from a live project",
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
			m, err := manifest.ExportFromLive(cmd.Context(), c, r.Owner, r.Project)
			if err != nil {
				return err
			}
			data, err := m.MarshalYAML()
			if err != nil {
				return err
			}
			if file != "" {
				return os.WriteFile(file, data, 0o644)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "output", "o", "", "write manifest to file")
	return cmd
}

func newDiffCmd(app *App) *cobra.Command {
	var out outFlags
	var file string
	cmd := &cobra.Command{
		Use:   "diff -f copr.yaml",
		Short: "Show field-level drift between manifest and live project",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadManifest(file)
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			diffs, err := m.DiffAgainst(cmd.Context(), c)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("PATH", "MANIFEST", "LIVE")
				for _, d := range diffs {
					t.Add(d.Path, d.Manifest, d.Live)
				}
				if err := renderResult(cmd, &out, t); err != nil {
					return err
				}
			} else if err := renderResult(cmd, &out, diffs); err != nil {
				return err
			}
			if len(diffs) > 0 {
				return cerr.New("drift", cerr.ExitDrift, "project has drifted from the manifest")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file")
	out.bind(cmd)
	return cmd
}

func newValidateCmd(app *App) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "validate -f copr.yaml",
		Short: "Validate a manifest without any network calls",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadManifest(file)
			if err != nil {
				return err
			}
			issues := m.Validate()
			for _, i := range issues {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", i.Level, i.Path, i.Detail)
			}
			if len(issues) > 0 {
				return cerr.Config("manifest has validation issues")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "manifest valid")
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file")
	return cmd
}

func loadManifest(file string) (*manifest.Manifest, error) {
	if file == "" {
		return nil, cerr.Usage("--file is required")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, cerr.Config("cannot read manifest").Wrap(err)
	}
	return manifest.Parse(data)
}

func formatIssues(issues []manifest.ValidationIssue) string {
	var b strings.Builder
	for _, i := range issues {
		fmt.Fprintf(&b, "%s: %s; ", i.Path, i.Detail)
	}
	return b.String()
}

// packageToSource converts a manifest package source to the copr source form.
func packageToSource(p manifest.Package) (map[string]any, copr.SourceType, error) {
	m := map[string]any{}
	st := copr.SourceType(p.Source.Type)
	switch st {
	case copr.SourceSCM:
		m["clone_url"] = p.Source.CloneURL
		if p.Source.Committish != "" {
			m["committish"] = p.Source.Committish
		}
		if p.Source.Subdirectory != "" {
			m["subdirectory"] = p.Source.Subdirectory
		}
		if p.Source.Spec != "" {
			m["spec"] = p.Source.Spec
		}
	case copr.SourceDistGit:
		m["package_name"] = p.Name
		if p.Source.Committish != "" {
			m["committish"] = p.Source.Committish
		}
	default:
		return nil, st, fmt.Errorf("unsupported source type %q in manifest", p.Source.Type)
	}
	return m, st, nil
}
