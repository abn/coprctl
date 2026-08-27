package copr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/abn/coprctl/internal/cerr"
)

// ProjectCreate is the payload for creating a project.
type ProjectCreate struct {
	Owner              string
	Name               string
	Chroots            []string
	Description        string
	Instructions       string
	Homepage           string
	Contact            string
	DevelMode          bool
	EnableNet          bool
	UnlistedOnHomepage bool
}

// CreateProject creates a project. existOK suppresses the conflict when the
// project already exists.
func (c *Client) CreateProject(ctx context.Context, in ProjectCreate, existOK bool) error {
	payload := map[string]any{
		"name":           in.Name,
		"chroots":        in.Chroots,
		"description":    in.Description,
		"instructions":   in.Instructions,
		"homepage":       in.Homepage,
		"contact":        in.Contact,
		"devel_mode":     in.DevelMode,
		"enable_net":     in.EnableNet,
		"unlisted_on_hp": in.UnlistedOnHomepage,
	}
	err := c.doJSON(ctx, http.MethodPost,
		fmt.Sprintf("/project/add/%s", in.Owner), payload, nil)
	if existOK && isConflict(err) {
		return nil
	}
	return err
}

// ProjectEdit updates project settings.
type ProjectEdit struct {
	Owner, Project                 string
	Description, Homepage, Contact string
	DevelMode                      *bool
	EnableNet                      *bool
}

// EditProject updates project settings. Only fields that are non-empty (or
// explicitly set via pointers) are sent, so a partial edit does not clobber
// the others.
func (c *Client) EditProject(ctx context.Context, in ProjectEdit) error {
	payload := map[string]any{}
	if in.Description != "" {
		payload["description"] = in.Description
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
	SourceSCM      SourceType = "scm"
	SourceDistGit  SourceType = "distgit"
	SourcePyPI     SourceType = "pypi"
	SourceRubyGems SourceType = "rubygems"
	SourceCustom   SourceType = "custom"
	SourceURL      SourceType = "url"
	SourceUpload   SourceType = "upload"
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
	for k, v := range in.Source {
		payload[k] = v
	}
	return payload
}

// DeletePackage removes a package.
func (c *Client) DeletePackage(ctx context.Context, owner, project, name string) error {
	payload := map[string]any{"packagename": name}
	path := fmt.Sprintf("/package/delete/%s/%s", owner, project)
	return c.doJSON(ctx, http.MethodDelete, path, payload, nil)
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
}

// SubmitBuild creates a build.
func (c *Client) SubmitBuild(ctx context.Context, in BuildSubmit) (*Build, error) {
	payload := map[string]any{
		"ownername":    in.Owner,
		"projectname":  in.Project,
		"chroot_names": in.Chroots,
	}
	if in.Dir != "" {
		payload["project_dirname"] = in.Dir
	}
	for k, v := range in.Source {
		payload[k] = v
	}
	path := fmt.Sprintf("/build/create/%s", in.SourceType)
	var b Build
	if err := c.doJSON(ctx, http.MethodPost, path, payload, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// CancelBuild cancels a build.
func (c *Client) CancelBuild(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/build/cancel/%d", id), nil, nil)
}

// RebuildPackage submits a build for an existing package using its stored
// source definition. The source_dict is resolved and passed to the build
// endpoint.
func (c *Client) RebuildPackage(ctx context.Context, owner, project, pkg string, chroots []string) (*Build, error) {
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
	return &b, nil
}

// DeleteBuild removes a build.
func (c *Client) DeleteBuild(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/build/delete/%d", id), nil, nil)
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
	u := c.BaseURL + "/api_3" + path
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return cerr.Transport("failed to encode request").Wrap(err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return cerr.Transport("failed to build request").Wrap(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", c.ua)
	if c.auth != nil {
		c.auth(req)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return cerr.Transport("request failed").Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return mapHTTPError(resp)
	}
	if out != nil && resp.StatusCode != http.StatusNoContent && resp.ContentLength != 0 {
		return decode(resp, out)
	}
	return nil
}

// UploadBuild uploads a local SRPM and creates a build.
func (c *Client) UploadBuild(ctx context.Context, owner, project, srpmPath string) (*Build, error) {
	f, err := os.Open(srpmPath)
	if err != nil {
		return nil, cerr.Config("cannot open SRPM").Wrap(err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("pkgs", filepath.Base(srpmPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	_ = mw.Close()

	u := fmt.Sprintf("%s/api_3/build/create/upload?ownername=%s&projectname=%s", c.BaseURL, owner, project)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", c.ua)
	if c.auth != nil {
		c.auth(req)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, cerr.Transport("upload failed").Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, mapHTTPError(resp)
	}
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

// UpsertPackage creates a package, tolerating an already-existing one (used
// by apply, which is additive and safe to re-run).
func (c *Client) UpsertPackage(ctx context.Context, in PackageCreate) error {
	err := c.CreatePackage(ctx, in)
	if err == nil {
		return nil
	}
	if isConflict(err) {
		return nil
	}
	if isBadRequestAlreadyExists(err) {
		return nil
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
