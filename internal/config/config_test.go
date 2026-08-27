package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
)

func writeLegacy(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "copr")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProfileFromLegacyOnly(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.toml")
	legacy := writeLegacy(t, `[copr-cli]
login = "abc123"
username = "devnullcake"
token = "sekrit"
copr_url = "https://copr.stg.fedoraproject.org"
# expiration date: 2027-02-23
`)
	m := New(cfg, legacy)
	p, err := m.Profile("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.URL != "https://copr.stg.fedoraproject.org" {
		t.Errorf("URL = %q", p.URL)
	}
	if p.Username != "devnullcake" {
		t.Errorf("Username = %q", p.Username)
	}
	if p.Login != "abc123" || p.Token != "sekrit" {
		t.Errorf("auth fields not parsed: login=%q token=%q", p.Login, p.Token)
	}
	login, token := p.Auth()
	if login != "abc123" || token != "sekrit" {
		t.Errorf("Auth() = (%q, %q)", login, token)
	}
}

func TestProfileFromFileWithLegacyFallback(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte(`default_profile = "fedora"
[profiles.fedora]
url = "https://copr.fedorainfracloud.org"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := writeLegacy(t, `[copr-cli]
login = "abc123"
token = "sekrit"
copr_url = "https://copr.stg.fedoraproject.org"
`)
	m := New(cfg, legacy)
	p, err := m.Profile("fedora")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// URL comes from the profile file; auth falls back to legacy.
	if p.URL != "https://copr.fedorainfracloud.org" {
		t.Errorf("URL = %q, want profile value", p.URL)
	}
	login, token := p.Auth()
	if login != "abc123" || token != "sekrit" {
		t.Errorf("Auth() = (%q,%q), want legacy fallback", login, token)
	}
}

func TestNoConfigError(t *testing.T) {
	m := New(filepath.Join(t.TempDir(), "nope.toml"), filepath.Join(t.TempDir(), "nope"))
	_, err := m.Profile("missing")
	if err == nil {
		t.Fatalf("expected error for missing profile")
	}
	if cerr.ExitCodeFor(err) != 3 {
		t.Errorf("expected exit code 3, got %d", cerr.ExitCodeFor(err))
	}
}

func TestLegacyAutoDetectFallback(t *testing.T) {
	// No coprctl config; only a legacy file. Profile() must fall back to it.
	cfg := filepath.Join(t.TempDir(), "config.toml")
	legacy := writeLegacy(t, `[copr-cli]
login = "abc123"
username = "devnullcake"
token = "sekrit"
copr_url = "https://copr.fedorainfracloud.org"
`)
	m := New(cfg, legacy)
	p, err := m.Profile("")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if p.URL != "https://copr.fedorainfracloud.org" {
		t.Errorf("URL = %q", p.URL)
	}
	if p.Username != "devnullcake" {
		t.Errorf("Username = %q", p.Username)
	}
}
