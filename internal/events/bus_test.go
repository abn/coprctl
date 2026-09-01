package events

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/abn/coprctl/internal/copr"
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

func TestPollSourceChrootStates(t *testing.T) {
	buildRaw, err := os.ReadFile("../copr/testdata/build-2926020.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	listRaw, err := os.ReadFile("../copr/testdata/build-chroot-list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api_3/build/2926020":
			w.Write(buildRaw)
		case r.URL.Path == "/api_3/build-chroot/list":
			if r.URL.Query().Get("build_id") != "2926020" {
				t.Errorf("build_id query = %q, want 2926020", r.URL.Query().Get("build_id"))
			}
			w.Write(listRaw)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	bus := New()
	defer bus.Close()
	ch := bus.Subscribe(16)
	src := &PollSource{Client: copr.New(srv.URL, nil), BuildIDs: []int{2926020}}
	src.pollOnce(context.Background(), bus, map[int]map[string]string{})

	got := map[string]string{}
	deadline := time.After(2 * time.Second)
	for got["_build"] == "" || got["epel-9-x86_64"] == "" || got["fedora-rawhide-x86_64"] == "" {
		select {
		case ev := <-ch:
			if ev.Chroot != "" {
				got[ev.Chroot] = ev.State
			} else {
				got["_build"] = ev.State
			}
		case <-deadline:
			t.Fatal("timed out waiting for the expected poll events")
		}
	}
	if got["_build"] != "starting" {
		t.Errorf("build rollup event state = %q, want starting", got["_build"])
	}
	if got["epel-9-x86_64"] != "failed" {
		t.Errorf("chroot state for epel-9-x86_64 = %q, want failed", got["epel-9-x86_64"])
	}
	if got["fedora-rawhide-x86_64"] != "succeeded" {
		t.Errorf("chroot state for fedora-rawhide-x86_64 = %q, want succeeded", got["fedora-rawhide-x86_64"])
	}
}

func TestPollSourceDegradedEmitsBuildOnly(t *testing.T) {
	buildRaw, err := os.ReadFile("../copr/testdata/build-running-chroots.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api_3/build/2926025":
			w.Write(buildRaw)
		case r.URL.Path == "/api_3/build-chroot/list":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	bus := New()
	defer bus.Close()
	ch := bus.Subscribe(16)
	src := &PollSource{Client: copr.New(srv.URL, nil), BuildIDs: []int{2926025}}
	src.pollOnce(context.Background(), bus, map[int]map[string]string{})

	select {
	case ev := <-ch:
		if ev.Kind != KindBuildState || ev.State != "running" {
			t.Errorf("event = %+v, want the build-level rollup running", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the build-level event")
	}
	select {
	case ev := <-ch:
		t.Errorf("unexpected event %+v; a degraded poll must not emit chroot events", ev)
	case <-time.After(100 * time.Millisecond):
	}
}
