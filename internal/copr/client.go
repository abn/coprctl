// Package copr is the API client for the Copr v3 REST surface (/api_3).
// It is kept behind an interface for testability and supports token and GSSAPI
// auth, internal pagination, and the corrected state set.
package copr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abn/coprctl/internal/cerr"
)

// AuthFunc supplies credentials for a request.
type AuthFunc func(req *http.Request)

// TokenAuth returns an AuthFunc that sets HTTP Basic credentials.
func TokenAuth(login, token string) AuthFunc {
	return func(req *http.Request) {
		req.SetBasicAuth(login, token)
	}
}

// Client is a Copr API client.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	auth    AuthFunc
	ua      string
}

// New builds a client for the given base URL and credentials.
func New(baseURL string, auth AuthFunc) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		auth:    auth,
		ua:      "coprctl/dev",
	}
}

// WithHTTP replaces the HTTP client (for tests).
func (c *Client) WithHTTP(hc *http.Client) *Client { c.HTTP = hc; return c }

// SetUserAgent sets the User-Agent header.
func (c *Client) SetUserAgent(ua string) { c.ua = ua }

// Get performs an authenticated GET and decodes the JSON body into v.
func (c *Client) Get(ctx context.Context, path string, query url.Values, v any) error {
	resp, err := c.request(ctx, http.MethodGet, path, query, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decode(resp, v)
}

// AuthIdentity is the user object returned by the auth-check endpoint.
type AuthIdentity struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// AuthCheck verifies the configured credentials live against the instance and
// returns the authenticated user's identity. A valid token returns nil with
// the identity; an invalid token returns an auth error.
func (c *Client) AuthCheck(ctx context.Context) (*AuthIdentity, error) {
	resp, err := c.request(ctx, http.MethodGet, "/auth-check", nil, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var id AuthIdentity
	if err := decode(resp, &id); err != nil {
		return nil, err
	}
	return &id, nil
}

// request performs an authenticated request to the API. path is relative to
// the /api_3 base; query is appended as URL parameters. Returns the raw
// response; the caller closes the body.
func (c *Client) request(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	u := c.BaseURL + "/api_3" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, cerr.Transport("failed to build request").Wrap(err)
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.send(req)
}

// requestURL performs an authenticated request to an absolute URL, used by
// DownloadFile for non-API downloads. No status mapping is applied; the caller
// decides what an error status means.
func (c *Client) requestURL(ctx context.Context, method, fullURL string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, cerr.Transport("failed to build request").Wrap(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.exec(req)
}

// exec runs a prepared request, applying UA and auth.
func (c *Client) exec(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", c.ua)
	if c.auth != nil {
		c.auth(req)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, cerr.Transport("request failed").Wrap(err)
	}
	return resp, nil
}

// send runs a prepared request, mapping error statuses to structured errors.
func (c *Client) send(req *http.Request) (*http.Response, error) {
	resp, err := c.exec(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, mapHTTPError(resp)
	}
	return resp, nil
}

func decode(resp *http.Response, v any) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return cerr.Transport("failed to read response").Wrap(err)
	}
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, v); err != nil {
		return cerr.Transport("failed to decode response").Wrap(err)
	}
	return nil
}

func mapHTTPError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return cerr.Transport("failed to read error response").Wrap(err)
	}
	apiMsg := apiErrorMessage(body)
	status := resp.StatusCode
	switch status {
	case 400:
		return cerr.New("bad_request", cerr.ExitGeneric, "bad request (400)").
			WithHint(apiMsg).Wrap(apiErr(apiMsg))
	case 401:
		return cerr.Auth("authentication failed (401)").
			WithHint(apiMsg).Wrap(apiErr(apiMsg))
	case 403:
		return cerr.New("permission_denied", cerr.ExitPermission, "permission denied (403)").
			WithHint(apiMsg).Wrap(apiErr(apiMsg))
	case 404:
		e := cerr.NotFound(resp.Request.URL.Path)
		if apiMsg != "" {
			e = e.WithHint(apiMsg)
		}
		return e
	case 409:
		return cerr.New("conflict", cerr.ExitConflict, "conflict (409)").
			WithHint(apiMsg).Wrap(apiErr(apiMsg))
	case 429:
		return cerr.New("rate_limited", cerr.ExitTimeout, "rate limited (429)").WithHint("retry after the Retry-After window")
	case 500, 502, 503, 504:
		return cerr.New("server_error", cerr.ExitTransport, fmt.Sprintf("server error (%d)", status))
	}
	return cerr.New("http_error", cerr.ExitGeneric, fmt.Sprintf("unexpected status %d", status)).
		WithHint(apiMsg)
}

// apiErrorMessage extracts the Copr API "error" or "output" field from a
// JSON error body, if present.
func apiErrorMessage(body []byte) string {
	var e struct {
		Error  string `json:"error"`
		Output string `json:"output"`
	}
	if json.Unmarshal(body, &e) == nil {
		if e.Error != "" {
			return e.Error
		}
		return e.Output
	}
	return strings.TrimSpace(string(body))
}

func apiErr(msg string) error {
	if msg == "" {
		return nil
	}
	return fmt.Errorf("%s", msg)
}

// Meta is the pagination envelope.
type Meta struct {
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	Order     string `json:"order"`
	OrderType string `json:"order_type"`
}
