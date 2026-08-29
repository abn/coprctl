package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/events"
	"github.com/abn/coprctl/internal/logdetective"
	"github.com/abn/coprctl/internal/logstream"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newLogCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Tail and inspect build logs",
	}
	out.bind(cmd)
	cmd.AddCommand(
		newLogTailCmd(app, &out),
		newLogFailuresCmd(app, &out),
		newLogDetectiveCmd(app, &out),
	)
	return cmd
}

func newLogFailuresCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "failures BUILD_ID",
		Short: "Extract the failing region from each failed chroot of a build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Kind != ref.KindBuild {
				return fmt.Errorf("expected a build id, got %q", args[0])
			}
			client, err := app.ReadClient()
			if err != nil {
				return err
			}
			failures, err := logstream.NewTailer(client, nil).ExtractFailures(cmd.Context(), r.BuildID)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				for _, f := range failures {
					fmt.Fprintf(cmd.OutOrStdout(), "== %s (%s)\n", f.Chroot, f.State)
					if f.Head != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", f.Head)
					}
					for _, ln := range f.Lines {
						fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", ln)
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
				return nil
			}
			return renderResult(cmd, out, failures)
		},
	}
	return cmd
}

// consumeEvents drains the bus until done is closed, printing JSONL lines for
// log events and state changes.
func consumeEvents(ctx context.Context, ch chan events.Event, done <-chan struct{}, grep string) {
	for {
		select {
		case <-done:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if grep != "" && ev.Kind == events.KindLogLine && !strings.Contains(ev.Line, grep) {
				continue
			}
			printEvent(ev)
		case <-ctx.Done():
			return
		}
	}
}

func printEvent(ev events.Event) {
	out := map[string]any{
		"schema": "coprctl.event/v1",
		"ts":     ev.TS.Format("2006-01-02T15:04:05Z"),
		"event":  string(ev.Kind),
	}
	if ev.BuildID != 0 {
		out["build_id"] = ev.BuildID
	}
	if ev.Chroot != "" {
		out["chroot"] = ev.Chroot
	}
	if ev.State != "" {
		out["state"] = ev.State
		if ev.Prev != "" {
			out["previous"] = ev.Prev
		}
	}
	if ev.Line != "" {
		out["line"] = ev.Line
		out["stream"] = ev.Stream
		if ev.Seq != 0 {
			out["seq"] = ev.Seq
		}
	}
	if ev.Dropped != 0 {
		out["dropped"] = ev.Dropped
	}
	if ev.Err != nil {
		out["message"] = ev.Err.Error()
	}
	_ = render.Render(os.Stdout, render.FormatJSONL, out)
}

func newLogTailCmd(app *App, out *outFlags) *cobra.Command {
	var follow bool
	var grep string
	cmd := &cobra.Command{
		Use:   "tail TARGET...",
		Short: "Tail build logs (a build id, build/chroot, or ref)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := app.ReadClient()
			if err != nil {
				return err
			}
			tailer := logstream.NewTailer(client, nil)
			// Build the tailer targets from the given references.
			var targets []logstream.Target
			for _, a := range args {
				r, err := ref.Parse(a, nil)
				if err != nil {
					return err
				}
				switch r.Kind {
				case ref.KindBuildChroot:
					url, err := tailer.Locate(cmd.Context(), r.BuildID, r.BuildCht)
					if err != nil {
						return err
					}
					targets = append(targets, logstream.Target{BuildID: r.BuildID, Chroot: r.BuildCht, URL: url})
				case ref.KindBuild:
					chroots, err := tailer.ResolveBuildChroots(cmd.Context(), r.BuildID)
					if err != nil {
						return err
					}
					for _, ch := range chroots {
						url, err := tailer.Locate(cmd.Context(), r.BuildID, ch)
						if err != nil {
							return err
						}
						targets = append(targets, logstream.Target{BuildID: r.BuildID, Chroot: ch, URL: url})
					}
				default:
					return fmt.Errorf("unsupported tail target %q (use a build id or build/chroot)", a)
				}
			}

			bus := events.New()
			defer bus.Close()
			sub := bus.Subscribe(1024)
			done := make(chan struct{})
			defer close(done)
			ctx, cancel := interruptible(cmd.Context())
			defer cancel()

			go consumeEvents(ctx, sub, done, grep)

			tailer = logstream.NewTailer(client, bus)
			tailer.Follow = follow
			tailer.Interval = pollInterval()
			return tailer.Run(ctx, targets)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow the log")
	cmd.Flags().StringVar(&grep, "grep", "", "grep pattern filter")
	return cmd
}

// interruptible returns a context cancelled on SIGINT.
func interruptible(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, func() {
		signal.Stop(ch)
		cancel()
	}
}

// pollInterval returns the default poll interval, overridable via COPRCTL_INTERVAL.
func pollInterval() time.Duration {
	if v := os.Getenv("COPRCTL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 3 * time.Second
}

func newLogDetectiveCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detective BUILD_ID/CHROOT",
		Short: "Ask log-detective.com to explain a failing build log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Kind != ref.KindBuildChroot {
				return fmt.Errorf("expected a build/chroot reference, got %q", args[0])
			}
			ld := logdetective.New()
			expl, err := ld.Explain(cmd.Context(), logdetective.ExplainRequest{
				BuildID: r.BuildID, Chroot: r.BuildCht,
			})
			if err != nil {
				return fmt.Errorf("log-detective: %w (fall back to 'coprctl log failures' for local analysis)", err)
			}
			if isHuman(out.format) {
				fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n%s\n", expl.Summary)
				if expl.Suggestion != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\nSuggestion:\n%s\n", expl.Suggestion)
				}
				return nil
			}
			return renderResult(cmd, out, expl)
		},
	}
	return cmd
}
