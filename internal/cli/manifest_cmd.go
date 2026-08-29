package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
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
	var dryRun, prune, yes bool
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
			if prune && !yes {
				return confirmRequired("--yes")
			}
			if err := applyManifest(cmd.Context(), app, m, prune); err != nil {
				return err
			}
			return renderResult(cmd, &out, map[string]any{"applied": m.Metadata.Owner + "/" + m.Metadata.Name})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change")
	cmd.Flags().BoolVar(&prune, "prune", false, "disable chroots absent from the manifest")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm --prune")
	out.bind(cmd)
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
			c, err := app.ReadClient()
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
			c, err := app.ReadClient()
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

// applyManifest creates or updates a project, chroots, and packages to match
// the manifest. Additive and safe to re-run after a partial failure. When
// prune is set, the enabled chroot set is set to exactly the manifest's list;
// otherwise chroots in the manifest are enabled additively alongside the live
// set.
func applyManifest(ctx context.Context, app *App, m *manifest.Manifest, prune bool) error {
	c, err := app.Client()
	if err != nil {
		return err
	}
	owner, project := m.Metadata.Owner, m.Metadata.Name
	_, gerr := c.GetProject(ctx, owner, project)
	exists := gerr == nil
	if exists {
		var devel *bool
		dv := m.Spec.Settings.DevelMode
		devel = &dv
		if err := c.EditProject(ctx, copr.ProjectEdit{
			Owner: owner, Project: project,
			Description: m.Spec.Description,
			Homepage:    m.Spec.Homepage,
			Contact:     m.Spec.Contact,
			DevelMode:   devel,
		}); err != nil {
			return err
		}
	} else {
		if err := c.CreateProject(ctx, copr.ProjectCreate{
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
	// Reconcile the enabled chroot set. Additive apply enables chroots listed
	// in the manifest that are not yet live; prune sets the set to exactly the
	// manifest's list, disabling anything the manifest no longer lists.
	if p, err := c.GetProject(ctx, owner, project); err == nil {
		want := map[string]bool{}
		for _, ch := range m.Spec.Chroots.Enabled {
			want[ch] = true
		}
		next := make([]string, 0, len(m.Spec.Chroots.Enabled))
		for _, ch := range m.Spec.Chroots.Enabled {
			next = append(next, ch)
		}
		if !prune {
			// Additive: keep chroots live that the manifest does not mention.
			for name := range p.ChrootRepos {
				if !want[name] {
					next = append(next, name)
				}
			}
		}
		sort.Strings(next)
		// Only call the API when the set actually differs from live.
		live := make([]string, 0, len(p.ChrootRepos))
		for name := range p.ChrootRepos {
			live = append(live, name)
		}
		sort.Strings(live)
		if !equalStrings(live, next) {
			if err := c.EditProject(ctx, copr.ProjectEdit{
				Owner: owner, Project: project, Chroots: &next,
			}); err != nil {
				return err
			}
		}
	}
	for ch, cfg := range m.Spec.Chroots.Config {
		if err := c.EditProjectChroot(ctx, copr.MockChrootEdit{
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
	for _, p := range m.Spec.Packages {
		src, st, err := packageToSource(p)
		if err != nil {
			return err
		}
		if err := c.UpsertPackage(ctx, copr.PackageCreate{
			Owner: owner, Project: project, Name: p.Name,
			SourceType: st, Source: src, AutoRebuild: p.AutoRebuild,
			SetAutoRebuild: true,
		}); err != nil {
			return err
		}
	}
	return nil
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

// equalStrings reports whether two slices contain the same strings, ignoring
// order.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			return false
		}
	}
	return true
}
