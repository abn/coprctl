package copr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/abn/coprctl/internal/cerr"
)

// ProjectCreate is the payload for creating a project. The shared, add+edit,
// and create-only fields are sent only when non-zero, never as implicit zero
// defaults: an explicit auto_prune:false or persistent:false trips the
// upstream admin-only exceptions at create.
type ProjectCreate struct {
	Owner                      string
	Name                       string
	Chroots                    []string
	Description                string
	Instructions               string
	Homepage                   string
	Contact                    string
	DevelMode                  bool
	EnableNet                  bool
	UnlistedOnHomepage         bool
	Persistent                 bool
	Storage                    string
	AutoPrune                  bool
	Bootstrap                  string
	Isolation                  string
	ModuleHotfixes             bool
	Appstream                  bool
	PackitForgeProjectsAllowed []string
	FollowFedoraBranching      bool
	RepoPriority               int
	Multilib                   bool
	FedoraReview               bool
	RuntimeDependencies        []string
	DeleteAfterDays            *int
}

// CreateProject creates a project. existOK suppresses the conflict when the
// project already exists.
func (c *Client) CreateProject(ctx context.Context, in ProjectCreate, existOK bool) error {
	payload := map[string]any{
		"name":         in.Name,
		"chroots":      in.Chroots,
		"description":  in.Description,
		"instructions": in.Instructions,
		"homepage":     in.Homepage,
		"contact":      in.Contact,
		"devel_mode":   in.DevelMode,
		"enable_net":   in.EnableNet,
		// unlisted_on_hp is always sent: the server default matches the zero
		// value, so it never changes a fresh project, unlike the declared-only
		// edit path.
		"unlisted_on_hp": in.UnlistedOnHomepage,
	}
	if in.Persistent {
		payload["persistent"] = true
	}
	if in.Storage != "" {
		payload["storage"] = in.Storage
	}
	if in.AutoPrune {
		payload["auto_prune"] = true
	}
	if in.Bootstrap != "" {
		payload["bootstrap"] = in.Bootstrap
	}
	if in.Isolation != "" {
		payload["isolation"] = in.Isolation
	}
	if in.ModuleHotfixes {
		payload["module_hotfixes"] = true
	}
	if in.Appstream {
		payload["appstream"] = true
	}
	if len(in.PackitForgeProjectsAllowed) > 0 {
		payload["packit_forge_projects_allowed"] = in.PackitForgeProjectsAllowed
	}
	if in.FollowFedoraBranching {
		payload["follow_fedora_branching"] = true
	}
	if in.RepoPriority != 0 {
		payload["repo_priority"] = in.RepoPriority
	}
	if in.Multilib {
		payload["multilib"] = true
	}
	if in.FedoraReview {
		payload["fedora_review"] = true
	}
	if len(in.RuntimeDependencies) > 0 {
		payload["runtime_dependencies"] = in.RuntimeDependencies
	}
	if in.DeleteAfterDays != nil {
		payload["delete_after_days"] = *in.DeleteAfterDays
	}
	err := c.doJSON(ctx, http.MethodPost,
		fmt.Sprintf("/project/add/%s", in.Owner), payload, nil)
	if existOK && isConflict(err) {
		return nil
	}
	return err
}

// ProjectEdit updates project settings. persistent and storage are create-only
// and are never carried here; unlisted_on_hp is editable upstream and belongs
// in the edit set.
type ProjectEdit struct {
	Owner, Project                 string
	Description, Homepage, Contact string
	Instructions                   string
	DevelMode                      *bool
	EnableNet                      *bool
	Chroots                        *[]string
	AutoPrune                      *bool
	Bootstrap                      string
	Isolation                      string
	ModuleHotfixes                 *bool
	Appstream                      *bool
	PackitForgeProjectsAllowed     []string
	FollowFedoraBranching          *bool
	RepoPriority                   int
	UnlistedOnHomepage             *bool
	Multilib                       *bool
	FedoraReview                   *bool
	RuntimeDependencies            []string
	DeleteAfterDays                *int
}

// EditProject updates project settings. Only fields that are non-empty (or
// explicitly set via pointers) are sent, so a partial edit does not clobber
// the others.
func (c *Client) EditProject(ctx context.Context, in ProjectEdit) error {
	payload := map[string]any{}
	if in.Description != "" {
		payload["description"] = in.Description
	}
	if in.Instructions != "" {
		payload["instructions"] = in.Instructions
	}
	if in.Homepage != "" {
		payload["homepage"] = in.Homepage
	}
	if in.Contact != "" {
		payload["contact"] = in.Contact
	}
	if in.DevelMode != nil {
		payload["devel_mode"] = *in.DevelMode
	}
	if in.EnableNet != nil {
		payload["enable_net"] = *in.EnableNet
	}
	if in.Chroots != nil {
		payload["chroots"] = *in.Chroots
	}
	if in.AutoPrune != nil {
		payload["auto_prune"] = *in.AutoPrune
	}
	if in.Bootstrap != "" {
		payload["bootstrap"] = in.Bootstrap
	}
	if in.Isolation != "" {
		payload["isolation"] = in.Isolation
	}
	if in.ModuleHotfixes != nil {
		payload["module_hotfixes"] = *in.ModuleHotfixes
	}
	if in.Appstream != nil {
		payload["appstream"] = *in.Appstream
	}
	if len(in.PackitForgeProjectsAllowed) > 0 {
		payload["packit_forge_projects_allowed"] = in.PackitForgeProjectsAllowed
	}
	if in.FollowFedoraBranching != nil {
		payload["follow_fedora_branching"] = *in.FollowFedoraBranching
	}
	if in.RepoPriority != 0 {
		payload["repo_priority"] = in.RepoPriority
	}
	if in.UnlistedOnHomepage != nil {
		payload["unlisted_on_hp"] = *in.UnlistedOnHomepage
	}
	if in.Multilib != nil {
		payload["multilib"] = *in.Multilib
	}
	if in.FedoraReview != nil {
		payload["fedora_review"] = *in.FedoraReview
	}
	if len(in.RuntimeDependencies) > 0 {
		payload["runtime_dependencies"] = in.RuntimeDependencies
	}
	if in.DeleteAfterDays != nil {
		payload["delete_after_days"] = *in.DeleteAfterDays
	}
	return c.doJSON(ctx, http.MethodPut,
		fmt.Sprintf("/project/edit/%s/%s", in.Owner, in.Project), payload, nil)
}

// DeleteProject removes a project.
func (c *Client) DeleteProject(ctx context.Context, owner, project string) error {
	return c.doJSON(ctx, http.MethodDelete,
		fmt.Sprintf("/project/delete/%s/%s", owner, project), map[string]any{"verify": true}, nil)
}

// RegenerateRepos triggers repository regeneration.
func (c *Client) RegenerateRepos(ctx context.Context, owner, project string) error {
	return c.doJSON(ctx, http.MethodPut,
		fmt.Sprintf("/project/regenerate-repos/%s/%s", owner, project), nil, nil)
}

// ForkProject copies a project.
func (c *Client) ForkProject(ctx context.Context, srcOwner, srcProject, dstOwner, dstProject string) error {
	payload := map[string]any{"projectname": dstProject}
	if dstOwner != "" {
		payload["ownername"] = dstOwner
	}
	return c.doJSON(ctx, http.MethodPost,
		fmt.Sprintf("/project/fork/%s/%s", srcOwner, srcProject), payload, nil)
}

// SourceType enumerates supported source types.
type SourceType string

const (
	SourceSCM       SourceType = "scm"
	SourceDistGit   SourceType = "distgit"
	SourcePyPI      SourceType = "pypi"
	SourceRubyGems  SourceType = "rubygems"
	SourceCustom    SourceType = "custom"
	SourceURL       SourceType = "url"
	SourceUpload    SourceType = "upload"
	SourceRpmUpload SourceType = "rpm-upload"
)

// PackageCreate is the payload for creating or editing a package.
type PackageCreate struct {
	Owner, Project, Name string
	SourceType           SourceType
	Source               map[string]any
	// AutoRebuild is sent on create; on edit it is only sent when
	// SetAutoRebuild is true, so an edit that does not touch the flag does not
	// clobber it.
	AutoRebuild    bool
	SetAutoRebuild bool
	// MaxBuilds and Timeout are write-only through the API (GET does not echo
	// them) and are sent only when non-nil; a zero value is expressible and
	// means the upstream default.
	MaxBuilds *int
	Timeout   *int
	// ChrootDenylist is comma-joined on the wire, matching the upstream
	// cleanup_chroot_denylist filter; a multi-entry list sent as a JSON array
	// would fail the per-item pattern check.
	ChrootDenylist []string
}

// CreatePackage adds a package with a source definition.
func (c *Client) CreatePackage(ctx context.Context, in PackageCreate) error {
	path := fmt.Sprintf("/package/add/%s/%s/%s/%s", in.Owner, in.Project, in.Name, in.SourceType)
	return c.doJSON(ctx, http.MethodPost, path, packagePayload(in), nil)
}

// EditPackage updates a package's source definition.
func (c *Client) EditPackage(ctx context.Context, in PackageCreate) error {
	path := fmt.Sprintf("/package/edit/%s/%s/%s/%s", in.Owner, in.Project, in.Name, in.SourceType)
	return c.doJSON(ctx, http.MethodPost, path, packagePayload(in), nil)
}

func packagePayload(in PackageCreate) map[string]any {
	payload := map[string]any{"package_name": in.Name}
	if in.SetAutoRebuild {
		payload["webhook_rebuild"] = in.AutoRebuild
	}
	if in.MaxBuilds != nil {
		payload["max_builds"] = *in.MaxBuilds
	}
	if in.Timeout != nil {
		payload["timeout"] = *in.Timeout
	}
	if len(in.ChrootDenylist) > 0 {
		payload["chroot_denylist"] = strings.Join(in.ChrootDenylist, ",")
	}
	for k, v := range in.Source {
		payload[k] = v
	}
	return payload
}

// DeletePackage removes a package. The route is the fixed /package/delete
// endpoint, which needs the full owner/project/name body; the old
// /package/delete/{owner}/{project} route does not exist upstream.
func (c *Client) DeletePackage(ctx context.Context, owner, project, name string) error {
	payload := map[string]any{"ownername": owner, "projectname": project, "package_name": name}
	return c.doJSON(ctx, http.MethodDelete, "/package/delete", payload, nil)
}

// ResetPackage clears a package's stored source definition (source_type and
// source_dict), so it reverts to having no configured source. This matches the
// upstream PUT /package/reset operation.
func (c *Client) ResetPackage(ctx context.Context, owner, project, name string) error {
	payload := map[string]any{"ownername": owner, "projectname": project, "package_name": name}
	return c.doJSON(ctx, http.MethodPut, "/package/reset", payload, nil)
}

// BuildSubmit submits a build from a source definition.
type BuildSubmit struct {
	Owner, Project, Package string
	SourceType              SourceType
	Source                  map[string]any
	Chroots                 []string
	Dir                     string
	UploadOptions
}

// UploadOptions carries the generic build options shared by submit and upload.
// A nil pointer or empty value means "not set" and is omitted, so the server
// or project default applies.
type UploadOptions struct {
	Background     *bool
	EnableNet      *bool
	Timeout        *int
	Bootstrap      string
	Isolation      string
	AfterBuildID   *int
	WithBuildID    *int
	ExcludeChroots []string
}

// fill writes the set options into payload under their wire keys.
func (o UploadOptions) fill(payload map[string]any) {
	if o.Background != nil {
		payload["background"] = *o.Background
	}
	if o.EnableNet != nil {
		payload["enable_net"] = *o.EnableNet
	}
	if o.Timeout != nil {
		payload["timeout"] = *o.Timeout
	}
	if o.Bootstrap != "" {
		payload["bootstrap"] = o.Bootstrap
	}
	if o.Isolation != "" {
		payload["isolation"] = o.Isolation
	}
	if o.AfterBuildID != nil {
		payload["after_build_id"] = *o.AfterBuildID
	}
	if o.WithBuildID != nil {
		payload["with_build_id"] = *o.WithBuildID
	}
	if len(o.ExcludeChroots) > 0 {
		payload["exclude_chroots"] = o.ExcludeChroots
	}
}

// SubmitBuild creates a build. The url source type creates one build per pkgs
// entry and returns them all in an items envelope, so it is decoded into a
// BuildList and every build is returned; all other source types return a
// single flat build object, wrapped in a one-element slice.
func (c *Client) SubmitBuild(ctx context.Context, in BuildSubmit) ([]Build, error) {
	payload := map[string]any{
		"ownername":    in.Owner,
		"projectname":  in.Project,
		"chroot_names": in.Chroots,
	}
	if in.Dir != "" {
		payload["project_dirname"] = dirnameFor(in.Project, in.Dir)
	}
	in.UploadOptions.fill(payload)
	for k, v := range in.Source {
		payload[k] = v
	}
	path := fmt.Sprintf("/build/create/%s", in.SourceType)
	if in.SourceType == SourceURL {
		var bl BuildList
		if err := c.doJSON(ctx, http.MethodPost, path, payload, &bl); err != nil {
			return nil, err
		}
		if len(bl.Items) == 0 {
			return nil, cerr.Transport("build submit returned no builds")
		}
		return bl.Items, nil
	}
	var b Build
	if err := c.doJSON(ctx, http.MethodPost, path, payload, &b); err != nil {
		return nil, err
	}
	// An empty 200 yields an all-zero Build; treat it as a transport error so
	// the CLI never reports success for a build that was not created.
	if b.ID == 0 {
		return nil, cerr.Transport("build submit returned an empty build")
	}
	return []Build{b}, nil
}

// dirnameFor builds the full Copr dirname for a project directory. Copr
// validates that the value is "<project>:<dir>" (the project name prefix is
// required), so a bare "testing" becomes "project:testing".
func dirnameFor(project, dir string) string {
	return project + ":" + dir
}

// CancelBuild cancels a build.
func (c *Client) CancelBuild(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/build/cancel/%d", id), nil, nil)
}

// RebuildPackage submits a build for an existing package using its stored
// source definition. The source_dict is resolved and passed to the build
// endpoint. A package cannot be url-sourced (such packages are stored with the
// "link" enum and have no /build/create/<type> route), so there is no envelope
// branch and a single build is always returned.
func (c *Client) RebuildPackage(ctx context.Context, owner, project, pkg string, chroots []string) ([]Build, error) {
	p, err := c.GetPackage(ctx, owner, project, pkg)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"ownername":    owner,
		"projectname":  project,
		"chroot_names": chroots,
	}
	for k, v := range p.SourceDict {
		payload[k] = v
	}
	path := fmt.Sprintf("/build/create/%s", p.SourceType)
	var b Build
	if err := c.doJSON(ctx, http.MethodPost, path, payload, &b); err != nil {
		return nil, err
	}
	if b.ID == 0 {
		return nil, cerr.Transport("build submit returned an empty build")
	}
	return []Build{b}, nil
}

// DeleteBuilds removes a batch of builds in one atomic request. The batch
// list endpoint is the single delete path: if any id is invalid or still
// running, nothing is deleted.
func (c *Client) DeleteBuilds(ctx context.Context, ids []int) error {
	return c.doJSON(ctx, http.MethodPost, "/build/delete/list",
		map[string]any{"builds": ids}, nil)
}

// MockChrootEdit edits a project chroot buildroot configuration.
type MockChrootEdit struct {
	Owner, Project, Chroot string
	AdditionalPackages     []string
	AdditionalRepos        []string
	AdditionalModules      []string
	WithOpts, WithoutOpts  []string
	Isolation              string
}

// EditProjectChroot updates a project chroot configuration.
func (c *Client) EditProjectChroot(ctx context.Context, in MockChrootEdit) error {
	payload := map[string]any{}
	if len(in.AdditionalPackages) > 0 {
		payload["additional_packages"] = in.AdditionalPackages
	}
	if len(in.AdditionalRepos) > 0 {
		payload["additional_repos"] = in.AdditionalRepos
	}
	if len(in.AdditionalModules) > 0 {
		payload["additional_modules"] = in.AdditionalModules
	}
	if len(in.WithOpts) > 0 {
		payload["with_opts"] = in.WithOpts
	}
	if len(in.WithoutOpts) > 0 {
		payload["without_opts"] = in.WithoutOpts
	}
	if in.Isolation != "" {
		payload["isolation"] = in.Isolation
	}
	path := fmt.Sprintf("/project-chroot/edit/%s/%s/%s", in.Owner, in.Project, in.Chroot)
	return c.doJSON(ctx, http.MethodPut, path, payload, nil)
}

// doJSON sends a JSON-encoded request and optionally decodes the response.
func (c *Client) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	contentType := ""
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return cerr.Transport("failed to encode request").Wrap(err)
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	}
	resp, err := c.request(ctx, method, path, nil, body, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode != http.StatusNoContent && resp.ContentLength != 0 {
		return decode(resp, out)
	}
	return nil
}

// UploadBuild uploads a local SRPM and creates a build, optionally into a
// project directory (side repo) named by dir. The generic build options ride
// the multipart json part; the chroot set stays SRPM-declared.
func (c *Client) UploadBuild(ctx context.Context, owner, project, srpmPath, dir string, opts UploadOptions) (*Build, error) {
	f, err := os.Open(srpmPath)
	if err != nil {
		return nil, cerr.Config("cannot open SRPM").Wrap(err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// The upstream client sends a "json" file part carrying the JSON payload,
	// alongside the SRPM as a "pkgs" file part. Without the json part the
	// endpoint rejects the request with HTTP 415.
	form := map[string]any{
		"ownername":   owner,
		"projectname": project,
	}
	if dir != "" {
		form["project_dirname"] = dirnameFor(project, dir)
	}
	opts.fill(form)
	formData, err := json.Marshal(form)
	if err != nil {
		return nil, err
	}
	jsonPart, err := mw.CreateFormFile("json", "json")
	if err != nil {
		return nil, err
	}
	jsonPart.Write(formData)

	pkgsPart, err := mw.CreateFormFile("pkgs", filepath.Base(srpmPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(pkgsPart, f); err != nil {
		return nil, err
	}
	_ = mw.Close()

	resp, err := c.request(ctx, http.MethodPost, "/build/create/upload", nil, &buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var b Build
	if err := decode(resp, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// RpmUploadSubmit publishes an already-built RPM directly into the chosen
// chroots, skipping the SRPM build and dist-git import phases.
type RpmUploadSubmit struct {
	Owner, Project, Dir string
	RpmPath             string
	Chroots             []string
	SHA256              string
}

// UploadRpmBuild sends a multipart request with the RPM as the pkgs file part
// and the form fields (ownername, projectname, project_dirname, chroots,
// sha256) as plain form fields. Unlike UploadBuild there is no json part: the
// endpoint parses the multipart form directly. Chroots repeat per field; an
// empty list sends no chroots field.
func (c *Client) UploadRpmBuild(ctx context.Context, in RpmUploadSubmit) (*Build, error) {
	f, err := os.Open(in.RpmPath)
	if err != nil {
		return nil, cerr.Config("cannot open RPM").Wrap(err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("ownername", in.Owner); err != nil {
		return nil, err
	}
	if err := mw.WriteField("projectname", in.Project); err != nil {
		return nil, err
	}
	if in.Dir != "" {
		if err := mw.WriteField("project_dirname", dirnameFor(in.Project, in.Dir)); err != nil {
			return nil, err
		}
	}
	for _, ch := range in.Chroots {
		if err := mw.WriteField("chroots", ch); err != nil {
			return nil, err
		}
	}
	if in.SHA256 != "" {
		if err := mw.WriteField("sha256", in.SHA256); err != nil {
			return nil, err
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", mime.FormatMediaType("form-data",
		map[string]string{"name": "pkgs", "filename": filepath.Base(in.RpmPath)}))
	h.Set("Content-Type", "application/x-rpm")
	pkgsPart, err := mw.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(pkgsPart, f); err != nil {
		return nil, err
	}
	_ = mw.Close()

	resp, err := c.request(ctx, http.MethodPost, "/build/create/rpm-upload", nil, &buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var b Build
	if err := decode(resp, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// GenerateWebhookSecret rotates a project's webhook secret and returns it.
func (c *Client) GenerateWebhookSecret(ctx context.Context, owner, project string) (string, error) {
	var resp struct {
		WebhookSecret string `json:"webhook_secret"`
	}
	path := fmt.Sprintf("/webhook/generate/%s/%s", owner, project)
	if err := c.doJSON(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.WebhookSecret, nil
}

// NewAPIToken is the response from POST /api_3/api-token.
type NewAPIToken struct {
	APILogin   string `json:"api_login"`
	APIToken   string `json:"api_token"`
	Expiration string `json:"expiration"`
}

// RotateAPIToken requests a new API token using the current credentials.
func (c *Client) RotateAPIToken(ctx context.Context) (*NewAPIToken, error) {
	var out NewAPIToken
	if err := c.doJSON(ctx, http.MethodPost, "/api-token", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpsertPackage creates a package, falling back to an edit when it already
// exists (used by apply, which is additive and safe to re-run). Upstream keeps
// the existing source on edit by merging the stored source_json_dict for keys
// the request omits, so the fallback reaches existing packages without
// clobbering their source.
func (c *Client) UpsertPackage(ctx context.Context, in PackageCreate) error {
	err := c.CreatePackage(ctx, in)
	if err == nil {
		return nil
	}
	if isConflict(err) {
		return c.EditPackage(ctx, in)
	}
	if isBadRequestAlreadyExists(err) {
		return c.EditPackage(ctx, in)
	}
	return err
}

// isConflict reports whether err is a 409 conflict error.
func isConflict(err error) bool {
	var ce *cerr.Error
	if errors.As(err, &ce) {
		return ce.Code == "conflict"
	}
	return false
}

// isBadRequestAlreadyExists reports whether a 400 is the "already exists"
// case, which staging surfaces instead of a 409 for duplicate packages.
func isBadRequestAlreadyExists(err error) bool {
	var ce *cerr.Error
	if errors.As(err, &ce) && ce.Code == "bad_request" {
		return strings.Contains(ce.Hint, "already")
	}
	return false
}
