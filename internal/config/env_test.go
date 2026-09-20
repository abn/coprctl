package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
)

// credentialEnv lists every variable that can supply credentials. TestMain
// clears them once so the ambient shell cannot change what the suite observes;
// individual tests set only the variables they exercise.
var credentialEnv = []string{
	EnvConfig, EnvURL, EnvUsername, EnvLogin, EnvToken,
	envConfigAlt, envURLAlt, envUsernameAlt, envLoginAlt, envTokenAlt,
}

func TestMain(m *testing.M) {
	for _, name := range credentialEnv {
		os.Unsetenv(name)
	}
	os.Exit(m.Run())
}

const testBlock = `[copr-cli]
username = "alice"
login = "LOGIN1"
token = "TOKEN1"
copr_url = https://copr.example.org
# expiration date: 2027-02-23
`

func TestEnvironmentProfileInactive(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"nothing set", nil},
		{"url only", map[string]string{EnvURL: "https://copr.stg.example.org"}},
		{"username only", map[string]string{EnvUsername: "carol"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, _, ok, err := EnvironmentProfile()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok {
				t.Fatal("environment credentials must stay inactive")
			}
		})
	}
}

func TestEnvironmentProfileFromBlock(t *testing.T) {
	t.Setenv(EnvConfig, testBlock)
	prof, src, ok, err := EnvironmentProfile()
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if prof.Login != "LOGIN1" || prof.Token != "TOKEN1" || prof.Username != "alice" {
		t.Errorf("profile = %+v", prof)
	}
	if prof.URL != "https://copr.example.org" {
		t.Errorf("url = %q", prof.URL)
	}
	if prof.TokenExpiry != "2027-02-23" {
		t.Errorf("expiry = %q", prof.TokenExpiry)
	}
	if src.Config != EnvConfig || src.Active() == false {
		t.Errorf("source = %+v", src)
	}
}

func TestEnvironmentProfilePerFieldOverridesBlock(t *testing.T) {
	t.Setenv(EnvConfig, testBlock)
	t.Setenv(EnvURL, "https://other.example.org/")
	t.Setenv(EnvUsername, "bob")
	t.Setenv(EnvLogin, "LOGIN2")
	t.Setenv(EnvToken, "TOKEN2")

	prof, src, ok, err := EnvironmentProfile()
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if prof.URL != "https://other.example.org" {
		t.Errorf("url = %q, want trailing slash trimmed", prof.URL)
	}
	if prof.Username != "bob" || prof.Login != "LOGIN2" || prof.Token != "TOKEN2" {
		t.Errorf("profile = %+v", prof)
	}
	for field, want := range map[string]string{
		"config": src.Config, "url": src.URL, "username": src.Username,
		"login": src.Login, "token": src.Token,
	} {
		if want == "" {
			t.Errorf("source %s is unset", field)
		}
	}
}

func TestEnvironmentProfileFallbackNames(t *testing.T) {
	t.Setenv(envConfigAlt, testBlock)
	t.Setenv(envURLAlt, "https://alt.example.org")
	t.Setenv(envLoginAlt, "LOGIN3")
	t.Setenv(envTokenAlt, "TOKEN3")

	prof, src, ok, err := EnvironmentProfile()
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if prof.Login != "LOGIN3" || prof.Token != "TOKEN3" || prof.URL != "https://alt.example.org" {
		t.Errorf("profile = %+v", prof)
	}
	if src.Config != envConfigAlt || src.Login != envLoginAlt || src.Token != envTokenAlt {
		t.Errorf("source = %+v", src)
	}
}

func TestEnvironmentProfileNativeBeatsFallback(t *testing.T) {
	t.Setenv(envTokenAlt, "fallback")
	t.Setenv(EnvToken, "native")
	t.Setenv(envLoginAlt, "fallback")
	t.Setenv(EnvLogin, "native")

	prof, src, ok, err := EnvironmentProfile()
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if prof.Token != "native" || prof.Login != "native" {
		t.Errorf("profile = %+v", prof)
	}
	if src.Token != EnvToken || src.Login != EnvLogin {
		t.Errorf("source = %+v", src)
	}
}

func TestEnvironmentProfileDefaultsURL(t *testing.T) {
	t.Setenv(EnvLogin, "LOGIN1")
	t.Setenv(EnvToken, "TOKEN1")
	prof, src, ok, err := EnvironmentProfile()
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if prof.URL != DefaultProductionURL {
		t.Errorf("url = %q, want %q", prof.URL, DefaultProductionURL)
	}
	if src.URL != "" {
		t.Errorf("defaulted url must not claim a variable source, got %q", src.URL)
	}
}

func TestEnvironmentProfileTokenRequiresLogin(t *testing.T) {
	t.Setenv(EnvToken, "TOKEN1")
	_, _, ok, err := EnvironmentProfile()
	if ok {
		t.Fatal("token without login must not activate credentials")
	}
	if err == nil {
		t.Fatal("expected an error for a token without a login")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitConfig {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitConfig)
	}
}

func TestEnvironmentProfileBlockWithoutToken(t *testing.T) {
	t.Setenv(EnvConfig, "[copr-cli]\ncopr_url = https://copr.example.org\n")
	_, _, ok, err := EnvironmentProfile()
	if ok || err == nil {
		t.Fatalf("ok = %v, err = %v, want an error", ok, err)
	}
	if cerr.ExitCodeFor(err) != cerr.ExitConfig {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitConfig)
	}
}

func TestEnvironmentProfileBlockPlusSeparateToken(t *testing.T) {
	// A block carrying only the instance URL still works when the token and
	// login come from their own variables.
	t.Setenv(EnvConfig, "[copr-cli]\ncopr_url = https://copr.example.org\n")
	t.Setenv(EnvLogin, "LOGIN9")
	t.Setenv(EnvToken, "TOKEN9")

	prof, _, ok, err := EnvironmentProfile()
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	if prof.URL != "https://copr.example.org" || prof.Login != "LOGIN9" || prof.Token != "TOKEN9" {
		t.Errorf("profile = %+v", prof)
	}
}

func TestEnvironmentProfileRejectsCoprctlConfig(t *testing.T) {
	// coprctl's TOML config uses "url", not "copr_url". Accepting it here would
	// send its token to the default instance instead of the configured one.
	t.Setenv(EnvConfig, "default_profile = \"staging\"\n\n[profiles.staging]\nurl = \"https://copr.stg.example.org\"\nlogin = \"LOGIN1\"\ntoken = \"TOKEN1\"\n")
	_, _, ok, err := EnvironmentProfile()
	if ok || err == nil {
		t.Fatalf("ok = %v, err = %v, want an error", ok, err)
	}
	if cerr.ExitCodeFor(err) != cerr.ExitConfig {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitConfig)
	}
}

func TestEffectiveEnvWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	m := New(filepath.Join(dir, "config.toml"), filepath.Join(dir, "copr"))
	if err := m.SetProfile("default", Profile{
		URL: "https://file.example.org", Username: "fileuser",
		Login: "filelogin", Token: "filetoken",
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvLogin, "envlogin")
	t.Setenv(EnvToken, "envtoken")

	prof, src, err := m.Effective("")
	if err != nil {
		t.Fatal(err)
	}
	if !src.Active() {
		t.Fatal("expected an environment source")
	}
	if prof.Login != "envlogin" || prof.Token != "envtoken" {
		t.Errorf("effective profile = %+v", prof)
	}
	// The environment must not pull the file instance along: defaulting to
	// production keeps a token from leaking to an unrelated host.
	if prof.URL != DefaultProductionURL {
		t.Errorf("url = %q, want %q", prof.URL, DefaultProductionURL)
	}

	// Write paths keep reading the file profile.
	fileProf, err := m.Profile("")
	if err != nil {
		t.Fatal(err)
	}
	if fileProf.Username != "fileuser" || fileProf.Token != "filetoken" {
		t.Errorf("file profile = %+v", fileProf)
	}
}

func TestEffectiveFallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	m := New(filepath.Join(dir, "config.toml"), filepath.Join(dir, "copr"))
	if err := m.SetProfile("default", Profile{
		URL: "https://file.example.org", Login: "filelogin", Token: "filetoken",
	}); err != nil {
		t.Fatal(err)
	}
	prof, src, err := m.Effective("")
	if err != nil {
		t.Fatal(err)
	}
	if src.Active() {
		t.Errorf("source = %+v, want zero", src)
	}
	if prof.URL != "https://file.example.org" || prof.Token != "filetoken" {
		t.Errorf("profile = %+v", prof)
	}
}

func TestEffectiveReportsMalformedEnv(t *testing.T) {
	dir := t.TempDir()
	m := New(filepath.Join(dir, "config.toml"), filepath.Join(dir, "copr"))
	t.Setenv(EnvToken, "TOKEN1")
	if _, _, err := m.Effective(""); err == nil {
		t.Fatal("expected the malformed environment to surface")
	}
}

func TestEnvSourceNames(t *testing.T) {
	src := EnvSource{Config: EnvConfig, Login: EnvLogin, Token: EnvToken}
	got := src.Names()
	want := []string{EnvConfig, EnvLogin, EnvToken}
	if len(got) != len(want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if (EnvSource{}).Active() {
		t.Error("zero source must be inactive")
	}
}
