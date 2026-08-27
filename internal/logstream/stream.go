// Package logstream implements the log tailer: resolve a target to build
// chroots, locate the live log, probe the gzip framing, and fetch incrementally
// or by full refetch. It emits log lines and truncation markers onto the shared
// event bus.
package logstream

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abn/coprctl/internal/events"
)

// Stream is a single tail of one build chroot's log.
type Stream struct {
	URL     string
	BuildID int
	Chroot  string

	http      *http.Client
	origin    string // non-CDN host when configured, else empty
	emitted   int64  // decompressed bytes already emitted
	offset    int64  // compressed byte offset (incremental mode)
	resumable bool   // whether incremental resume works
	probed    bool
	isGzip    bool   // whether the active log is gzip-compressed
	plain     bool   // fell back to the uncompressed .log
	pending   string // partial trailing line held across fetches
	seq       int    // monotonic per-stream line sequence
	interval  time.Duration
}

// NewStream creates a stream for a log URL.
func NewStream(url string, buildID int, chroot string, hc *http.Client) *Stream {
	return &Stream{URL: url, BuildID: buildID, Chroot: chroot, http: hc,
		interval: 3 * time.Second, resumable: true, isGzip: true}
}

// SetOrigin sets the non-CDN origin host to prefer for live tailing.
func (s *Stream) SetOrigin(origin string) { s.origin = strings.TrimRight(origin, "/") }

// SetInterval sets the poll interval.
func (s *Stream) SetInterval(d time.Duration) { s.interval = d }

// Fetch pulls the latest log bytes and emits new lines to the bus. It returns
// true while the log may still grow.
func (s *Stream) Fetch(ctx context.Context, bus *events.Bus) (bool, error) {
	if !s.probed {
		if err := s.probe(); err != nil {
			return false, err
		}
		s.probed = true
	}
	// Plain (uncompressed) logs are read directly.
	if s.plain {
		return s.fetchPlain(ctx, bus)
	}
	// In incremental mode, request the tail of the compressed file.
	if s.resumable {
		gotNew, err := s.fetchIncremental(ctx, bus)
		if err == nil {
			return gotNew, nil
		}
		// Fall back to full refetch on any resume failure.
		s.resumable = false
	}
	return s.fetchFull(ctx, bus)
}

func (s *Stream) url() string {
	if s.origin != "" {
		// Replace the host of URL with the origin host.
		if i := strings.Index(s.URL, "://"); i > 0 {
			rest := s.URL[i+3:]
			if j := strings.Index(rest, "/"); j > 0 {
				return s.origin + rest[j:]
			}
		}
	}
	return s.URL
}

// probe determines whether the log exists and which framing it uses. Logs may
// not exist until the build starts; the caller retries.
func (s *Stream) probe() error {
	// Prefer the gzip log; fall back to the uncompressed log for older
	// backends that never gzipped builder-live.log.
	candidates := []string{s.url()}
	if strings.HasSuffix(s.url(), ".gz") {
		candidates = append(candidates, strings.TrimSuffix(s.url(), ".gz"))
	}
	for i, u := range candidates {
		req, err := http.NewRequest(http.MethodHead, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := s.http.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode >= 400 {
			continue
		}
		// Found a candidate.
		if i > 0 {
			// Fell back to the uncompressed log.
			s.plain = true
			s.isGzip = false
			s.URL = u
		}
		return nil
	}
	return fmt.Errorf("log not available yet")
}

func (s *Stream) fetchIncremental(ctx context.Context, bus *events.Bus) (bool, error) {
	if s.plain || !s.isGzip {
		return false, fmt.Errorf("not gzip, use full refetch")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", s.offset))
	resp, err := s.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusRequestedRangeNotSatisfiable:
		// 416: no new bytes.
		return false, nil
	case http.StatusPartialContent:
		// Check the first two bytes are the gzip magic; if not, this stream
		// is not resumable.
		head := make([]byte, 2)
		if _, err := io.ReadFull(resp.Body, head); err != nil {
			return false, err
		}
		if !(head[0] == 0x1f && head[1] == 0x8b) {
			s.resumable = false
			return false, fmt.Errorf("not a gzip member boundary")
		}
		// body read is handled by tracked below
		// Track the compressed offset by counting actual bytes read from the
		// body, since ContentLength is unreliable for chunked responses.
		tracked := &offsetReader{src: resp.Body, base: s.offset}
		dec, err := gzip.NewReader(io.MultiReader(bytes.NewReader(head), tracked))
		if err != nil {
			return false, err
		}
		gotNew, err := s.emitReader(dec, 0, bus)
		s.offset = tracked.total + 2
		return gotNew, err
	default:
		// 200 means the server ignored Range; switch to full refetch.
		s.resumable = false
		return false, fmt.Errorf("server ignored Range")
	}
}

func (s *Stream) fetchFull(ctx context.Context, bus *events.Bus) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := s.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("log fetch failed: %d", resp.StatusCode)
	}
	// A full refetch re-reads from the partial line's start, so drop any
	// buffered partial to avoid double-buffering it.
	s.pending = ""
	// Guard against transparent decompression: if the server decoded the gzip
	// for us (Content-Encoding: gzip on a .gz resource), read plain.
	if s.isGzip && !resp.Uncompressed {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return false, err
		}
		return s.emitReader(gz, s.emitted, bus)
	}
	return s.emitReader(resp.Body, s.emitted, bus)
}

// fetchPlain reads an uncompressed log, skipping already-emitted bytes.
func (s *Stream) fetchPlain(ctx context.Context, bus *events.Bus) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := s.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("log fetch failed: %d", resp.StatusCode)
	}
	s.pending = ""
	return s.emitReader(resp.Body, s.emitted, bus)
}

// emitReader decodes the gzip stream and emits new lines. skip is the number
// of decompressed bytes to drop (used by full refetch to avoid re-emitting
// content already shown by prior incremental fetches). A trailing partial line
// is buffered and only emitted once it ends in a newline.
func (s *Stream) emitReader(gz io.Reader, skip int64, bus *events.Bus) (bool, error) {
	// Skip decompressed bytes already emitted in prior fetches.
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, gz, skip); err != nil {
			return false, err
		}
	}
	br := bufio.NewReaderSize(gz, 64*1024)
	gotNew := false
	for {
		line, err := br.ReadString('\n')
		complete := strings.HasSuffix(line, "\n")
		// Combine with any pending partial from a prior fetch.
		if s.pending != "" {
			line = s.pending + line
			s.pending = ""
		}
		if complete {
			line = strings.TrimSuffix(line, "\n")
			if line != "" {
				s.emitted += int64(len(line) + 1)
				gotNew = true
				s.seq++
				bus.Publish(events.Event{
					Kind: events.KindLogLine, BuildID: s.BuildID, Chroot: s.Chroot,
					Stream: "builder-live", Seq: s.seq, Line: line,
				})
			}
		} else if len(line) > 0 {
			// Partial line at EOF: buffer it for the next fetch.
			s.pending = line
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return gotNew, err
		}
	}
	return gotNew, nil
}

// offsetReader counts bytes read from a source, offset by a base.
type offsetReader struct {
	src   io.Reader
	total int64
	base  int64
}

func (o *offsetReader) Read(p []byte) (int, error) {
	n, err := o.src.Read(p)
	o.total += int64(n)
	return n, err
}
