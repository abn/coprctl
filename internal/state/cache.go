// Package state holds local, cached, and derived state: the chroot catalog
// cache and later webhook secrets and hook IDs. The chroot cache is what makes
// three-segment reference disambiguation work offline.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/abn/coprctl/internal/copr"
)

// CacheDir returns the per-profile cache directory, honouring XDG_CACHE_HOME.
func CacheDir(profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdg := os.Getenv("XDG_CACHE_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(xdg, "coprctl", profile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// chrootTTL is how long the catalog is cached before refresh.
const chrootTTL = 24 * time.Hour

// ChrootCache caches the mock-chroot catalog for offline disambiguation.
type ChrootCache struct {
	dir string
}

// NewChrootCache creates a cache rooted at dir.
func NewChrootCache(dir string) *ChrootCache { return &ChrootCache{dir: dir} }

type chrootFile struct {
	Updated time.Time        `json:"updated"`
	Chroots copr.MockChroots `json:"chroots"`
}

// Load reads the cache if it is fresh, else returns nil.
func (c *ChrootCache) Load() (*copr.MockChroots, bool) {
	if c == nil || c.dir == "" {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(c.dir, "chroots.json"))
	if err != nil {
		return nil, false
	}
	var cf chrootFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, false
	}
	if time.Since(cf.Updated) > chrootTTL {
		return nil, false
	}
	return &cf.Chroots, true
}

// Store writes the catalog to the cache.
func (c *ChrootCache) Store(ch copr.MockChroots) error {
	if c == nil || c.dir == "" {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}
	cf := chrootFile{Updated: time.Now(), Chroots: ch}
	data, err := json.Marshal(&cf)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, "chroots.json"), data, 0o600)
}

// Clear removes the cached catalog so the next read refetches it.
func (c *ChrootCache) Clear() error {
	if c == nil || c.dir == "" {
		return nil
	}
	err := os.Remove(filepath.Join(c.dir, "chroots.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
