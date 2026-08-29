package logstream

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abn/coprctl/internal/events"
)

// gzipBytes compresses data with a single gzip member.
func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

// gzipMulti compresses data as multiple gzip members.
func gzipMulti(parts ...string) []byte {
	var buf bytes.Buffer
	for _, p := range parts {
		w := gzip.NewWriter(&buf)
		_, _ = w.Write([]byte(p))
		_ = w.Close()
	}
	return buf.Bytes()
}

func collect(bus *events.Bus, ch chan events.Event, n int) []events.Event {
	var out []events.Event
	timeout := time.After(3 * time.Second)
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-timeout:
			return out
		}
	}
	return out
}

// TestIncrementalResume tests a multi-member gzip log served with Range
// support: the first fetch gets member 1, the second resumes at the offset
// and gets member 2, the third resumes at the accumulated offset and gets
// member 3, with no torn or duplicated lines across any fetch.
func TestIncrementalResume(t *testing.T) {
	m1 := gzipBytes([]byte("line one\nline two\n"))
	m2 := gzipBytes([]byte("line three\n"))
	m3 := gzipBytes([]byte("line four\nline five\n"))
	full := append(append(append([]byte{}, m1...), m2...), m3...)

	// The server grows the log one member per GET: fetch 1 serves only m1,
	// fetch 2 serves m1+m2, fetch 3 serves all three. Range support returns
	// the raw bytes from the requested offset, so a correct tailer always
	// resumes on a gzip member boundary.
	call := 0
	var served []byte
	var ranges []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			call++
			switch call {
			case 1:
				served = m1
			case 2:
				served = append(append([]byte{}, m1...), m2...)
			default:
				served = full
			}
		}
		rang := r.Header.Get("Range")
		if rang == "" {
			w.Header().Set("Accept-Ranges", "bytes")
			_, _ = w.Write(served)
			return
		}
		var start int
		_, _ = fmt.Sscanf(rang, "bytes=%d-", &start)
		ranges = append(ranges, start)
		if start >= len(served) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(served)-1, len(served)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(served[start:])
	}))
	defer srv.Close()

	bus := events.New()
	defer bus.Close()
	ch := bus.Subscribe(100)
	s := NewStream(srv.URL, 42, "fedora-42-x86_64", srv.Client())
	s.SetInterval(10 * time.Millisecond)

	// First fetch: member 1 only.
	if got, err := s.Fetch(context.Background(), bus); err != nil {
		t.Fatalf("first fetch: %v", err)
	} else if !got {
		t.Fatal("expected new content on first fetch")
	}
	lines := collect(bus, ch, 2)
	if len(lines) != 2 || lines[0].Line != "line one" || lines[1].Line != "line two" {
		t.Fatalf("first fetch lines = %+v", lines)
	}
	// Second fetch resumes at the compressed offset and reads member 2.
	gotNew, err := s.Fetch(context.Background(), bus)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !gotNew {
		t.Fatal("expected new content on second fetch")
	}
	lines = collect(bus, ch, 1)
	if len(lines) != 1 || lines[0].Line != "line three" {
		t.Fatalf("expected exactly 'line three' on resume, got %+v", lines)
	}
	// Third fetch resumes at the accumulated offset (m1+m2) and reads member 3.
	gotNew, err = s.Fetch(context.Background(), bus)
	if err != nil {
		t.Fatalf("third fetch: %v", err)
	}
	if !gotNew {
		t.Fatal("expected new content on third fetch")
	}
	lines = collect(bus, ch, 2)
	if len(lines) != 2 || lines[0].Line != "line four" || lines[1].Line != "line five" {
		t.Fatalf("expected 'line four' and 'line five' on third fetch, got %+v", lines)
	}
	// The range on the third fetch must start at the accumulated offset, not
	// at the size of the last chunk alone.
	if len(ranges) != 3 || ranges[2] != len(m1)+len(m2) {
		t.Fatalf("expected third range to start at %d, got %v", len(m1)+len(m2), ranges)
	}
}

// TestFullRefetchNoDuplicates tests a single-member gzip log where the server
// ignores Range (always returns 200). The tailer must fall back to full
// refetch and skip already-emitted bytes.
func TestFullRefetchNoDuplicates(t *testing.T) {
	log := "alpha\nbeta\ngamma\n"
	gz := gzipBytes([]byte(log))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ignore Range; always return the full body.
		_, _ = w.Write(gz)
	}))
	defer srv.Close()

	bus := events.New()
	defer bus.Close()
	ch := bus.Subscribe(100)
	s := NewStream(srv.URL, 1, "c1", srv.Client())
	s.SetInterval(10 * time.Millisecond)

	if _, err := s.Fetch(context.Background(), bus); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	lines := collect(bus, ch, 3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %+v", len(lines), lines)
	}
	// Second fetch must produce no new lines (all already emitted).
	gotNew, err := s.Fetch(context.Background(), bus)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if gotNew {
		t.Fatal("second fetch should not emit new lines")
	}
}

// TestNoPartialLines ensures a partial trailing line is buffered and never
// emitted until a newline arrives.
func TestNoPartialLines(t *testing.T) {
	gz := gzipBytes([]byte("complete line\npar"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gz)
	}))
	defer srv.Close()

	bus := events.New()
	defer bus.Close()
	ch := bus.Subscribe(100)
	s := NewStream(srv.URL, 1, "c1", srv.Client())

	gotNew, err := s.Fetch(context.Background(), bus)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	_ = gotNew
	lines := collect(bus, ch, 1)
	if len(lines) != 1 {
		t.Fatalf("expected only the complete line, got %d: %+v", len(lines), lines)
	}
	if lines[0].Line != "complete line" {
		t.Errorf("line = %q", lines[0].Line)
	}
}

// TestPlainLogFallback tests that a build with no .gz (older backend) falls
// back to the uncompressed builder-live.log and decodes it directly.
func TestPlainLogFallback(t *testing.T) {
	plain := []byte("line a\nline b\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/builder-live.log.gz":
			http.NotFound(w, r)
		case "/builder-live.log":
			_, _ = w.Write(plain)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	bus := events.New()
	defer bus.Close()
	ch := bus.Subscribe(100)
	s := NewStream(srv.URL+"/builder-live.log.gz", 1, "c1", srv.Client())
	s.SetInterval(10 * time.Millisecond)

	if got, err := s.Fetch(context.Background(), bus); err != nil {
		t.Fatalf("fetch: %v", err)
	} else if !got {
		t.Fatal("expected new content")
	}
	if !s.plain {
		t.Fatal("expected stream to fall back to plain log")
	}
	lines := collect(bus, ch, 2)
	if len(lines) != 2 || lines[0].Line != "line a" || lines[1].Line != "line b" {
		t.Fatalf("unexpected lines: %+v", lines)
	}
}

// TestTornLineAcrossFetches verifies that a partial line split across two
// fetches is emitted once, complete, with no torn halves.
func TestTornLineAcrossFetches(t *testing.T) {
	// First fetch: a complete line plus a partial "hel".
	// Second fetch: the rest "lo\n" completes the line.
	part1 := gzipBytes([]byte("one\ntwo\nhel"))
	part2 := gzipBytes([]byte("lo\nthree\n"))

	call := 0
	full := part1
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			call++
			if call == 2 {
				full = append(append([]byte{}, part1...), part2...)
			}
		}
		rang := r.Header.Get("Range")
		if rang == "" {
			_, _ = w.Write(full)
			return
		}
		var start int
		_, _ = fmt.Sscanf(rang, "bytes=%d-", &start)
		if start >= len(full) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(full[start:])
	}))
	defer srv.Close()

	bus := events.New()
	defer bus.Close()
	ch := bus.Subscribe(100)
	s := NewStream(srv.URL+"/builder-live.log.gz", 1, "c1", srv.Client())
	s.SetInterval(10 * time.Millisecond)

	// First fetch emits "one" and "two" (the "hel" partial is buffered).
	if got, err := s.Fetch(context.Background(), bus); err != nil || !got {
		t.Fatalf("first fetch: got=%v err=%v", got, err)
	}
	lines := collect(bus, ch, 2)
	if len(lines) != 2 || lines[0].Line != "one" || lines[1].Line != "two" {
		t.Fatalf("first fetch lines = %+v", lines)
	}
	// Second fetch completes "hello" and emits "three".
	if got, err := s.Fetch(context.Background(), bus); err != nil || !got {
		t.Fatalf("second fetch: got=%v err=%v", got, err)
	}
	lines = collect(bus, ch, 2)
	if len(lines) != 2 {
		t.Fatalf("second fetch lines = %+v", lines)
	}
	if lines[0].Line != "hello" || lines[1].Line != "three" {
		t.Fatalf("second fetch lines = %+v", lines)
	}
}

// Test416NoNewBytes verifies a 416 response is treated as no-new-bytes.
func Test416NoNewBytes(t *testing.T) {
	gz := gzipBytes([]byte("line\n"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rang := r.Header.Get("Range")
		if rang == "" {
			_, _ = w.Write(gz)
			return
		}
		var start int
		_, _ = fmt.Sscanf(rang, "bytes=%d-", &start)
		if start >= len(gz) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(gz[start:])
	}))
	defer srv.Close()
	bus := events.New()
	defer bus.Close()
	s := NewStream(srv.URL+"/builder-live.log.gz", 1, "c1", srv.Client())
	s.SetInterval(10 * time.Millisecond)
	if got, err := s.Fetch(context.Background(), bus); err != nil || !got {
		t.Fatalf("first fetch: got=%v err=%v", got, err)
	}
	// Second fetch: range past EOF -> 416 -> no new bytes, no error.
	if got, err := s.Fetch(context.Background(), bus); err != nil {
		t.Fatalf("second fetch error: %v", err)
	} else if got {
		t.Fatal("expected no new bytes on 416")
	}
}
