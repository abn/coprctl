package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/config"
)

// credentialEnvVars lists every variable that can supply credentials, so
// TestMain can neutralise the ambient shell for the whole package.
var credentialEnvVars = []string{
	config.EnvConfig, config.EnvURL, config.EnvUsername, config.EnvLogin, config.EnvToken,
	"COPR_CONFIG", "COPR_URL", "COPR_USERNAME", "COPR_LOGIN", "COPR_TOKEN",
}

func TestMain(m *testing.M) {
	for _, name := range credentialEnvVars {
		os.Unsetenv(name)
	}
	os.Exit(m.Run())
}

// isolateHome points HOME and the XDG config location at a fresh temp dir, so a
// test never reads or writes the developer's real configuration.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

func TestAuthLoginPersistsEnvConfig(t *testing.T) {
	home := isolateHome(t)
	t.Setenv(config.EnvConfig, "[copr-cli]\nusername = \"alice\"\nlogin = \"LOGIN_ENV\"\ntoken = \"TOKEN_ENV\"\ncopr_url = https://copr.example.org\n")

	app := NewApp()
	var buf bytes.Buffer
	cmd := newAuthCmd(app)
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"login", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v (out %q)", err, buf.String())
	}
	if res["source"] != "env" {
		t.Errorf("source = %v, want env", res["source"])
	}
	if res["profile"] != "copr.example.org" {
		t.Errorf("profile = %v, want copr.example.org", res["profile"])
	}
	// The credentials are written to the config file verbatim.
	data, err := os.ReadFile(filepath.Join(home, ".config", "coprctl", "config.toml"))
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	for _, want := range []string{"alice", "LOGIN_ENV", "TOKEN_ENV"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config is missing %q:\n%s", want, data)
		}
	}
}

func TestAuthRotateEnvRequiresReveal(t *testing.T) {
	isolateHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected without --reveal, got %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv(config.EnvLogin, "LOGIN_ENV")
	t.Setenv(config.EnvToken, "TOKEN_ENV")
	t.Setenv(config.EnvURL, srv.URL)

	app := NewApp()
	cmd := newAuthCmd(app)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"rotate", "--yes", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected rotation to be refused without --reveal")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitPrecondition {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitPrecondition)
	}
	if _, statErr := os.Stat(app.cfgPath); !os.IsNotExist(statErr) {
		t.Errorf("config file must not be written, stat err = %v", statErr)
	}
}

func TestAuthRotateEnvRevealPrintsOnly(t *testing.T) {
	isolateHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api_3/api-token" {
			json.NewEncoder(w).Encode(map[string]any{
				"api_login": "NEWLOGIN", "api_token": "NEWTOKEN", "expiration": "2027-01-01",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv(config.EnvLogin, "LOGIN_ENV")
	t.Setenv(config.EnvToken, "TOKEN_ENV")
	t.Setenv(config.EnvURL, srv.URL)

	app := NewApp()
	var buf bytes.Buffer
	cmd := newAuthCmd(app)
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"rotate", "--yes", "--reveal", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v (out %q)", err, buf.String())
	}
	if res["persisted"] != false || res["token"] != "NEWTOKEN" || res["login"] != "NEWLOGIN" {
		t.Errorf("result = %v", res)
	}
	// Ephemeral credentials are never persisted, not even after rotation.
	if _, statErr := os.Stat(app.cfgPath); !os.IsNotExist(statErr) {
		t.Errorf("config file must not be written, stat err = %v", statErr)
	}
}

func TestConfigShowReportsEnvSource(t *testing.T) {
	isolateHome(t)
	t.Setenv(config.EnvLogin, "LOGIN_ENV")
	t.Setenv(config.EnvToken, "TOKEN_ENV")

	app := NewApp()
	var buf bytes.Buffer
	cmd := newConfigCmd(app)
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"show", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v (out %q)", err, buf.String())
	}
	if res["source"] != "env" {
		t.Errorf("source = %v, want env", res["source"])
	}
	envSource, _ := res["env_source"].(string)
	for _, want := range []string{config.EnvLogin, config.EnvToken} {
		if !strings.Contains(envSource, want) {
			t.Errorf("env_source %q is missing %q", envSource, want)
		}
	}
	if res["token_present"] != true {
		t.Errorf("token_present = %v, want true", res["token_present"])
	}
}

func TestUsernameFromEnvVarSkipsLookup(t *testing.T) {
	isolateHome(t)
	t.Setenv(config.EnvUsername, "alice")
	t.Setenv(config.EnvLogin, "LOGIN_ENV")
	t.Setenv(config.EnvToken, "TOKEN_ENV")

	app := NewApp()
	if got := app.Username(context.Background()); got != "alice" {
		t.Errorf("username = %q, want alice", got)
	}
}

func TestUsernameResolvedFromAuthCheck(t *testing.T) {
	isolateHome(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/auth-check" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{"name": "resolved", "id": 7})
	}))
	defer srv.Close()
	t.Setenv(config.EnvLogin, "LOGIN_ENV")
	t.Setenv(config.EnvToken, "TOKEN_ENV")
	t.Setenv(config.EnvURL, srv.URL)

	app := NewApp()
	for i := 0; i < 2; i++ {
		if got := app.Username(context.Background()); got != "resolved" {
			t.Fatalf("username = %q, want resolved", got)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("auth-check calls = %d, want 1 (cached)", got)
	}
}

func TestConfigSetDoesNotPersistEnvCredentials(t *testing.T) {
	home := isolateHome(t)
	t.Setenv(config.EnvLogin, "LOGIN_ENV")
	t.Setenv(config.EnvToken, "TOKEN_ENV")

	app := NewApp()
	cmd := newConfigCmd(app)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"set", "username", "bob", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "coprctl", "config.toml"))
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if !strings.Contains(string(data), "bob") {
		t.Errorf("username not written:\n%s", data)
	}
	// The write path must read the file profile, never the effective one.
	for _, leak := range []string{"TOKEN_ENV", "LOGIN_ENV"} {
		if strings.Contains(string(data), leak) {
			t.Errorf("environment credential %q leaked into the config:\n%s", leak, data)
		}
	}
}

func TestConfigShowEnvTOMLConfig(t *testing.T) {
	isolateHome(t)
	t.Setenv(config.EnvConfig, `default_profile = "staging"

[profiles.production]
url = "https://copr.fedorainfracloud.org"
username = "alice"
login = "PL"
token = "PT"

[profiles.staging]
url = "https://copr.stg.example.org"
username = "bob"
login = "SL"
token = "ST"
`)

	app := NewApp()
	var buf bytes.Buffer
	cmd := newConfigCmd(app)
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"show", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v (out %q)", err, buf.String())
	}
	if res["source"] != "env" || res["url"] != "https://copr.stg.example.org" || res["username"] != "bob" {
		t.Errorf("result = %v", res)
	}
}

func TestAuthLoginPersistsEnvTOMLProfile(t *testing.T) {
	home := isolateHome(t)
	t.Setenv(config.EnvConfig, `default_profile = "staging"

[profiles.production]
url = "https://copr.fedorainfracloud.org"
login = "PL"
token = "PT"

[profiles.staging]
url = "https://copr.stg.example.org"
login = "SL"
token = "ST"
`)

	app := NewApp()
	var buf bytes.Buffer
	cmd := newAuthCmd(app)
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"login", "--profile", "production", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v (out %q)", err, buf.String())
	}
	if res["profile"] != "production" {
		t.Errorf("profile = %v, want production", res["profile"])
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "coprctl", "config.toml"))
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if !strings.Contains(string(data), "PL") || strings.Contains(string(data), "SL") {
		t.Errorf("expected only the selected profile to persist:\n%s", data)
	}
}

func TestReadClientRejectsMalformedEnv(t *testing.T) {
	isolateHome(t)
	// A token without a login is a broken setup: it must fail loudly rather
	// than silently degrade to an anonymous client.
	t.Setenv(config.EnvToken, "TOKEN_ENV")

	app := NewApp()
	if _, err := app.ReadClient(); err == nil {
		t.Fatal("expected an error for a token without a login")
	}
}
