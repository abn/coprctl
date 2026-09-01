// Package forge manages forge-side webhook hooks (GitHub and GitLab). The
// GitHub integration needs admin:repo_hook scope only.
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

// Hook is a forge webhook. GitHub reports the destination under config.url and
// its own API URL under url; GitLab reports the destination directly under url.
type Hook struct {
	ID     int64      `json:"id"`
	URL    string     `json:"url"`
	Active bool       `json:"active"`
	Events []string   `json:"events"`
	Config HookConfig `json:"config"`
}

// DestinationURL returns the URL the forge will POST to, independent of how the
// forge reports it.
func (h Hook) DestinationURL() string {
	if h.Config.URL != "" {
		return h.Config.URL
	}
	return h.URL
}

// HookConfig carries the webhook destination and delivery settings.
type HookConfig struct {
	URL string `json:"url"`
}

// HookOptions carries the trigger selection for a hook. Forges translate it
// differently: GitHub maps to an event list, GitLab to the
// push_events/tag_push_events toggles.
type HookOptions struct {
	// TagOnly triggers on tag pushes only (GitHub `create` event; GitLab
	// tag_push_events toggle).
	TagOnly bool
	// Events is an explicit GitHub-style event list. GitLab ignores it.
	Events []string
}

// HookManager is the narrow forge-side surface the integration needs. It keeps
// the Copr receiver URL and package scoping testable independently of any one
// forge client.
type HookManager interface {
	ListHooks(ctx context.Context, owner, repo string) ([]Hook, error)
	CreateHook(ctx context.Context, owner, repo, url string, o HookOptions) (*Hook, error)
	UpdateHook(ctx context.Context, owner, repo string, id int64, url string, o HookOptions) error
	DeleteHook(ctx context.Context, owner, repo string, id int64) error
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

// HookEvents resolves the GitHub event list for the options: an explicit list
// wins, otherwise tag-only maps to the `create` event (which fires on tag
// creation) and branch pushes opt back in with `push`.
func HookEvents(o HookOptions) []string {
	if len(o.Events) > 0 {
		return o.Events
	}
	if o.TagOnly {
		return []string{"create"}
	}
	return []string{"push", "create"}
}

// CreateHook creates a webhook.
func (g *GitHub) CreateHook(ctx context.Context, owner, repo, url string, o HookOptions) (*Hook, error) {
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": HookEvents(o),
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
func (g *GitHub) UpdateHook(ctx context.Context, owner, repo string, id int64, url string, o HookOptions) error {
	payload := map[string]any{
		"active": true,
		"events": HookEvents(o),
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

// PingHook triggers a ping and returns the latest delivery's status code,
// polling deliveries until one appears or the deadline passes.
func (g *GitHub) PingHook(ctx context.Context, owner, repo string, id int64) (int, error) {
	if err := g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/hooks/%d/pings", owner, repo, id), nil, nil); err != nil {
		return 0, err
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var deliveries []struct {
			Status     string `json:"status"`
			StatusCode int    `json:"status_code"`
		}
		if err := g.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/hooks/%d/deliveries", owner, repo, id), nil, &deliveries); err != nil {
			return 0, err
		}
		if len(deliveries) > 0 {
			return deliveries[0].StatusCode, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return 0, fmt.Errorf("no delivery recorded within 15s")
}
