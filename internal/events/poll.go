package events

import (
	"context"
	"math/rand"
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
		b, err := p.Client.GetBuild(ctx, id)
		if err != nil {
			bus.Publish(Event{Kind: KindError, BuildID: id, Err: err})
			continue
		}
		if p.OnBuild != nil {
			p.OnBuild(b)
		}
		// Whole-build rollup state change.
		rollup := copr.RollupState(b)
		if prev[id] == nil || prev[id]["_rollup"] != rollup {
			bus.Publish(Event{Kind: KindBuildState, BuildID: id, State: rollup,
				Prev: prev[id]["_rollup"]})
		}
		// Per-chroot state changes.
		if prev[id] == nil {
			prev[id] = map[string]string{}
		}
		for chroot, state := range b.ChrootStates() {
			if prev[id][chroot] != state {
				bus.Publish(Event{Kind: KindChrootState, BuildID: id, Chroot: chroot,
					State: state, Prev: prev[id][chroot]})
				prev[id][chroot] = state
			}
		}
		if prev[id]["_rollup"] != rollup {
			prev[id]["_rollup"] = rollup
		}
	}
}

// jitter returns an interval with +/-20% jitter.
func jitter(base time.Duration) time.Duration {
	f := 0.8 + rand.Float64()*0.4 //nolint:gosec // not security-sensitive
	return time.Duration(float64(base) * f)
}

var _ = jitter
