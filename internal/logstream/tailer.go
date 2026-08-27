package logstream

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/events"
)

// Target resolves to a build-chroot log URL.
type Target struct {
	BuildID int
	Chroot  string
	URL     string
}

// Tailer orchestrates concurrent streams for one or more targets, publishing
// log lines onto the shared event bus. It stops when all owning build chroots
// reach a terminal state and a final fetch returns no new bytes, unless Follow
// is set.
type Tailer struct {
	Client     *copr.Client
	Bus        *events.Bus
	Interval   time.Duration
	MaxStreams int
	Follow     bool
}

// NewTailer builds a Tailer.
func NewTailer(c *copr.Client, bus *events.Bus) *Tailer {
	return &Tailer{Client: c, Bus: bus, Interval: 3 * time.Second, MaxStreams: 8}
}

// Locate resolves a build/chroot to its authoritative result_url log.
func (t *Tailer) Locate(ctx context.Context, buildID int, chroot string) (string, error) {
	chroots, err := t.Client.ListBuildChroots(ctx, buildID)
	if err != nil {
		return "", err
	}
	for _, bc := range chroots {
		if bc.Chroot == chroot {
			if bc.ResultURL != "" {
				return strings.TrimRight(bc.ResultURL, "/") + "/builder-live.log.gz", nil
			}
		}
	}
	return "", fmt.Errorf("no result url for build %d chroot %s", buildID, chroot)
}

// ResolveBuildChroots returns the chroot names for a build.
func (t *Tailer) ResolveBuildChroots(ctx context.Context, buildID int) ([]string, error) {
	chroots, err := t.Client.ListBuildChroots(ctx, buildID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(chroots))
	for _, bc := range chroots {
		names = append(names, bc.Chroot)
	}
	return names, nil
}

// Run tails all targets with a bounded worker pool until they are terminal
// and quiet (or forever, when Follow is set).
func (t *Tailer) Run(ctx context.Context, targets []Target) error {
	// A done channel signals streams to drain when the build is terminal.
	done := make(chan struct{})
	defer close(done)

	var wg sync.WaitGroup
	sem := make(chan struct{}, t.MaxStreams)

	// Worker pool: cover every target regardless of the concurrency cap.
	for _, tg := range targets {
		wg.Add(1)
		go func(tg Target) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-done:
				return
			}
			t.runStream(ctx, done, tg)
		}(tg)
	}

	// Terminate streams once the build is terminal (unless following).
	if !t.Follow {
		go func() {
			select {
			case <-ctx.Done():
			case <-t.monitorTerminal(ctx, targets):
				close(done)
			}
		}()
	}

	wg.Wait()
	return nil
}

func (t *Tailer) runStream(ctx context.Context, done chan struct{}, tg Target) {
	s := NewStream(tg.URL, tg.BuildID, tg.Chroot, t.Client.HTTP)
	s.SetInterval(t.Interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
		}
		_, err := s.Fetch(ctx, t.Bus)
		if err != nil && !strings.Contains(err.Error(), "not available yet") {
			t.Bus.Publish(events.Event{Kind: events.KindError, BuildID: tg.BuildID, Chroot: tg.Chroot, Err: err})
		}
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-time.After(t.Interval):
		}
	}
}

// monitorTerminal polls build states and returns when all targets are terminal
// and two quiet passes have completed.
func (t *Tailer) monitorTerminal(ctx context.Context, targets []Target) <-chan struct{} {
	term := make(chan struct{})
	go func() {
		defer close(term)
		quiet := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			allTerminal := true
			for _, tg := range targets {
				b, err := t.Client.GetBuild(ctx, tg.BuildID)
				if err != nil {
					allTerminal = false
					continue
				}
				if st := b.ChrootStates()[tg.Chroot]; !copr.IsTerminal(st) {
					allTerminal = false
				}
			}
			if allTerminal {
				quiet++
				if quiet >= 2 {
					return
				}
			} else {
				quiet = 0
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(t.Interval):
			}
		}
	}()
	return term
}
