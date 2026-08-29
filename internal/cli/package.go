package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newPackageCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Manage Copr packages",
	}
	out.bind(cmd)
	cmd.AddCommand(
		newPackageCreateCmd(app, &out),
		newPackageEditCmd(app, &out),
		newPackageGetCmd(app, &out),
		newPackageListCmd(app, &out),
		newPackageResetCmd(app, &out),
		newPackageDeleteCmd(app, &out),
	)
	return cmd
}

// sourceFlags carries the shared source definition flags for all source types.
type sourceFlags struct {
	sourceType string

	cloneURL, commit, subdir, spec, scmType, method string
	name, distgit, namespace                        string
	pypiName, pypiVersion, pythonVersions           string
	gemName                                         string
	script, scriptChroot, scriptBuilddeps           string
	urls                                            []string
	uploadPath                                      string
	autoRebuild                                     bool
}

func (s *sourceFlags) bind(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&s.sourceType, "source", "", "source type: scm, distgit, pypi, rubygems, custom, url, upload")

	f.StringVar(&s.cloneURL, "clone-url", "", "scm: clone URL")
	f.StringVar(&s.commit, "commit", "", "scm/distgit: commit or branch ref")
	f.StringVar(&s.subdir, "subdir", "", "scm: subdirectory containing the spec")
	f.StringVar(&s.spec, "spec", "", "scm: spec file path")
	f.StringVar(&s.scmType, "scm-type", "git", "scm: git or svn")
	f.StringVar(&s.method, "method", "rpkg", "scm: rpkg, tito, tito_test, make_srpm")

	f.StringVar(&s.name, "name", "", "distgit: package name")
	f.StringVar(&s.distgit, "distgit", "", "distgit: fedora, centos, ...")
	f.StringVar(&s.namespace, "namespace", "", "distgit: namespace")

	f.StringVar(&s.pypiName, "pypi-name", "", "pypi: package name")
	f.StringVar(&s.pypiVersion, "pypi-version", "", "pypi: version")
	f.StringVar(&s.pythonVersions, "python-versions", "", "pypi: python versions")

	f.StringVar(&s.gemName, "gem", "", "rubygems: gem name")

	f.StringVar(&s.script, "script", "", "custom: script path")
	f.StringVar(&s.scriptChroot, "script-chroot", "", "custom: script chroot")
	f.StringVar(&s.scriptBuilddeps, "script-builddeps", "", "custom: script build deps")

	f.StringSliceVar(&s.urls, "url", nil, "url: SRPM or spec URLs")

	f.StringVar(&s.uploadPath, "upload", "", "upload: local SRPM path")

	f.BoolVar(&s.autoRebuild, "auto-rebuild", false, "enable webhook auto-rebuild")
}

// sourceMap builds the source-dict form fields for the current source type.
func (s *sourceFlags) sourceMap() (copr.SourceType, map[string]any, error) {
	m := map[string]any{}
	switch copr.SourceType(s.sourceType) {
	case copr.SourceSCM:
		if s.cloneURL == "" {
			return "", nil, fmt.Errorf("--clone-url is required for scm source")
		}
		m["clone_url"] = s.cloneURL
		setSS(m, "committish", s.commit)
		setSS(m, "subdirectory", s.subdir)
		setSS(m, "spec", s.spec)
		setSS(m, "scm_type", s.scmType)
		setSS(m, "source_build_method", s.method)
	case copr.SourceDistGit:
		if s.name == "" {
			return "", nil, fmt.Errorf("--name is required for distgit source")
		}
		m["package_name"] = s.name
		setSS(m, "distgit", s.distgit)
		setSS(m, "namespace", s.namespace)
		setSS(m, "committish", s.commit)
	case copr.SourcePyPI:
		if s.pypiName == "" {
			return "", nil, fmt.Errorf("--pypi-name is required for pypi source")
		}
		m["pypi_package_name"] = s.pypiName
		setSS(m, "pypi_package_version", s.pypiVersion)
		if s.pythonVersions != "" {
			m["python_versions"] = []string{s.pythonVersions}
		}
	case copr.SourceRubyGems:
		if s.gemName == "" {
			return "", nil, fmt.Errorf("--gem is required for rubygems source")
		}
		m["gem_name"] = s.gemName
	case copr.SourceCustom:
		if s.script == "" {
			return "", nil, fmt.Errorf("--script is required for custom source")
		}
		m["script"] = s.script
		setSS(m, "chroot", s.scriptChroot)
		setSS(m, "builddeps", s.scriptBuilddeps)
	case copr.SourceURL:
		if len(s.urls) == 0 {
			return "", nil, fmt.Errorf("--url is required for url source")
		}
		m["pkgs"] = s.urls
	case copr.SourceUpload:
		if s.uploadPath == "" {
			return "", nil, fmt.Errorf("--upload is required for upload source")
		}
	default:
		return "", nil, fmt.Errorf("unknown or missing source type %q", s.sourceType)
	}
	return copr.SourceType(s.sourceType), m, nil
}

func setSS(m map[string]any, k, v string) {
	if v != "" {
		m[k] = v
	}
}

func newPackageCreateCmd(app *App, out *outFlags) *cobra.Command {
	var src sourceFlags
	cmd := &cobra.Command{
		Use:   "create REF/PKG",
		Short: "Create a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
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
			if err := c.CreatePackage(cmd.Context(), copr.PackageCreate{
				Owner: r.Owner, Project: r.Project, Name: r.Segment,
				SourceType: st, Source: sm, AutoRebuild: src.autoRebuild, SetAutoRebuild: true,
			}); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"created": args[0]})
		},
	}
	src.bind(cmd)
	return cmd
}

func newPackageEditCmd(app *App, out *outFlags) *cobra.Command {
	var src sourceFlags
	cmd := &cobra.Command{
		Use:   "edit REF/PKG",
		Short: "Edit a package's source definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
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
			if err := c.EditPackage(cmd.Context(), copr.PackageCreate{
				Owner: r.Owner, Project: r.Project, Name: r.Segment,
				SourceType: st, Source: sm, AutoRebuild: src.autoRebuild,
				SetAutoRebuild: cmd.Flags().Changed("auto-rebuild"),
			}); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"edited": args[0]})
		},
	}
	src.bind(cmd)
	return cmd
}

func newPackageGetCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get REF/PKG",
		Short: "Show a package's source definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			p, err := c.GetPackage(cmd.Context(), r.Owner, r.Project, r.Segment)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("Name", p.Name)
				t.Add("Source type", string(p.SourceType))
				t.Add("Auto rebuild", fmt.Sprintf("%v", p.AutoRebuild))
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, p)
		},
	}
	return cmd
}

func newPackageResetCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "reset REF/PKG",
		Short: "Clear a package's stored source definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
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
			if err := c.ResetPackage(cmd.Context(), r.Owner, r.Project, r.Segment); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"reset": args[0]})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}

func newPackageListCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list REF",
		Short: "List packages in a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			pkgs, err := c.ListPackages(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("NAME", "TYPE", "AUTO-REBUILD")
				for _, p := range pkgs {
					t.Add(p.Name, string(p.SourceType), fmt.Sprintf("%v", p.AutoRebuild))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, pkgs)
		},
	}
	return cmd
}

func newPackageDeleteCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete REF/PKG",
		Short: "Delete a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
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
			if err := c.DeletePackage(cmd.Context(), r.Owner, r.Project, r.Segment); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"deleted": args[0]})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}
