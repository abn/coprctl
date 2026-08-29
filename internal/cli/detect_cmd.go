package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/detect"
	"github.com/abn/coprctl/internal/manifest"
)

func newDetectCmd(app *App) *cobra.Command {
	var out outFlags
	var path string
	cmd := &cobra.Command{
		Use:   "detect [PATH]",
		Short: "Read-only: infer a project setup from a source repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := "."
			if len(args) == 1 {
				p = args[0]
			}
			res, err := detect.Detect(p, true)
			if err != nil {
				return err
			}
			return renderResult(cmd, &out, res)
		},
	}
	out.bind(cmd)
	cmd.Flags().StringVar(&path, "path", "", "path to the source repository")
	return cmd
}

func newInitCmd(app *App) *cobra.Command {
	var out outFlags
	var path, owner, name string
	var chroots []string
	var yes bool
	cmd := &cobra.Command{
		Use:   "init [PATH]",
		Short: "Scaffold a manifest and create a working Copr project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := "."
			if len(args) == 1 {
				p = args[0]
			}
			res, err := detect.Detect(p, true)
			if err != nil {
				return err
			}
			if owner == "" {
				return fmt.Errorf("--owner is required non-interactively")
			}
			if len(res.Specs) == 0 {
				return fmt.Errorf("no spec files found in %s", p)
			}
			if len(chroots) == 0 {
				return fmt.Errorf("--chroot is required (chroots cannot be guessed)")
			}
			if res.CloneURL == "" {
				return fmt.Errorf("no git origin detected; a source URL cannot be inferred")
			}
			m := res.Proposed
			if m == nil {
				m = &manifest.Manifest{}
			}
			m.Metadata.Owner = owner
			if name != "" {
				m.Metadata.Name = name
			}
			m.Spec.Chroots.Enabled = chroots

			// Write the manifest.
			manifestPath := "copr.yaml"
			data, err := m.MarshalYAML()
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("--yes is required to write files and create the project non-interactively")
			}
			if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
				return err
			}
			// Reuse the apply logic: create the project and packages.
			if err := applyManifest(cmd.Context(), app, m, false); err != nil {
				return err
			}
			return renderResult(cmd, &out, map[string]any{
				"init":               true,
				"manifest":           manifestPath,
				"project":            owner + "/" + m.Metadata.Name,
				"packages":           len(m.Spec.Packages),
				"chroots":            len(chroots),
				"inferred":           len(res.Specs),
				"decisions_required": res.Decisions,
			})
		},
	}
	out.bind(cmd)
	cmd.Flags().StringVar(&path, "path", "", "path to the source repository")
	cmd.Flags().StringVar(&owner, "owner", "", "owner of the Copr project")
	cmd.Flags().StringVar(&name, "name", "", "Copr project name (default: repo name)")
	cmd.Flags().StringSliceVar(&chroots, "chroot", nil, "chroots to enable")
	bindChrootCompletion(app, cmd, "chroot")
	cmd.Flags().BoolVar(&yes, "yes", false, "assume yes")
	return cmd
}

func newSyncCmd(app *App) *cobra.Command {
	var out outFlags
	var file string
	var check bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile the manifest against the source repo and Copr",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
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
			if check {
				if len(diffs) > 0 {
					return cerr.New("drift", cerr.ExitDrift, "project has drifted from the manifest")
				}
				return renderResult(cmd, &out, map[string]any{"sync": "ok"})
			}
			return renderResult(cmd, &out, map[string]any{"sync": "ok", "diffs": diffs})
		},
	}
	out.bind(cmd)
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file")
	cmd.Flags().BoolVar(&check, "check", false, "CI gate: exit 12 on drift, mutate nothing")
	return cmd
}
