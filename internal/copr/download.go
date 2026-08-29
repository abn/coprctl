package copr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/abn/coprctl/internal/cerr"
)

// ErrFileNotFound marks a download target that returned HTTP 404. Callers
// treat it as a warning and continue rather than aborting a download run.
var ErrFileNotFound = errors.New("download: file not found")

// BuiltPackage is one built RPM's NEVRA as reported by the built-packages
// endpoint.
type BuiltPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
	Epoch   string `json:"epoch"`
}

// BuiltPackages maps chroot name to the list of packages built there.
type BuiltPackages map[string]struct {
	Packages []BuiltPackage `json:"packages"`
}

// GetBuiltPackages lists the packages a build produced, per chroot.
func (c *Client) GetBuiltPackages(ctx context.Context, buildID int) (BuiltPackages, error) {
	var bp BuiltPackages
	if err := c.Get(ctx, fmt.Sprintf("/build/built-packages/%d", buildID), nil, &bp); err != nil {
		return nil, err
	}
	return bp, nil
}

// BuiltPackageFilename derives the RPM filename for a built package.
func BuiltPackageFilename(p BuiltPackage) string {
	return fmt.Sprintf("%s-%s-%s.%s.rpm", p.Name, p.Version, p.Release, p.Arch)
}

// DownloadFile fetches url into destPath using the client's HTTP transport
// with auth applied, mirroring UploadBuild. A 404 is returned as
// ErrFileNotFound so the caller can warn and continue.
func (c *Client) DownloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return cerr.Transport("failed to build request").Wrap(err)
	}
	req.Header.Set("User-Agent", c.ua)
	if c.auth != nil {
		c.auth(req)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return cerr.Transport("download failed").Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrFileNotFound
	}
	if resp.StatusCode >= 400 {
		return cerr.Transport(fmt.Sprintf("download failed: %d", resp.StatusCode))
	}
	out, err := os.Create(destPath)
	if err != nil {
		return cerr.Config("cannot create file").Wrap(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return cerr.Transport("write failed").Wrap(err)
	}
	return nil
}
