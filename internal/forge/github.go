// Package forge manages forge-side webhook hooks (GitHub first, others later).
// The integration needs admin:repo_hook scope only.
package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Hook is a forge webhook.
type Hook struct {
	ID     int64    `json:"id"`
	URL    string   `json:"url"`
	Active bool     `json:"active"`
	Events []string `json:"events"`
}

// GitHub is a thin client for the subset of the GitHub API the integration
// needs: hooks CRUD, ping, and delivery checks.
type GitHub struct {
	Token   string
	HTTP    *http.Client
	baseURL string
}

// NewGitHub builds a GitHub client from a token.
func NewGitHub(token string) *GitHub {
	return &GitHub{Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}, baseURL: "https://api.github.com"}
}

func (g *GitHub) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		rb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github %s %s: %d %s", method, path, resp.StatusCode, rb)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ListHooks lists the hooks on a repo.
func (g *GitHub) ListHooks(ctx context.Context, owner, repo string) ([]Hook, error) {
	var hooks []Hook
	err := g.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/hooks", owner, repo), nil, &hooks)
	return hooks, err
}

// CreateHook creates a webhook.
func (g *GitHub) CreateHook(ctx context.Context, owner, repo, url string, events []string) (*Hook, error) {
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": events,
		"config": map[string]any{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
		},
	}
	var h Hook
	err := g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/hooks", owner, repo), payload, &h)
	return &h, err
}

// UpdateHook updates a hook's config and events.
func (g *GitHub) UpdateHook(ctx context.Context, owner, repo string, id int64, url string, events []string) error {
	payload := map[string]any{
		"active": true,
		"events": events,
		"config": map[string]any{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
		},
	}
	return g.do(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/%s/hooks/%d", owner, repo, id), payload, nil)
}

// DeleteHook removes a hook.
func (g *GitHub) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	return g.do(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/%s/hooks/%d", owner, repo, id), nil, nil)
}

// PingHook triggers a ping and returns the latest delivery's status code.
func (g *GitHub) PingHook(ctx context.Context, owner, repo string, id int64) (int, error) {
	if err := g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/hooks/%d/pings", owner, repo, id), nil, nil); err != nil {
		return 0, err
	}
	// Give the delivery a moment to be recorded, then read it back.
	time.Sleep(2 * time.Second)
	var deliveries []struct {
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
	}
	if err := g.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/hooks/%d/deliveries", owner, repo, id), nil, &deliveries); err != nil {
		return 0, err
	}
	if len(deliveries) == 0 {
		return 0, fmt.Errorf("no delivery recorded")
	}
	return deliveries[0].StatusCode, nil
}
