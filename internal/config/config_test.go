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

func TestParseLegacyBlockWithExpiryComment(t *testing.T) {
	block := `[copr-cli]
login = "abc123"
username = "devnullcake"
token = "sekrit"
copr_url = "https://copr.stg.fedoraproject.org"
# expiration date: 2027-02-23
`
	p, err := ParseLegacyBlock([]byte(block))
	if err != nil {
		t.Fatal(err)
	}
	if p.Login != "abc123" || p.Token != "sekrit" || p.Username != "devnullcake" {
		t.Errorf("parsed fields wrong: %+v", p)
	}
	if p.TokenExpiry != "2027-02-23" {
		t.Errorf("expiry = %q, want 2027-02-23 (read from comment)", p.TokenExpiry)
	}
	if p.URL != "https://copr.stg.fedoraproject.org" {
		t.Errorf("url = %q", p.URL)
	}
}

func TestDetectInstance(t *testing.T) {
	tests := []struct{ url, want string }{
		{"https://copr.fedorainfracloud.org", InstanceProduction},
		{"https://copr.stg.fedoraproject.org", InstanceStaging},
		{"https://copr.internal.example.com", "copr.internal.example.com"},
	}
	for _, tt := range tests {
		if got := DetectInstance(tt.url); got != tt.want {
			t.Errorf("DetectInstance(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSetProfileSetsDefault(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.toml")
	m := New(cfg, "/nonexistent")
	err := m.SetProfile("production", Profile{
		URL: "https://copr.fedorainfracloud.org", Username: "abn", Login: "l", Token: "t",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.DefaultProfileName(); got != "production" {
		t.Errorf("default profile = %q, want production", got)
	}
	// A subsequent profile should not steal the default.
	if err := m.SetProfile("staging", Profile{URL: "https://copr.stg.fedoraproject.org"}); err != nil {
		t.Fatal(err)
	}
	if got := m.DefaultProfileName(); got != "production" {
		t.Errorf("default profile changed to %q", got)
	}
}
