// Package events defines the event bus that drives watch, tail, and monitor.
// There is exactly one implementation of "what is happening"; the TUI, log
// tailer, and JSONL output are all consumers of the same bus.
package events

import (
	"context"
	"sync"
	"time"
)

// Kind is an event category.
type Kind string

const (
	KindBuildState    Kind = "build.state"
	KindChrootState   Kind = "chroot.state"
	KindLogLine       Kind = "log.line"
	KindLogTruncated  Kind = "log.truncated"
	KindBuildFinished Kind = "build.finished"
	KindError         Kind = "error"
)

// Event is a single event on the bus.
type Event struct {
	TS      time.Time
	Kind    Kind
	BuildID int
	Chroot  string
	State   string
	Prev    string
	Stream  string
	Seq     int
	Line    string
	Dropped int
	Source  string
	Err     error
}

// Bus is a fan-out event bus. Sources publish to it; consumers subscribe.
type Bus struct {
	mu     sync.RWMutex
	subs   map[int]chan Event
	nextID int
}

// New creates a Bus.
func New() *Bus {
	return &Bus{subs: map[int]chan Event{}}
}

// Subscribe returns a new subscription channel.
func (b *Bus) Subscribe(buffer int) chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	ch := make(chan Event, buffer)
	b.subs[id] = ch
	return ch
}

// Publish fans an event out to all subscribers, dropping it for slow
// consumers rather than blocking. A drop is flagged on the event so
// downstream log output can report the loss.
func (b *Bus) Publish(ev Event) {
	if ev.TS.IsZero() {
		ev.TS = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Slow consumer: drop rather than block the publisher.
			ev.Dropped = 1
		}
	}
}

// Close closes all subscriptions.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = map[int]chan Event{}
}

// Source produces events for the bus. Implementations include the polling
// source (default) and later the messaging and datagrepper sources.
type Source interface {
	// Run streams events into the bus until the context is cancelled.
	Run(ctx context.Context, bus *Bus)
}
