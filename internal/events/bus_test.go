package events

import (
	"testing"
	"time"
)

func TestBusFanOut(t *testing.T) {
	b := New()
	defer b.Close()
	a := b.Subscribe(10)
	c := b.Subscribe(10)
	b.Publish(Event{Kind: KindLogLine, Line: "hello"})
	select {
	case ev := <-a:
		if ev.Kind != KindLogLine || ev.Line != "hello" {
			t.Errorf("subscriber a got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber a timed out")
	}
	select {
	case ev := <-c:
		if ev.Line != "hello" {
			t.Errorf("subscriber c got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber c timed out")
	}
}

func TestBusDropSlowConsumer(t *testing.T) {
	b := New()
	defer b.Close()
	// A tiny buffer subscriber that never drains should not block the
	// publisher.
	_ = b.Subscribe(0)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			b.Publish(Event{Kind: KindLogLine, Seq: i})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked on slow consumer")
	}
}

func TestEventTimestampDefault(t *testing.T) {
	b := New()
	defer b.Close()
	ch := b.Subscribe(1)
	b.Publish(Event{Kind: KindError})
	ev := <-ch
	if ev.TS.IsZero() {
		t.Fatal("expected timestamp to be set")
	}
}
