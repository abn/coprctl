package copr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/abn/coprctl/internal/cerr"
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
	EnableNet    bool              `json:"enable_net"`
	ChrootRepos  map[string]string `json:"chroot_repos"`
}

// ProjectList is the list result.
type ProjectList struct {
	Items []Project `json:"items"`
	Meta  Meta      `json:"meta"`
}

// GetProject fetches a single project.
func (c *Client) GetProject(ctx context.Context, owner, project string) (*Project, error) {
	var p Project
	if err := c.Get(ctx, "/project/", projectQuery(owner, project), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// pageSize is the per-request page size for paginated list methods.
const pageSize = 100

// projectQuery builds the common ownername/projectname query params.
func projectQuery(owner, project string) url.Values {
	q := url.Values{}
	if owner != "" {
		q.Set("ownername", owner)
	}
	if project != "" {
		q.Set("projectname", project)
	}
	return q
}

// pageQuery applies the page size and offset to q.
func pageQuery(q url.Values, offset int) {
	q.Set("limit", fmt.Sprintf("%d", pageSize))
	q.Set("offset", fmt.Sprintf("%d", offset))
}

// paginate fetches all pages of T for a query, stopping on an empty page or
// once the total cap is reached. limit <= 0 returns everything; getPage
// performs the request for one page and returns its items.
func paginate[T any](q url.Values, limit int, getPage func(url.Values) ([]T, error)) ([]T, error) {
	var all []T
	offset := 0
	for {
		pageQuery(q, offset)
		items, err := getPage(q)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) == 0 {
			break
		}
		if limit > 0 && len(all) >= limit {
			break
		}
		offset += len(items)
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// ListProjects lists projects for an owner, paginating internally. limit is
// the maximum number of projects to return; limit <= 0 returns everything.
func (c *Client) ListProjects(ctx context.Context, owner string, limit int) ([]Project, error) {
	return paginate(projectQuery(owner, ""), limit, func(q url.Values) ([]Project, error) {
		var pl ProjectList
		if err := c.Get(ctx, "/project/list", q, &pl); err != nil {
			return nil, err
		}
		return pl.Items, nil
	})
}

// SearchProjects searches for projects by query.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]Project, error) {
	q := url.Values{}
	q.Set("query", query)
	var pl ProjectList
	if err := c.Get(ctx, "/project/search", q, &pl); err != nil {
		return nil, err
	}
	return pl.Items, nil
}

// MockChroots is the chroot catalog: {name: comment}.
type MockChroots map[string]string

// ListMockChroots returns the instance chroot catalog.
func (c *Client) ListMockChroots(ctx context.Context) (MockChroots, error) {
	var m MockChroots
	if err := c.Get(ctx, "/mock-chroots/list", nil, &m); err != nil {
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
	ID            int           `json:"id"`
	ProjectName   string        `json:"projectname"`
	OwnerName     string        `json:"ownername"`
	ProjectDir    string        `json:"project_dirname"`
	SourcePackage SourcePackage `json:"source_package"`
	State         string        `json:"state"`
	Submitted     Timestamp     `json:"submitted_on"`
	Started       Timestamp     `json:"started_on"`
	Ended         Timestamp     `json:"ended_on"`
	RepoURL       string        `json:"repo_url"`
	// Chroots lists the chroot names this build targets.
	Chroots []string `json:"chroots"`
	// Builds maps chroot name to build chroot detail. It is enrichment-only,
	// populated via AttachBuildChroots and hidden from the wire.
	Builds map[string]*BuildChroot `json:"-"`
}

// SourcePackage is the source package a build was created from. The wire
// always emits it as {name,url,version}, with null members for project-scoped
// builds.
type SourcePackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

// PackageName returns the source package name, empty for webhook and
// project-scoped builds that have no package.
func (b *Build) PackageName() string { return b.SourcePackage.Name }

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
			return nil
		}
		// Copr emits a handful of timestamp shapes depending on the endpoint;
		// try the common variants before giving up on the value.
		for _, layout := range []string{
			time.RFC3339Nano,
			"2006-01-02T15:04:05-0700",
			"2006-01-02T15:04:05.999999999-0700",
			"2006-01-02 15:04:05",
		} {
			if tt, err := time.Parse(layout, str); err == nil {
				t.Unix = tt.Unix()
				return nil
			}
		}
		return fmt.Errorf("invalid timestamp %q", str)
	}
	return json.Unmarshal(b, &t.Unix)
}

// MarshalJSON emits the unix epoch, matching the API list format.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	if !t.IsSet {
		return []byte("null"), nil
	}
	return json.Marshal(t.Unix)
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
	Chroot    string    `json:"name"`
	State     string    `json:"state"`
	ResultURL string    `json:"result_url"`
	StartedOn Timestamp `json:"started_on"`
	EndedOn   Timestamp `json:"ended_on"`
}

// GetBuild fetches a single build by id.
func (c *Client) GetBuild(ctx context.Context, id int) (*Build, error) {
	var b Build
	if err := c.Get(ctx, fmt.Sprintf("/build/%d", id), nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// BuildList is the list result.
type BuildList struct {
	Items []Build `json:"items"`
	Meta  Meta    `json:"meta"`
}

// ListBuilds lists builds, paginating internally. limit is the maximum
// number of builds to return; limit <= 0 returns everything.
func (c *Client) ListBuilds(ctx context.Context, owner, project, pkg string, limit int) ([]Build, error) {
	q := projectQuery(owner, project)
	if pkg != "" {
		q.Set("packagename", pkg)
	}
	return paginate(q, limit, func(q url.Values) ([]Build, error) {
		var bl BuildList
		if err := c.Get(ctx, "/build/list", q, &bl); err != nil {
			return nil, err
		}
		return bl.Items, nil
	})
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
	var pl PackageList
	if err := c.Get(ctx, "/package/list", projectQuery(owner, project), &pl); err != nil {
		return nil, err
	}
	return pl.Items, nil
}

// GetPackage fetches a single package by name.
func (c *Client) GetPackage(ctx context.Context, owner, project, name string) (*Package, error) {
	pkgs, err := c.ListPackages(ctx, owner, project)
	if err != nil {
		return nil, err
	}
	for i := range pkgs {
		if pkgs[i].Name == name {
			return &pkgs[i], nil
		}
	}
	return nil, cerr.NotFound(fmt.Sprintf("%s/%s/%s", owner, project, name))
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
// detailed Builds map (populated only via AttachBuildChroots) and falling back
// to the Chroots name list with the build's own state.
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

// RollupState derives a single build-level state from the attached per-chroot
// states, following the documented rollup rules. It is a model utility for
// consumers that already hold per-chroot data, not a substitute for the server
// rollup b.State, which is authoritative and may diverge for in-flight and
// mixed states.
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
	var env MonitorEnvelope
	if err := c.Get(ctx, "/monitor", projectQuery(owner, project), &env); err != nil {
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
	if err := c.Get(ctx, "/build-chroot/list", q, &l); err != nil {
		return nil, err
	}
	return l.Items, nil
}

// AttachBuildChroots merges per-chroot detail into b.Builds keyed by chroot
// name. It is the only way Builds gets populated; a bare GetBuild leaves it
// nil and ChrootStates falls back to the Chroots name list.
func (b *Build) AttachBuildChroots(chroots []BuildChroot) {
	if b.Builds == nil {
		b.Builds = map[string]*BuildChroot{}
	}
	for i := range chroots {
		bc := chroots[i]
		b.Builds[bc.Chroot] = &bc
	}
}

// GetBuildDetail fetches a build and attaches its per-chroot detail when
// available. A transient failure of the build-chroot/list request degrades to
// the bare build; a permanent client error fails the call, because a degraded
// fetch would otherwise look like a valid empty chroot set to consumers that
// act on per-chroot data.
func (c *Client) GetBuildDetail(ctx context.Context, id int) (*Build, error) {
	b, err := c.GetBuild(ctx, id)
	if err != nil {
		return nil, err
	}
	chroots, err := c.ListBuildChroots(ctx, id)
	if err != nil {
		if !transientErr(err) {
			return nil, err
		}
		return b, nil
	}
	b.AttachBuildChroots(chroots)
	return b, nil
}

// transientErr reports whether a request failed transiently: a server error,
// a network failure, or a context deadline. Permanent 4xx client errors are
// not transient, so a permanent build-chroot/list failure is never swallowed
// as a degraded fetch.
func transientErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var ce *cerr.Error
	return errors.As(err, &ce) && ce.ExitCode == cerr.ExitTransport
}
