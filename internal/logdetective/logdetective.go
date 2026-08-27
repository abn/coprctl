// Package logdetective is a thin client for the log-detective.com service,
// which explains a failing Copr build log in plain language.
package logdetective

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// baseURL is the log-detective service endpoint.
const baseURL = "https://log-detective.com/frontend/explain/copr"

// ExplainRequest describes what to explain.
type ExplainRequest struct {
	BuildID int
	Chroot  string
}

// Explanation is the service response.
type Explanation struct {
	// The service returns a plain-language explanation plus a suggestion.
	Summary    string   `json:"summary"`
	Suggestion string   `json:"suggestion"`
	Errors     []string `json:"errors"`
}

// Client calls the log-detective service.
type Client struct {
	HTTP    *http.Client
	baseURL string
}

// New builds a Client.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}, baseURL: baseURL}
}

// Explain asks the service to explain a failing build/chroot log. If the
// service is unreachable or the build is not known to it, it returns a
// non-fatal error so the caller can fall back to local analysis.
func (c *Client) Explain(ctx context.Context, req ExplainRequest) (*Explanation, error) {
	url := fmt.Sprintf("%s/%d/%s", c.baseURL, req.BuildID, req.Chroot)
	body, err := json.Marshal(map[string]any{})
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("User-Agent", "coprctl/0.1")
	resp, err := c.HTTP.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("log-detective returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// The response may be a plain string or a structured object.
	var expl Explanation
	if json.Unmarshal(data, &expl) == nil && (expl.Summary != "" || expl.Suggestion != "") {
		return &expl, nil
	}
	return &Explanation{Summary: string(data)}, nil
}
