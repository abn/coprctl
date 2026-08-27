// Package config manages instance profiles, the precedence stack, and the
// legacy ~/.config/copr import. Profiles are first-class so the tool never
// assumes a single Fedora instance.
package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/abn/coprctl/internal/cerr"
)

// File is the on-disk configuration (TOML).
type File struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles"`
	Defaults       Defaults           `toml:"defaults"`
}

// Defaults holds command-level defaults shared across profiles.
type Defaults struct {
	Output     string   `toml:"output"`
	Chroots    []string `toml:"chroots"`
	MaxStreams int      `toml:"max_streams"`
	Interval   string   `toml:"interval"`
}

// Profile is a single instance configuration.
type Profile struct {
	URL          string `toml:"url"`
	Username     string `toml:"username"`
	Login        string `toml:"login"`
	Token        string `toml:"token"`
	TokenCommand string `toml:"token_command"`
	GSSAPI       bool   `toml:"gssapi"`
	LogOriginURL string `toml:"log_origin_url"`
	Messaging    string `toml:"messaging"`
	TokenExpiry  string `toml:"token_expiry"`
}

// Precedence names for provenance reporting.
const (
	FromFlag    = "flag"
	FromEnv     = "env"
	FromFile    = "file"
	FromLegacy  = "legacy"
	FromDefault = "default"
)

// Manager loads configuration and exposes the effective profile.
type Manager struct {
	path   string
	legacy string
	file   File
	loaded bool
}

// New creates a Manager for the given config and legacy paths.
func New(configPath, legacyPath string) *Manager {
	return &Manager{path: configPath, legacy: legacyPath}
}

// Matches reports whether the manager's config paths equal the given ones.
func (m *Manager) Matches(configPath, legacyPath string) bool {
	return m.path == configPath && m.legacy == legacyPath
}

// DefaultPaths returns the default config and legacy paths honouring XDG.
func DefaultPaths() (configPath, legacyPath string) {
	home, _ := os.UserHomeDir()
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "coprctl", "config.toml"),
		filepath.Join(home, ".config", "copr")
}

// Load reads the config file if present.
func (m *Manager) Load() error {
	if m.loaded {
		return nil
	}
	if m.file.Profiles == nil {
		m.file.Profiles = map[string]Profile{}
	}
	if data, err := os.ReadFile(m.path); err == nil {
		if err := toml.Unmarshal(data, &m.file); err != nil {
			return cerr.Config("failed to parse config").Wrap(err)
		}
	}
	if m.file.DefaultProfile == "" {
		m.file.DefaultProfile = "default"
	}
	m.loaded = true
	return nil
}

// Profile returns the effective profile by name. When name is empty the default
// profile is used. Missing or incomplete profiles fall back to the legacy
// config file as a compatibility source.
func (m *Manager) Profile(name string) (Profile, error) {
	if err := m.Load(); err != nil {
		return Profile{}, err
	}
	if name == "" {
		name = m.file.DefaultProfile
	}
	p, ok := m.file.Profiles[name]
	legacy, legacyErr := m.loadLegacy()
	if !ok {
		if legacyErr != nil {
			return Profile{}, cerr.New("no_config", cerr.ExitNoConfig,
				fmt.Sprintf("no configuration found for profile %q", name)).WithHint(
				"run 'coprctl config migrate' to import a legacy config")
		}
		return legacy, nil
	}
	if legacyErr == nil {
		p = mergeLegacy(p, legacy)
	}
	return p, nil
}

// ProfileNames returns configured profile names (excluding legacy-only).
func (m *Manager) ProfileNames() []string {
	_ = m.Load()
	names := make([]string, 0, len(m.file.Profiles)+1)
	names = append(names, m.file.DefaultProfile)
	for n := range m.file.Profiles {
		if n != m.file.DefaultProfile {
			names = append(names, n)
		}
	}
	return names
}

// DefaultProfileName returns the configured default profile name.
func (m *Manager) DefaultProfileName() string {
	_ = m.Load()
	return m.file.DefaultProfile
}

// Defaults returns the file defaults.
func (m *Manager) Defaults() Defaults { return m.file.Defaults }

// LoadLegacy parses the legacy [copr-cli] config file. It is read-only
// compatibility; the tool never writes to it.
func (m *Manager) LoadLegacy() (Profile, error) { return m.loadLegacy() }

func (m *Manager) loadLegacy() (Profile, error) {
	f, err := os.Open(m.legacy)
	if err != nil {
		return Profile{}, err
	}
	defer f.Close()

	p := Profile{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		switch key {
		case "login":
			p.Login = val
		case "username":
			p.Username = val
		case "token":
			p.Token = val
		case "copr_url":
			p.URL = strings.TrimRight(val, "/")
		case "expiration date":
			p.TokenExpiry = val
		}
	}
	if err := sc.Err(); err != nil {
		return Profile{}, err
	}
	if p.URL == "" {
		return Profile{}, cerr.Config("legacy config is missing copr_url")
	}
	return p, nil
}

func mergeLegacy(primary, legacy Profile) Profile {
	if primary.URL == "" {
		primary.URL = legacy.URL
	}
	if primary.Login == "" {
		primary.Login = legacy.Login
	}
	if primary.Token == "" {
		primary.Token = legacy.Token
	}
	if primary.Username == "" {
		primary.Username = legacy.Username
	}
	if primary.TokenExpiry == "" {
		primary.TokenExpiry = legacy.TokenExpiry
	}
	return primary
}

// Auth returns the (login, token) credentials for the profile. A token_command
// takes precedence over an inline token.
func (p Profile) Auth() (string, string) {
	if p.TokenCommand != "" {
		if tok, err := runTokenCommand(p.TokenCommand); err == nil && tok != "" {
			return p.Login, tok
		}
	}
	return p.Login, p.Token
}

// BaseURL returns the base URL trimmed of a trailing slash.
func (p Profile) BaseURL() string { return strings.TrimRight(p.URL, "/") }

func runTokenCommand(cmd string) (string, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", cerr.Config("empty token_command")
	}
	out, err := exec.Command(parts[0], parts[1:]...).Output()
	if err != nil {
		return "", cerr.Config("token_command failed").Wrap(err)
	}
	return strings.TrimSpace(string(out)), nil
}
