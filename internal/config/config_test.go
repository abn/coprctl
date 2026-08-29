package config

import (
	"os"
	"path/filepath"
	"strings"
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
		{"https://eur.openeuler.openatom.cn", InstanceOpenEuler},
		{"https://copr.internal.example.com", "copr.internal.example.com"},
		{"https://copr.mycompany.dev", "copr.mycompany.dev"},
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

func TestWriteLegacyCredentials(t *testing.T) {
	legacy := writeLegacy(t, `[copr-cli]
login = "oldlogin"
username = "devnullcake"
token = "oldtoken"
copr_url = "https://copr.stg.fedoraproject.org"
# expiration date: 2027-02-23
`)
	m := New("/nonexistent.toml", legacy)
	if err := m.WriteLegacyCredentials("newlogin", "newtoken", "2028-01-01"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(legacy)
	s := string(data)
	for _, want := range []string{"login = newlogin", "token = newtoken", "# expiration date: 2028-01-01"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "oldtoken") {
		t.Error("old token still present")
	}
}

func TestLoadMissingConfigIsFine(t *testing.T) {
	m := New(filepath.Join(t.TempDir(), "nope.toml"), filepath.Join(t.TempDir(), "nope"))
	if err := m.Load(); err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
}

func TestLoadUnreadableConfigErrors(t *testing.T) {
	// A directory in place of the config file is a real read error, not a
	// missing file; it must surface instead of silently returning defaults.
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	m := New(cfg, filepath.Join(t.TempDir(), "nope"))
	err := m.Load()
	if err == nil {
		t.Fatal("expected error reading an unreadable config path")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitConfig {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitConfig)
	}
}

func TestDefaultPathsWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	cfg, legacy := DefaultPaths()
	if cfg != "" || legacy != "" {
		t.Errorf("DefaultPaths() = (%q,%q), want empty when HOME is unset", cfg, legacy)
	}
}

func TestAuthTokenCommandFailureSurfaces(t *testing.T) {
	p := Profile{Login: "me", Token: "stale", TokenCommand: "false"}
	login, token := p.Auth()
	if login != "" || token != "" {
		t.Errorf("Auth() = (%q,%q), want empty (no silent fallback to inline token)", login, token)
	}
	if _, _, err := p.AuthErr(); err == nil {
		t.Error("expected error from a failing token_command")
	}
}

func TestAuthTokenCommandSuccess(t *testing.T) {
	p := Profile{Login: "me", Token: "stale", TokenCommand: "printf fresh"}
	login, token := p.Auth()
	if login != "me" || token != "fresh" {
		t.Errorf("Auth() = (%q,%q), want (me,fresh)", login, token)
	}
	if _, _, err := p.AuthErr(); err != nil {
		t.Errorf("AuthErr() unexpected error: %v", err)
	}
}

func TestAuthInlineFallbackWithoutTokenCommand(t *testing.T) {
	p := Profile{Login: "me", Token: "inline"}
	login, token := p.Auth()
	if login != "me" || token != "inline" {
		t.Errorf("Auth() = (%q,%q), want inline fallback", login, token)
	}
	if _, _, err := p.AuthErr(); err != nil {
		t.Errorf("AuthErr() unexpected error: %v", err)
	}
}
