package events

import (
	"context"
	"time"

	"github.com/abn/coprctl/internal/copr"
)

// PollSource watches one or more build IDs and publishes state-change events
// to the bus. It is the default, always-available event source.
type PollSource struct {
	Client   *copr.Client
	BuildIDs []int
	Interval time.Duration     // base poll interval
	OnBuild  func(*copr.Build) // optional callback with the latest build
}

// Run polls the builds until the context is cancelled.
func (p *PollSource) Run(ctx context.Context, bus *Bus) {
	interval := p.Interval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	prev := map[int]map[string]string{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx, bus, prev)
		}
	}
}

func (p *PollSource) pollOnce(ctx context.Context, bus *Bus, prev map[int]map[string]string) {
	for _, id := range p.BuildIDs {
		b, err := p.Client.GetBuildDetail(ctx, id)
		if err != nil {
			bus.Publish(Event{Kind: KindError, BuildID: id, Err: err})
			continue
		}
		if p.OnBuild != nil {
			p.OnBuild(b)
		}
		if prev[id] == nil {
			prev[id] = map[string]string{}
		}
		// Whole-build server rollup state change.
		if prev[id]["_state"] != b.State {
			bus.Publish(Event{Kind: KindBuildState, BuildID: id, State: b.State,
				Prev: prev[id]["_state"]})
			prev[id]["_state"] = b.State
		}
		// On a degraded fetch the per-chroot fallback maps every chroot name to
		// the build rollup, so emitting it would re-publish chroot events that
		// flip with the rollup each poll. Emit build-level state only.
		if b.Builds == nil {
			continue
		}
		// Per-chroot state changes.
		for chroot, state := range b.ChrootStates() {
			if prev[id][chroot] != state {
				bus.Publish(Event{Kind: KindChrootState, BuildID: id, Chroot: chroot,
					State: state, Prev: prev[id][chroot]})
				prev[id][chroot] = state
			}
		}
	}
}
