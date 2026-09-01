package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// GitLab is a thin client for the subset of the GitLab project webhooks API
// the integration needs: hooks CRUD and test. The endpoints follow the
// documented API (docs.gitlab.com/api/project_webhooks); live behaviour on a
// real instance is pending a check before release.
type GitLab struct {
	Token   string
	HTTP    *http.Client
	baseURL string
}

// NewGitLab builds a GitLab client from a token. apiURL overrides the default
// gitlab.com endpoint for a self-hosted instance and must include the /api/v4
// prefix when given.
func NewGitLab(token, apiURL string) *GitLab {
	if apiURL == "" {
		apiURL = "https://gitlab.com/api/v4"
	}
	return &GitLab{Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}, baseURL: apiURL}
}

// projectPath returns the API path for an owner/repo pair. GitLab accepts the
// URL-encoded namespace path (GROUP/PROJECT) in place of a numeric project id.
func projectPath(owner, repo string) string {
	return "/projects/" + url.PathEscape(owner+"/"+repo)
}

func (g *GitLab) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, rdr)
	if err != nil {
		return 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		rb, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, fmt.Errorf("gitlab %s %s: %d %s", method, path, resp.StatusCode, rb)
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

// ListHooks lists the webhooks on a project.
func (g *GitLab) ListHooks(ctx context.Context, owner, repo string) ([]Hook, error) {
	var hooks []Hook
	_, err := g.do(ctx, http.MethodGet, projectPath(owner, repo)+"/hooks", nil, &hooks)
	return hooks, err
}

// gitlabHookPayload maps the trigger options onto GitLab's boolean toggles.
// GitLab has no event-name list: tag-only is tag_push_events on and
// push_events off; branch pushes opt back in via push_events.
func gitlabHookPayload(o HookOptions, url string) map[string]any {
	return map[string]any{
		"url":                     url,
		"enable_ssl_verification": true,
		"push_events":             !o.TagOnly,
		"tag_push_events":         true,
	}
}

// CreateHook creates a project webhook.
func (g *GitLab) CreateHook(ctx context.Context, owner, repo, url string, o HookOptions) (*Hook, error) {
	var h Hook
	_, err := g.do(ctx, http.MethodPost, projectPath(owner, repo)+"/hooks", gitlabHookPayload(o, url), &h)
	return &h, err
}

// UpdateHook updates a project webhook's destination and toggles.
func (g *GitLab) UpdateHook(ctx context.Context, owner, repo string, id int64, url string, o HookOptions) error {
	_, err := g.do(ctx, http.MethodPut, projectPath(owner, repo)+"/hooks/"+strconv.FormatInt(id, 10), gitlabHookPayload(o, url), nil)
	return err
}

// DeleteHook removes a project webhook.
func (g *GitLab) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	_, err := g.do(ctx, http.MethodDelete, projectPath(owner, repo)+"/hooks/"+strconv.FormatInt(id, 10), nil, nil)
	return err
}

// TestHook triggers a delivery test for the named event toggle and returns the
// request status. GitLab has no GitHub-style delivery readback; the test
// endpoint reports 201 when the delivery is accepted.
func (g *GitLab) TestHook(ctx context.Context, owner, repo string, id int64, trigger string) (int, error) {
	return g.do(ctx, http.MethodPost, projectPath(owner, repo)+"/hooks/"+strconv.FormatInt(id, 10)+"/test/"+trigger, nil, nil)
}
