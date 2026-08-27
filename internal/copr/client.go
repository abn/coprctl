// Package copr is the API client for the Copr v3 REST surface (/api_3).
// It is kept behind an interface for testability and supports token and GSSAPI
// auth, internal pagination, and the corrected state set.
package copr

import (
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
func (c *Client) Get(path string, query url.Values, v any) error {
	resp, err := c.do("GET", path, query, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decode(resp, v)
}

// Do performs a request and returns the raw response. Caller closes the body.
func (c *Client) do(method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	u := c.BaseURL + "/api_3" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, cerr.Transport("failed to build request").Wrap(err)
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	if c.auth != nil {
		c.auth(req)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, cerr.Transport("request failed").Wrap(err)
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
	body, _ := io.ReadAll(resp.Body)
	apiMsg := apiErrorMessage(body)
	status := resp.StatusCode
	switch status {
	case 400:
		return cerr.New("bad_request", cerr.ExitGeneric, "bad request (400)").
			WithHint(apiMsg).Wrap(apiErr(apiMsg))
	case 401:
		return cerr.Auth("authentication failed (401)").Wrap(fmt.Errorf("%s", body))
	case 403:
		return cerr.New("permission_denied", cerr.ExitPermission, "permission denied (403)")
	case 404:
		return cerr.NotFound(resp.Request.URL.Path)
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
