package logstream

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// FailureRegion is the extracted failing region of a build-chroot log.
type FailureRegion struct {
	BuildID int      `json:"build_id"`
	Chroot  string   `json:"chroot"`
	State   string   `json:"state"`
	Head    string   `json:"head,omitempty"`
	Lines   []string `json:"lines"`
}

// ExtractFailures fetches the log for each failed chroot of a build and
// extracts a bounded failing region using ordered heuristics. It is the
// first thing an agent should call when a build fails.
func (t *Tailer) ExtractFailures(ctx context.Context, buildID int) ([]FailureRegion, error) {
	chroots, err := t.Client.ListBuildChroots(ctx, buildID)
	if err != nil {
		return nil, err
	}
	var failures []FailureRegion
	for _, bc := range chroots {
		if !isFailed(bc.State) {
			continue
		}
		url := strings.TrimRight(bc.ResultURL, "/") + "/builder-live.log.gz"
		region, err := extractRegion(ctx, t.Client.HTTP, url, buildID, bc.Chroot, bc.State)
		if err != nil {
			// Record the fetch error so the agent knows the log was not read.
			region = FailureRegion{BuildID: buildID, Chroot: bc.Chroot, State: bc.State,
				Head: fmt.Sprintf("could not fetch log: %v", err)}
		}
		failures = append(failures, region)
	}
	return failures, nil
}

func isFailed(state string) bool {
	return state == "failed"
}

// extractRegion downloads and decodes the log, then extracts the failing
// region using ordered heuristics: error markers, failed build dependencies,
// and the region around the final failure marker.
func extractRegion(ctx context.Context, hc *http.Client, url string, buildID int, chroot, state string) (FailureRegion, error) {
	lines, err := fetchLines(ctx, hc, url)
	if err != nil {
		return FailureRegion{}, err
	}
	region := FailureRegion{BuildID: buildID, Chroot: chroot, State: state}
	// Find the last failure marker.
	markers := []string{"error:", "Error:", "Failed build dependencies:", "FAILED", "Copr build error:"}
	idx := -1
	for i, ln := range lines {
		for _, m := range markers {
			if strings.Contains(ln, m) {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		// No marker found; show the tail.
		region.Head = "no failure marker found; showing tail"
		start := len(lines) - 40
		if start < 0 {
			start = 0
		}
		region.Lines = lines[start:]
		return region, nil
	}
	// Show up to 10 lines before the marker through 15 after.
	start := idx - 10
	if start < 0 {
		start = 0
	}
	end := idx + 15
	if end > len(lines) {
		end = len(lines)
	}
	region.Head = fmt.Sprintf("failure near line %d of %d", idx+1, len(lines))
	region.Lines = lines[start:end]
	return region, nil
}

// fetchLines downloads and decodes a possibly-gzip build log into lines.
func fetchLines(ctx context.Context, hc *http.Client, url string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("log fetch failed: %d", resp.StatusCode)
	}
	var r io.Reader = resp.Body
	if strings.HasSuffix(url, ".gz") && !resp.Uncompressed {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	return lines, sc.Err()
}
