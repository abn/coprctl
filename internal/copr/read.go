package copr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"time"
)

// Project is a Copr project.
type Project struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Ownername    string            `json:"ownername"`
	FullName     string            `json:"full_name"`
	Description  string            `json:"description"`
	Instructions string            `json:"instructions"`
	Homepage     string            `json:"homepage"`
	Contact      string            `json:"contact"`
	DevelMode    bool              `json:"devel_mode"`
	ChrootRepos  map[string]string `json:"chroot_repos"`
}

// ProjectList is the list result.
type ProjectList struct {
	Items []Project `json:"items"`
	Meta  Meta      `json:"meta"`
}

// GetProject fetches a single project.
func (c *Client) GetProject(ctx context.Context, owner, project string) (*Project, error) {
	q := url.Values{}
	q.Set("ownername", owner)
	q.Set("projectname", project)
	var p Project
	if err := c.Get("/project/", q, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListProjects lists projects for an owner, paginating internally.
func (c *Client) ListProjects(ctx context.Context, owner string, limit int) ([]Project, error) {
	if limit <= 0 {
		limit = 100
	}
	var all []Project
	offset := 0
	for {
		q := url.Values{}
		if owner != "" {
			q.Set("ownername", owner)
		}
		q.Set("limit", fmt.Sprintf("%d", limit))
		q.Set("offset", fmt.Sprintf("%d", offset))
		var pl ProjectList
		if err := c.Get("/project/list", q, &pl); err != nil {
			return nil, err
		}
		all = append(all, pl.Items...)
		if len(pl.Items) < limit {
			break
		}
		offset += len(pl.Items)
	}
	return all, nil
}

// SearchProjects searches for projects by query.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]Project, error) {
	q := url.Values{}
	q.Set("query", query)
	var pl ProjectList
	if err := c.Get("/project/search", q, &pl); err != nil {
		return nil, err
	}
	return pl.Items, nil
}

// MockChroots is the chroot catalog: {name: comment}.
type MockChroots map[string]string

// ListMockChroots returns the instance chroot catalog.
func (c *Client) ListMockChroots(ctx context.Context) (MockChroots, error) {
	var m MockChroots
	if err := c.Get("/mock-chroots/list", nil, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ChrootNames returns the sorted names in a catalog.
func (m MockChroots) ChrootNames() []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Build is a Copr build.
type Build struct {
	ID          int       `json:"id"`
	ProjectName string    `json:"projectname"`
	OwnerName   string    `json:"ownername"`
	ProjectDir  string    `json:"project_dirname"`
	PackageName string    `json:"packagename"`
	State       string    `json:"state"`
	SourceType  string    `json:"source_type"`
	Submitted   Timestamp `json:"submitted_on"`
	Started     Timestamp `json:"started_on"`
	Ended       Timestamp `json:"ended_on"`
	RepoURL     string    `json:"repo_url"`
	// Chroots lists the chroot names this build targets.
	Chroots []string `json:"chroots"`
	// Builds maps chroot name to build chroot detail when present.
	Builds map[string]*BuildChroot `json:"builds"`
}

// Timestamp is a Copr timestamp that may arrive as a unix epoch integer or an
// ISO-8601 string depending on the endpoint.
type Timestamp struct {
	Unix  int64
	IsSet bool
}

// UnmarshalJSON accepts a unix epoch number or an ISO-8601 string.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" || s == "" {
		return nil
	}
	t.IsSet = true
	if b[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		tt, err := time.Parse(time.RFC3339, str)
		if err == nil {
			t.Unix = tt.Unix()
		}
		return nil
	}
	return json.Unmarshal(b, &t.Unix)
}

// Time returns the timestamp as a time.Time.
func (t Timestamp) Time() time.Time {
	if !t.IsSet {
		return time.Time{}
	}
	return time.Unix(t.Unix, 0)
}

// BuildChroot is one execution of a build in one chroot.
type BuildChroot struct {
	BuildID   int    `json:"build_id"`
	Chroot    string `json:"name"`
	State     string `json:"state"`
	ResultURL string `json:"result_url"`
	StartedOn string `json:"started_on"`
	EndedOn   string `json:"ended_on"`
}

// GetBuild fetches a single build by id.
func (c *Client) GetBuild(ctx context.Context, id int) (*Build, error) {
	var b Build
	if err := c.Get(fmt.Sprintf("/build/%d", id), nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// BuildList is the list result.
type BuildList struct {
	Items []Build `json:"items"`
	Meta  Meta    `json:"meta"`
}

// ListBuilds lists builds, paginating internally.
func (c *Client) ListBuilds(ctx context.Context, owner, project, pkg string, limit int) ([]Build, error) {
	if limit <= 0 {
		limit = 100
	}
	var all []Build
	offset := 0
	for {
		q := url.Values{}
		if owner != "" {
			q.Set("ownername", owner)
		}
		if project != "" {
			q.Set("projectname", project)
		}
		if pkg != "" {
			q.Set("packagename", pkg)
		}
		q.Set("limit", fmt.Sprintf("%d", limit))
		q.Set("offset", fmt.Sprintf("%d", offset))
		var bl BuildList
		if err := c.Get("/build/list", q, &bl); err != nil {
			return nil, err
		}
		all = append(all, bl.Items...)
		if len(bl.Items) < limit {
			break
		}
		offset += len(bl.Items)
	}
	return all, nil
}

// Package is a Copr package (a source definition, not an RPM).
type Package struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	SourceType  SourceType        `json:"source_type"`
	AutoRebuild bool              `json:"auto_rebuild"`
	SourceDict  map[string]string `json:"source_dict"`
}

// PackageList is the list result.
type PackageList struct {
	Items []Package `json:"items"`
	Meta  Meta      `json:"meta"`
}

// ListPackages lists the packages in a project.
func (c *Client) ListPackages(ctx context.Context, owner, project string) ([]Package, error) {
	q := url.Values{}
	q.Set("ownername", owner)
	q.Set("projectname", project)
	var pl PackageList
	if err := c.Get("/package/list", q, &pl); err != nil {
		return nil, err
	}
	return pl.Items, nil
}

// Terminal build states.
var terminalStates = map[string]bool{
	"succeeded": true,
	"failed":    true,
	"canceled":  true,
	"skipped":   true,
}

// runningStates are the in-progress states.
var runningStates = map[string]bool{
	"starting": true,
	"running":  true,
}

// ChrootStates returns the per-chroot state map for a build, preferring the
// detailed Builds map and falling back to the Chroots name list with the
// build's own state.
func (b *Build) ChrootStates() map[string]string {
	if len(b.Builds) > 0 {
		m := map[string]string{}
		for name, bc := range b.Builds {
			state := ""
			if bc != nil {
				state = bc.State
			}
			m[name] = state
		}
		return m
	}
	m := map[string]string{}
	for _, name := range b.Chroots {
		m[name] = b.State
	}
	return m
}

// IsTerminal reports whether a build state is terminal.
func IsTerminal(state string) bool { return terminalStates[state] }

// IsRunning reports whether a build state is in progress.
func IsRunning(state string) bool { return runningStates[state] }

// RollupState derives a single build-level state from per-chroot states,
// following the documented rollup rules.
func (b *Build) RollupState() string {
	return RollupState(b)
}

// RollupState computes the rollup for a build.
func RollupState(b *Build) string {
	chroots := b.ChrootStates()
	if len(chroots) == 0 {
		return b.State
	}
	hasFailed := false
	hasCanceled := false
	hasRunning := false
	hasPending := false
	for _, state := range chroots {
		switch {
		case state == "failed":
			hasFailed = true
		case state == "canceled":
			hasCanceled = true
		case runningStates[state]:
			hasRunning = true
		case terminalStates[state]:
			// succeeded/skipped
		default:
			hasPending = true
		}
	}
	switch {
	case hasFailed:
		return "failed"
	case hasCanceled:
		return "canceled"
	case hasRunning:
		return "running"
	case hasPending:
		return "pending"
	}
	return "succeeded"
}

// Monitor is one package's per-chroot state row.
type MonitorRow struct {
	Name    string                       `json:"name"`
	Chroots map[string]MonitorChrootInfo `json:"chroots"`
}

// MonitorChrootInfo is a single chroot's state in a monitor row.
type MonitorChrootInfo struct {
	BuildID    int    `json:"build_id"`
	State      string `json:"state"`
	Status     int    `json:"status"`
	PkgVersion string `json:"pkg_version"`
}

// MonitorEnvelope wraps the monitor response.
type MonitorEnvelope struct {
	Output   string       `json:"output"`
	Message  string       `json:"message"`
	Packages []MonitorRow `json:"packages"`
}

// Monitor returns the package x chroot state matrix for a project.
func (c *Client) Monitor(ctx context.Context, owner, project string) ([]MonitorRow, error) {
	q := url.Values{}
	q.Set("ownername", owner)
	q.Set("projectname", project)
	var env MonitorEnvelope
	if err := c.Get("/monitor", q, &env); err != nil {
		return nil, err
	}
	return env.Packages, nil
}

// ListBuildChroots lists the build chroots of a build.
func (c *Client) ListBuildChroots(ctx context.Context, buildID int) ([]BuildChroot, error) {
	q := url.Values{}
	q.Set("build_id", fmt.Sprintf("%d", buildID))
	var l struct {
		Items []BuildChroot `json:"items"`
		Meta  Meta          `json:"meta"`
	}
	if err := c.Get("/build-chroot/list", q, &l); err != nil {
		return nil, err
	}
	return l.Items, nil
}
