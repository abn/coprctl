package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/events"
	"github.com/abn/coprctl/internal/logstream"
	"github.com/abn/coprctl/internal/ref"
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
	)
	return cmd
}

func newBuildRebuildCmd(app *App, out *outFlags) *cobra.Command {
	var chroots []string
	var preflight bool
	var onlyFailed string
	cmd := &cobra.Command{
		Use:   "rebuild REF/PKG [build flags]",
		Short: "Rebuild a package from its stored source definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForcePackage: true})
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
				prev, err := c.GetBuild(cmd.Context(), buildID)
				if err != nil {
					return err
				}
				failed := failedChroots(prev)
				if len(failed) == 0 {
					if isHuman(out.format) {
						fmt.Fprintf(cmd.OutOrStdout(), "no failed chroots to rebuild for build %d\n", buildID)
						return nil
					}
					return renderResult(cmd, out, map[string]any{"build_id": buildID, "rebuilt": 0})
				}
				chroots = failed
			}
			// Preflight runs a local Tier-1 build first; on failure it does
			// not submit (or warns, see try's behavior).
			if preflight {
				if err := runRebuildPreflight(cmd, app, c, r); err != nil {
					return err
				}
			}
			b, err := c.RebuildPackage(cmd.Context(), r.Owner, r.Project, r.Segment, chroots)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("ID", fmt.Sprintf("%d", b.ID))
				t.Add("State", b.State)
				if onlyFailed != "" {
					t.Add("Rebuilt chroots", fmt.Sprintf("%d", len(chroots)))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, b)
		},
	}
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to build in (globs allowed)")
	cmd.Flags().BoolVar(&preflight, "preflight", false, "run a local Tier-1 preflight before submitting")
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

// runRebuildPreflight runs a local container preflight for the package's
// project chroots and blocks submission on failure.
func runRebuildPreflight(cmd *cobra.Command, app *App, c *copr.Client, r ref.Ref) error {
	rt, err := ctrruntime.Detect("")
	if err != nil {
		return cerr.New("no_runtime", cerr.ExitPrecondition,
			"preflight requested but no container runtime is available")
	}
	// Build in the default rawhide chroot when none is specified.
	targets := []string{"fedora-rawhide-x86_64"}
	for _, ch := range []string{} {
		targets = append(targets, ch)
	}
	// We do not have a local spec path for an arbitrary Copr package; report
	// that a local checkout is needed for preflight.
	fmt.Fprintf(cmd.OutOrStdout(), "preflight: container runtime %s available\n", rt.Name())
	fmt.Fprintln(cmd.OutOrStdout(), "preflight: pass a local spec path to 'coprctl try' for a full Tier-1 build; continuing with submission")
	return nil
}

func newBuildReproduceCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reproduce BUILD_ID/CHROOT",
		Short: "Print the local mock reproduction recipe from a build log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Kind != ref.KindBuildChroot {
				return fmt.Errorf("expected a build/chroot reference, got %q", args[0])
			}
			client, err := app.ReadClient()
			if err != nil {
				return err
			}
			rep, err := logstream.NewTailer(client, nil).ExtractReproduction(cmd.Context(), r.BuildID, r.BuildCht)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				fmt.Fprintln(cmd.OutOrStdout(), "# Reproduce this build locally at mock-level fidelity")
				fmt.Fprintln(cmd.OutOrStdout(), "sudo dnf install copr-rpmbuild mock")
				if rep.Recipe != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", rep.Recipe)
				}
				if rep.TaskURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n# task: %s\n", rep.TaskURL)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\n# Or reproduce at container (Tier 1) fidelity with:")
				fmt.Fprintln(cmd.OutOrStdout(), "coprctl try ./rpm --chroot <chroot>")
				return nil
			}
			return renderResult(cmd, out, rep)
		},
	}
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
			c, err := app.ReadClient()
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
			c, err := app.ReadClient()
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
	var from, runtimeName string
	cmd := &cobra.Command{
		Use:   "submit REF --source TYPE [flags]",
		Short: "Submit a build",
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

			// Warn when a targeted chroot is EOL and will not accept builds.
			warnInactiveChroots(cmd, chroots, app)

			// --from builds a source RPM locally from a spec directory, then
			// uploads and submits it, chaining build srpm into submit.
			if from != "" {
				if _, statErr := os.Stat(from); statErr != nil {
					return fmt.Errorf("path %q not found; pass a local spec directory", from)
				}
				rt, err := ctrruntime.Detect(runtimeName)
				if err != nil {
					return cerr.New("no_runtime", cerr.ExitPrecondition, err.Error())
				}
				spec, err := findSpec(from)
				if err != nil {
					return err
				}
				ch := "fedora-rawhide-x86_64"
				if len(chroots) > 0 {
					ch = chroots[0]
				}
				m := resolveChrootImage(ch)
				if m.Match == "none" {
					return cerr.New("no_image", cerr.ExitPrecondition, m.Reason)
				}
				if err := rt.Run(cmd.Context(), ctrruntime.RunSpec{
					Image:   m.Image,
					WorkDir: filepath.Dir(spec),
					Mount:   "/sources",
					Env:     []string{"SRPM_ONLY=1", "OUTPUT=/sources/.rpmbuild"},
					Args:    []string{"/usr/bin/rpmbuilder"},
					Stdout:  cmd.OutOrStdout(),
				}); err != nil {
					return cerr.New("srpm_failed", cerr.ExitBuildFailed, "source RPM build failed")
				}
				srpm, err := findSRPM(filepath.Dir(spec))
				if err != nil {
					return err
				}
				b, err := c.UploadBuild(cmd.Context(), r.Owner, r.Project, srpm)
				if err != nil {
					return err
				}
				if isHuman(out.format) {
					t := render.NewTable("FIELD", "VALUE")
					t.Add("ID", fmt.Sprintf("%d", b.ID))
					t.Add("State", b.State)
					t.Add("SRPM", srpm)
					return renderResult(cmd, out, t)
				}
				return renderResult(cmd, out, b)
			}

			st, sm, err := src.sourceMap()
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
	cmd.Flags().StringVar(&from, "from", "", "build a local SRPM from this spec directory, then upload and submit")
	cmd.Flags().StringVar(&runtimeName, "runtime", "", "container runtime for --from (podman, docker, auto)")
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

func newBuildDeleteCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete BUILD_ID...",
		Short: "Delete one or more builds",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return confirmRequired("--yes")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			for _, a := range args {
				r, err := ref.Parse(a, nil)
				if err != nil {
					return err
				}
				if r.Kind != ref.KindBuild {
					return fmt.Errorf("expected a build id, got %q", a)
				}
				if err := c.DeleteBuild(cmd.Context(), r.BuildID); err != nil {
					return err
				}
			}
			return renderResult(cmd, out, map[string]any{"deleted": len(args)})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}

// newBuildWatchCmd polls a build and prints state changes until it reaches a
// terminal state. It is the plain-text rendering of the same event stream that
// drives the TUI and JSONL output.
func newBuildWatchCmd(app *App, out *outFlags) *cobra.Command {
	var until, format string
	cmd := &cobra.Command{
		Use:   "watch BUILD_ID...",
		Short: "Watch a build until it reaches a terminal state",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			var ids []int
			for _, a := range args {
				r, err := ref.Parse(a, nil)
				if err != nil {
					return err
				}
				if r.Kind != ref.KindBuild {
					return fmt.Errorf("expected a build id, got %q", a)
				}
				ids = append(ids, r.BuildID)
			}
			bus := events.New()
			defer bus.Close()
			sub := bus.Subscribe(1024)
			ctx, cancel := interruptible(cmd.Context())
			defer cancel()
			src := &events.PollSource{Client: c, BuildIDs: ids, Interval: pollInterval()}
			go src.Run(ctx, bus)
			return watchConsume(ctx, bus, sub, until, format, cmd)
		},
	}
	cmd.Flags().StringVar(&until, "until", "terminal", "stop at state (succeeded, failed, canceled, terminal)")
	cmd.Flags().StringVarP(&format, "output", "o", "plain", "output format: plain, jsonl")
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
					printEvent(ev)
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

// newBuildSrpmCmd builds a source RPM from a local spec directory using the
// rpmbuilder container, mirroring the SRPM_ONLY stage of the try preflight.
func newBuildSrpmCmd(app *App, out *outFlags) *cobra.Command {
	var path, chroot, runtimeName string
	cmd := &cobra.Command{
		Use:   "srpm [PATH]",
		Short: "Build a source RPM from a local spec using a container",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := ctrruntime.Detect(runtimeName)
			if err != nil {
				return cerr.New("no_runtime", cerr.ExitPrecondition, err.Error())
			}
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
			if chroot == "" {
				chroot = "fedora-rawhide-x86_64"
			}
			m := resolveChrootImage(chroot)
			if m.Match == "none" {
				return cerr.New("no_image", cerr.ExitPrecondition, m.Reason)
			}
			if err := rt.Run(cmd.Context(), ctrruntime.RunSpec{
				Image:   m.Image,
				WorkDir: filepath.Dir(spec),
				Mount:   "/sources",
				Env:     []string{"SRPM_ONLY=1", "OUTPUT=/sources/.rpmbuild"},
				Args:    []string{"/usr/bin/rpmbuilder"},
				Stdout:  cmd.OutOrStdout(),
			}); err != nil {
				return cerr.New("srpm_failed", cerr.ExitBuildFailed, "source RPM build failed")
			}
			srpm, err := findSRPM(filepath.Dir(spec))
			if err != nil {
				return err
			}
			result := map[string]any{
				"image":  m.Image,
				"chroot": chroot,
				"spec":   spec,
				"output": srpm,
			}
			return renderResult(cmd, out, result)
		},
	}
	out.bind(cmd)
	cmd.Flags().StringVar(&path, "path", "", "path to the spec directory")
	cmd.Flags().StringVar(&chroot, "chroot", "fedora-rawhide-x86_64", "chroot to build against")
	cmd.Flags().StringVar(&runtimeName, "runtime", "", "container runtime (podman, docker, auto)")
	return cmd
}
