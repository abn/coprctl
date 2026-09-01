package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/events"
	"github.com/abn/coprctl/internal/logstream"
	"github.com/abn/coprctl/internal/render"
	ctrruntime "github.com/abn/coprctl/internal/runtime"
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
		newBuildRebuildCmd(app, &out),
		newBuildWatchCmd(app, &out),
		newBuildCancelCmd(app, &out),
		newBuildDeleteCmd(app, &out),
		newBuildReproduceCmd(app, &out),
		newBuildSrpmCmd(app, &out),
		newBuildDownloadCmd(app, &out),
	)
	return cmd
}

func newBuildRebuildCmd(app *App, out *outFlags) *cobra.Command {
	var chroots *[]string
	var preflight bool
	var onlyFailed string
	cmd := &cobra.Command{
		Use:   "rebuild REF/PKG [build flags]",
		Short: "Rebuild a package from its stored source definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parsePackageRef(app, args)
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			if onlyFailed != "" {
				buildID, err := strconv.Atoi(onlyFailed)
				if err != nil {
					return cerr.New("invalid_build_id", cerr.ExitUsage,
						"--only-failed expects a numeric build id")
				}
				prev, err := c.GetBuildDetail(cmd.Context(), buildID)
				if err != nil {
					return err
				}
				if prev.Builds == nil {
					// A degraded fetch leaves Builds nil; without per-chroot
					// states we cannot know which chroots failed, and guessing
					// would submit a full rebuild.
					return cerr.Transport(fmt.Sprintf(
						"per-chroot detail for build %d could not be fetched; rebuild was not submitted", buildID))
				}
				failed := failedChroots(prev)
				if len(failed) == 0 {
					if isHuman(cmd, out.format) {
						fmt.Fprintf(cmd.OutOrStdout(), "no failed chroots to rebuild for build %d\n", buildID)
						return nil
					}
					return renderResult(cmd, out, map[string]any{"build_id": buildID, "rebuilt": 0})
				}
				*chroots = failed
			}
			// Preflight checks the local container runtime is available; a
			// full Tier-1 build needs a local spec and is covered by try.
			if preflight {
				if err := runRebuildPreflight(cmd); err != nil {
					return err
				}
			}
			builds, err := c.RebuildPackage(cmd.Context(), r.Owner, r.Project, r.Segment, *chroots)
			if err != nil {
				return err
			}
			b := builds[0]
			return renderHumanOr(cmd, out, builds, func() *render.Table {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("ID", fmt.Sprintf("%d", b.ID))
				t.Add("State", b.State)
				if onlyFailed != "" {
					t.Add("Rebuilt chroots", fmt.Sprintf("%d", len(*chroots)))
				}
				return t
			})
		},
	}
	chroots = addChrootFlag(app, cmd, "chroots to build in (globs allowed)", true)
	cmd.Flags().BoolVar(&preflight, "preflight", false, "check a container runtime is available before submitting")
	cmd.Flags().StringVar(&onlyFailed, "only-failed", "", "rebuild only chroots that failed in this build id (overrides --chroot)")
	return cmd
}

// failedChroots returns the sorted chroot names of a build whose state is
// "failed".
func failedChroots(b *copr.Build) []string {
	var names []string
	for name, state := range b.ChrootStates() {
		if state == "failed" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// runRebuildPreflight verifies the local container runtime is available for a
// later Tier-1 build and reports the result; it does not build anything.
func runRebuildPreflight(cmd *cobra.Command) error {
	rt, err := ctrruntime.Detect("")
	if err != nil {
		return cerr.New("no_runtime", cerr.ExitPrecondition,
			"preflight requested but no container runtime is available")
	}
	// We do not have a local spec path for an arbitrary Copr package; report
	// that a local checkout is needed for preflight.
	fmt.Fprintf(cmd.OutOrStdout(), "preflight: container runtime %s available\n", rt.Name())
	fmt.Fprintln(cmd.OutOrStdout(), "preflight: pass a local spec path to 'coprctl try' for a full Tier-1 build; continuing with submission")
	return nil
}

// buildReproduceResult wraps the log reproduction recipe with the stored
// source build config for machine output; the config fields are absent when no
// config is available.
type buildReproduceResult struct {
	*logstream.Reproduction
	SourceType   string         `json:"source_type,omitempty"`
	SourceDict   map[string]any `json:"source_dict,omitempty"`
	MemoryLimit  *int           `json:"memory_limit,omitempty"`
	Timeout      *int           `json:"timeout,omitempty"`
	IsBackground *bool          `json:"is_background,omitempty"`
}

func newBuildReproduceCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reproduce BUILD_ID/CHROOT",
		Short: "Print the local mock reproduction recipe from a build log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseBuildChrootRef(args)
			if err != nil {
				return err
			}
			client, err := app.ReadClient()
			if err != nil {
				return err
			}
			rep, recipeErr := logstream.NewTailer(client, nil).ExtractReproduction(cmd.Context(), r.BuildID, r.BuildCht)
			// The stored source build config enriches the recipe and is the
			// fallback when the log has none; only a 404 on an old build is
			// ignored, any other fetch error surfaces.
			var cfg *copr.SourceBuildConfig
			cfgC, cfgErr := client.GetSourceBuildConfig(cmd.Context(), r.BuildID)
			if cfgErr == nil {
				cfg = cfgC
			} else if cerr.ExitCodeFor(cfgErr) != cerr.ExitNotFound {
				return cfgErr
			}
			if recipeErr != nil && cfg == nil {
				return recipeErr
			}
			result := &buildReproduceResult{Reproduction: rep}
			if cfg != nil {
				result.SourceType = cfg.SourceType
				result.SourceDict = cfg.SourceDict
				result.MemoryLimit = cfg.MemoryLimit
				result.Timeout = cfg.Timeout
				result.IsBackground = &cfg.IsBackground
			}
			if isHuman(cmd, out.format) {
				if rep != nil && rep.Recipe != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "# Reproduce this build locally at mock-level fidelity")
					fmt.Fprintln(cmd.OutOrStdout(), "sudo dnf install copr-rpmbuild mock")
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", rep.Recipe)
					if rep.TaskURL != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "\n# task: %s\n", rep.TaskURL)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "\n# Or reproduce at container (Tier 1) fidelity with:")
					fmt.Fprintln(cmd.OutOrStdout(), "coprctl try ./rpm --chroot <chroot>")
				}
				if cfg != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "\n# Source definition:\n%s\n", submitInvocationFromSource(cfg.SourceType, cfg.SourceDict))
				}
				return nil
			}
			return renderResult(cmd, out, result)
		},
	}
	return cmd
}

// submitInvocationFromSource reconstructs a build submit invocation from a
// stored source-build-config dict. The stored keys differ from the submit-time
// sourceMap keys: scm stores type and srpm_build_method, url stores the single
// url (submitted as pkgs), and a distgit package with a custom clone URL
// stores it under clone_url. Unknown types emit the type only.
func submitInvocationFromSource(sourceType string, dict map[string]any) string {
	switch sourceType {
	case "scm":
		var b strings.Builder
		b.WriteString("coprctl build submit REF --source scm")
		if v := dictString(dict, "clone_url"); v != "" {
			b.WriteString(" --clone-url " + v)
		}
		if v := dictString(dict, "type"); v != "" {
			b.WriteString(" --scm-type " + v)
		}
		if v := dictString(dict, "committish"); v != "" {
			b.WriteString(" --commit " + v)
		}
		if v := dictString(dict, "subdirectory"); v != "" {
			b.WriteString(" --subdir " + v)
		}
		if v := dictString(dict, "spec"); v != "" {
			b.WriteString(" --spec " + v)
		}
		if v := dictString(dict, "srpm_build_method"); v != "" {
			b.WriteString(" --method " + v)
		}
		return b.String()
	case "link", "url":
		if v := dictString(dict, "url"); v != "" {
			return "coprctl build submit REF --source url --url " + v
		}
		return "coprctl build submit REF --source url"
	case "distgit":
		var b strings.Builder
		b.WriteString("coprctl build submit REF --source distgit")
		if v := dictString(dict, "package_name"); v != "" {
			b.WriteString(" --name " + v)
		}
		if v := dictString(dict, "clone_url"); v != "" {
			b.WriteString(" --clone-url " + v)
		}
		if v := dictString(dict, "distgit"); v != "" {
			b.WriteString(" --distgit " + v)
		}
		if v := dictString(dict, "namespace"); v != "" {
			b.WriteString(" --namespace " + v)
		}
		if v := dictString(dict, "committish"); v != "" {
			b.WriteString(" --commit " + v)
		}
		return b.String()
	case "pypi":
		var b strings.Builder
		b.WriteString("coprctl build submit REF --source pypi")
		if v := dictString(dict, "pypi_package_name"); v != "" {
			b.WriteString(" --pypi-name " + v)
		}
		if v := dictString(dict, "pypi_package_version"); v != "" {
			b.WriteString(" --pypi-version " + v)
		}
		if v, ok := dict["python_versions"].([]any); ok && len(v) > 0 {
			versions := make([]string, 0, len(v))
			for _, p := range v {
				if s, ok := p.(string); ok && s != "" {
					versions = append(versions, s)
				}
			}
			if len(versions) > 0 {
				b.WriteString(" --python-versions " + strings.Join(versions, ","))
			}
		}
		return b.String()
	case "rubygems":
		if v := dictString(dict, "gem_name"); v != "" {
			return "coprctl build submit REF --source rubygems --gem " + v
		}
		return "coprctl build submit REF --source rubygems"
	case "custom":
		var b strings.Builder
		b.WriteString("coprctl build submit REF --source custom")
		if v := dictString(dict, "script"); v != "" {
			b.WriteString(" --script " + v)
		}
		if v := dictString(dict, "chroot"); v != "" {
			b.WriteString(" --script-chroot " + v)
		}
		if v := dictString(dict, "builddeps"); v != "" {
			b.WriteString(" --script-builddeps " + v)
		}
		return b.String()
	default:
		return "coprctl build submit REF --source " + sourceType
	}
}

// dictString returns the string value of a stored source_dict key, or "" when
// the key is absent or holds a non-string value.
func dictString(dict map[string]any, key string) string {
	if v, ok := dict[key].(string); ok {
		return v
	}
	return ""
}

// buildGetResult enriches a build with its per-chroot detail for machine
// output; "chroots" is the structured chroot list rather than the bare names.
type buildGetResult struct {
	*copr.Build
	Chroots []copr.BuildChroot `json:"chroots"`
}

func newBuildGetCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get BUILD_ID",
		Short: "Show a build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseBuildID(args)
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			b, err := c.GetBuild(cmd.Context(), id)
			if err != nil {
				return err
			}
			chroots, err := c.ListBuildChroots(cmd.Context(), id)
			if err != nil {
				return err
			}
			if isHuman(cmd, out.format) {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("ID", fmt.Sprintf("%d", b.ID))
				t.Add("Project", b.OwnerName+"/"+b.ProjectName)
				t.Add("Package", b.PackageName())
				t.Add("State", b.State)
				if err := renderResult(cmd, out, t); err != nil {
					return err
				}
				if len(chroots) == 0 {
					return nil
				}
				sort.Slice(chroots, func(i, j int) bool { return chroots[i].Chroot < chroots[j].Chroot })
				fmt.Fprintln(cmd.OutOrStdout())
				ct := render.NewTable("CHROOT", "STATE")
				for _, ch := range chroots {
					ct.Add(ch.Chroot, ch.State)
				}
				return renderResult(cmd, out, ct)
			}
			return renderResult(cmd, out, buildGetResult{Build: b, Chroots: chroots})
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
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			builds, err := c.ListBuilds(cmd.Context(), r.Owner, r.Project, "", limit)
			if err != nil {
				return wrapGroupNotFoundHint(r.Owner, instanceBase(app), err)
			}
			return renderHumanOr(cmd, out, builds, func() *render.Table {
				t := render.NewTable("ID", "PACKAGE", "STATE")
				for _, b := range builds {
					t.Add(fmt.Sprintf("%d", b.ID), b.PackageName(), b.State)
				}
				return t
			})
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "number of builds to list")
	return cmd
}

func newBuildSubmitCmd(app *App, out *outFlags) *cobra.Command {
	var src sourceFlags
	var chroots *[]string
	var dir string
	var from, runtimeName string
	var rpmPath, sha256 string
	var watch bool
	var background, enableNet bool
	var timeout int
	var bootstrap, isolation string
	var excludeChroots *[]string
	var afterBuildID, withBuildID int
	cmd := &cobra.Command{
		Use:   "submit REF --source TYPE [flags]",
		Short: "Submit a build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			// The batch options are mutually exclusive upstream; fail locally
			// before hitting the server.
			if cmd.Flags().Changed("after-build-id") && cmd.Flags().Changed("with-build-id") {
				return cerr.Usage("--after-build-id and --with-build-id are mutually exclusive")
			}
			// bootstrap and isolation are whitelisted select values upstream.
			switch bootstrap {
			case "", "on", "off", "default", "image", "unchanged":
			default:
				return cerr.Usage(fmt.Sprintf("invalid --bootstrap %q: one of on, off, default, image, unchanged", bootstrap))
			}
			switch isolation {
			case "", "simple", "nspawn", "default", "unchanged":
			default:
				return cerr.Usage(fmt.Sprintf("invalid --isolation %q: one of simple, nspawn, default, unchanged", isolation))
			}
			if cmd.Flags().Changed("timeout") && timeout <= 0 {
				return cerr.Usage("--timeout must be a positive number of seconds")
			}

			// Tri-state bools: absent keeps the server/project default, while
			// an explicit --enable-net=false actively disables it.
			var backgroundPtr *bool
			if cmd.Flags().Changed("background") {
				backgroundPtr = &background
			}
			var enableNetPtr *bool
			if cmd.Flags().Changed("enable-net") {
				enableNetPtr = &enableNet
			}
			var timeoutPtr *int
			if cmd.Flags().Changed("timeout") {
				timeoutPtr = &timeout
			}
			var afterID *int
			if cmd.Flags().Changed("after-build-id") {
				afterID = &afterBuildID
			}
			var withID *int
			if cmd.Flags().Changed("with-build-id") {
				withID = &withBuildID
			}
			opts := copr.UploadOptions{
				Background: backgroundPtr, EnableNet: enableNetPtr, Timeout: timeoutPtr,
				Bootstrap: bootstrap, Isolation: isolation,
				AfterBuildID: afterID, WithBuildID: withID,
				ExcludeChroots: *excludeChroots,
			}

			c, err := app.Client()
			if err != nil {
				return err
			}

			// Warn when a targeted chroot is EOL and will not accept builds.
			warnInactiveChroots(cmd, *chroots, app)

			// The :dir from the reference wins over the --dir flag.
			effDir := dir
			if r.Dir != "" {
				effDir = r.Dir
			}

			// --from builds a source RPM locally from a spec directory, then
			// uploads and submits it, chaining build srpm into submit.
			if from != "" {
				if _, statErr := os.Stat(from); statErr != nil {
					return fmt.Errorf("path %q not found; pass a local spec directory", from)
				}
				spec, err := findSpec(from)
				if err != nil {
					return err
				}
				buildDir, err := prepareSpecDir(spec)
				if err != nil {
					return err
				}
				if buildDir != filepath.Dir(spec) {
					defer os.RemoveAll(buildDir)
				}
				buildSpec, err := findSpec(buildDir)
				if err != nil {
					return err
				}
				ch := "fedora-rawhide-x86_64"
				if len(*chroots) > 0 {
					ch = (*chroots)[0]
				}
				br, err := resolveBuilder(runtimeName, "srpm")
				if err != nil {
					return err
				}
				srpm, err := br.BuildSRPM(cmd.Context(), buildSpec, ch, cmd.OutOrStdout())
				if err != nil {
					return err
				}
				b, err := c.UploadBuild(cmd.Context(), r.Owner, r.Project, srpm, effDir, opts)
				if err != nil {
					return err
				}
				if err := renderHumanOr(cmd, out, b, func() *render.Table {
					t := render.NewTable("FIELD", "VALUE")
					t.Add("ID", fmt.Sprintf("%d", b.ID))
					t.Add("State", b.State)
					t.Add("Backend", br.Name())
					t.Add("SRPM", srpm)
					return t
				}); err != nil {
					return err
				}
				if watch {
					return watchBuild(cmd, app, b.ID)
				}
				return nil
			}

			// Upload builds take the SRPM as a multipart payload; the chroot
			// set comes from the SRPM itself, so there is no per-chroot JSON.
			if copr.SourceType(src.sourceType) == copr.SourceUpload {
				if src.uploadPath == "" {
					return cerr.Usage("--upload is required for upload source")
				}
				if _, statErr := os.Stat(src.uploadPath); statErr != nil {
					return fmt.Errorf("path %q not found; pass a local SRPM", src.uploadPath)
				}
				if len(*chroots) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: upload builds build in the chroots declared by the SRPM; --chroot is ignored\n")
				}
				b, err := c.UploadBuild(cmd.Context(), r.Owner, r.Project, src.uploadPath, effDir, opts)
				if err != nil {
					return err
				}
				if err := renderHumanOr(cmd, out, b, func() *render.Table {
					t := render.NewTable("FIELD", "VALUE")
					t.Add("ID", fmt.Sprintf("%d", b.ID))
					t.Add("State", b.State)
					t.Add("SRPM", src.uploadPath)
					return t
				}); err != nil {
					return err
				}
				if watch {
					return watchBuild(cmd, app, b.ID)
				}
				return nil
			}

			if copr.SourceType(src.sourceType) == copr.SourceRpmUpload {
				if rpmPath == "" {
					return cerr.Usage("--rpm is required for rpm-upload source")
				}
				if _, statErr := os.Stat(rpmPath); statErr != nil {
					return fmt.Errorf("path %q not found; pass a local RPM", rpmPath)
				}
				if len(*chroots) == 0 {
					return cerr.Usage("--chroot is required for rpm-upload source (an omitted chroot list would publish to every project chroot)")
				}
				b, err := c.UploadRpmBuild(cmd.Context(), copr.RpmUploadSubmit{
					Owner: r.Owner, Project: r.Project, Dir: effDir,
					RpmPath: rpmPath, Chroots: *chroots, SHA256: sha256,
				})
				if err != nil {
					return mapRpmUploadError(err)
				}
				if err := renderHumanOr(cmd, out, b, func() *render.Table {
					t := render.NewTable("FIELD", "VALUE")
					t.Add("ID", fmt.Sprintf("%d", b.ID))
					t.Add("State", b.State)
					t.Add("RPM", rpmPath)
					return t
				}); err != nil {
					return err
				}
				if watch {
					return watchBuild(cmd, app, b.ID)
				}
				return nil
			}

			st, sm, err := src.sourceMap()
			if err != nil {
				return err
			}
			builds, err := c.SubmitBuild(cmd.Context(), copr.BuildSubmit{
				Owner: r.Owner, Project: r.Project,
				SourceType: st, Source: sm, Chroots: *chroots, Dir: effDir,
				UploadOptions: opts,
			})
			if err != nil {
				return err
			}
			if err := renderHumanOr(cmd, out, builds, func() *render.Table {
				t := render.NewTable("ID", "STATE")
				for _, b := range builds {
					t.Add(fmt.Sprintf("%d", b.ID), b.State)
				}
				return t
			}); err != nil {
				return err
			}
			if watch {
				for _, b := range builds {
					if err := watchBuild(cmd, app, b.ID); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	src.bind(cmd)
	chroots = addChrootFlag(app, cmd, "chroots to build in (globs allowed; required for rpm-upload)", true)
	cmd.Flags().StringVar(&dir, "dir", "", "side repo / project directory")
	cmd.Flags().StringVar(&from, "from", "", "build a local SRPM from this spec directory, then upload and submit")
	cmd.Flags().StringVar(&runtimeName, "runtime", "auto", "build backend for --from: auto, container, native, mock")
	cmd.Flags().StringVar(&rpmPath, "rpm", "", "local already-built RPM to publish directly (rpm-upload)")
	cmd.Flags().StringVar(&sha256, "sha256", "", "expected SHA256 hex digest of the RPM; the build is rejected on mismatch (rpm-upload)")
	cmd.Flags().BoolVar(&background, "background", false, "submit and return after queueing (the server reports is_background)")
	cmd.Flags().BoolVar(&enableNet, "enable-net", false, "enable network access in the buildroot")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "per-build timeout in seconds")
	cmd.Flags().StringVar(&bootstrap, "bootstrap", "", "bootstrap mode: on, off, default, image, unchanged")
	cmd.Flags().StringVar(&isolation, "isolation", "", "isolation mode: simple, nspawn, default, unchanged")
	excludeChroots = addExcludeChrootFlag(app, cmd)
	cmd.Flags().IntVar(&afterBuildID, "after-build-id", 0, "build after the batch containing this build id")
	cmd.Flags().IntVar(&withBuildID, "with-build-id", 0, "build in the same batch as this build id")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "wait for the submitted build to reach a terminal state")
	bindRefCompletion(app, cmd)
	return cmd
}

func mapRpmUploadError(err error) error {
	var ce *cerr.Error
	if errors.As(err, &ce) && ce.Code == "bad_request" && strings.Contains(ce.Hint, "not enabled") {
		return cerr.New("feature_disabled", cerr.ExitPrecondition,
			"rpm-upload is not enabled on this Copr instance").
			WithHint("DIRECT_RPM_UPLOAD is off here; rpm-upload works on instances that enable it, typically self-hosted, not on Fedora infrastructure")
	}
	return err
}

func newBuildCancelCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel BUILD_ID",
		Short: "Cancel a build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseBuildID(args)
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			if err := c.CancelBuild(cmd.Context(), id); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"canceled": id})
		},
	}
	return cmd
}

func newBuildDeleteCmd(app *App, out *outFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "delete BUILD_ID...",
		Short: "Delete one or more builds",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !*yes {
				return confirmRequired("--yes")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			ids, err := parseBuildIDs(args)
			if err != nil {
				return err
			}
			// One atomic batch call: a single invalid or running id aborts the
			// whole delete instead of leaving a partial one.
			if err := c.DeleteBuilds(cmd.Context(), ids); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"deleted": len(args)})
		},
	}
	yes = addYesFlag(cmd, yesHelp, true)
	return cmd
}

// newBuildWatchCmd polls a build and prints state changes until it reaches a
// terminal state. It is the plain-text rendering of the same event stream that
// drives the TUI and JSONL output.
func newBuildWatchCmd(app *App, out *outFlags) *cobra.Command {
	var until string
	cmd := &cobra.Command{
		Use:   "watch BUILD_ID...",
		Short: "Watch a build until it reaches a terminal state",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			ids, err := parseBuildIDs(args)
			if err != nil {
				return err
			}
			bus := events.New()
			defer bus.Close()
			sub := bus.Subscribe(1024)
			ctx, cancel := interruptible(cmd.Context())
			defer cancel()
			src := &events.PollSource{Client: c, BuildIDs: ids, Interval: pollInterval()}
			go src.Run(ctx, bus)
			return watchConsume(ctx, bus, sub, until, out.format, cmd)
		},
	}
	cmd.Flags().StringVar(&until, "until", "terminal", "stop at state (succeeded, failed, canceled, terminal)")
	return cmd
}

func watchConsume(ctx context.Context, bus *events.Bus, ch chan events.Event, until, format string, cmd *cobra.Command) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if ev.Kind == events.KindBuildState || ev.Kind == events.KindChrootState {
				if format == "jsonl" {
					if err := printEvent(cmd.OutOrStdout(), ev); err != nil {
						return err
					}
				} else {
					if ev.Chroot != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "build %d %s: %s -> %s\n", ev.BuildID, ev.Chroot, ev.Prev, ev.State)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "build %d: %s\n", ev.BuildID, ev.State)
					}
				}
				if ev.Kind == events.KindBuildState && reached(ev.State, until) {
					return nil
				}
			}
		}
	}
}

func reached(state, until string) bool {
	if until == "terminal" {
		return copr.IsTerminal(state)
	}
	return state == until
}

// watchBuild polls a single build until it reaches a terminal state, printing
// state changes. It is the chain target for build submit --watch.
func watchBuild(cmd *cobra.Command, app *App, buildID int) error {
	c, err := app.ReadClient()
	if err != nil {
		return err
	}
	bus := events.New()
	defer bus.Close()
	sub := bus.Subscribe(1024)
	ctx, cancel := interruptible(cmd.Context())
	defer cancel()
	src := &events.PollSource{Client: c, BuildIDs: []int{buildID}, Interval: pollInterval()}
	go src.Run(ctx, bus)
	return watchConsume(ctx, bus, sub, "terminal", "plain", cmd)
}

// newBuildSrpmCmd builds a source RPM from a local spec directory using the
// rpmbuilder container, mirroring the SRPM_ONLY stage of the try preflight.
func newBuildSrpmCmd(app *App, out *outFlags) *cobra.Command {
	var path, chroot string
	var runtimeName *string
	cmd := &cobra.Command{
		Use:   "srpm [PATH]",
		Short: "Build a source RPM from a local spec (container, native, or mock)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcPath := "."
			if len(args) == 1 {
				if _, statErr := os.Stat(args[0]); statErr != nil {
					return fmt.Errorf("path %q not found; pass a local spec directory", args[0])
				}
				srcPath = args[0]
			}
			if path != "" {
				srcPath = path
			}
			spec, err := findSpec(srcPath)
			if err != nil {
				return err
			}
			buildDir, err := prepareSpecDir(spec)
			if err != nil {
				return err
			}
			if buildDir != filepath.Dir(spec) {
				defer os.RemoveAll(buildDir)
			}
			buildSpec, err := findSpec(buildDir)
			if err != nil {
				return err
			}
			if chroot == "" {
				chroot = "fedora-rawhide-x86_64"
			}
			b, err := resolveBuilder(*runtimeName, "srpm")
			if err != nil {
				return err
			}
			srpm, err := b.BuildSRPM(cmd.Context(), buildSpec, chroot, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			result := map[string]any{
				"backend": b.Name(),
				"chroot":  chroot,
				"spec":    spec,
				"output":  srpm,
			}
			return renderResult(cmd, out, result)
		},
	}
	out.bind(cmd)
	cmd.Flags().StringVar(&path, "path", "", "path to the spec directory")
	cmd.Flags().StringVar(&chroot, "chroot", "fedora-rawhide-x86_64", "chroot to build against")
	runtimeName = addRuntimeFlag(cmd)
	bindChrootCompletion(app, cmd, "chroot")
	return cmd
}
