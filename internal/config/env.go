package config

import (
	"os"
	"strings"

	"github.com/abn/coprctl/internal/cerr"
)

// Environment variable names for ephemeral credentials. The COPRCTL_ prefix is
// the native namespace; the COPR_ names are honoured as a fallback because they
// are an established convention in CI pipelines, even though copr-cli itself
// reads no environment variables.
const (
	EnvConfig   = "COPRCTL_CONFIG"
	EnvURL      = "COPRCTL_URL"
	EnvUsername = "COPRCTL_USERNAME"
	EnvLogin    = "COPRCTL_LOGIN"
	EnvToken    = "COPRCTL_TOKEN"

	envConfigAlt   = "COPR_CONFIG"
	envURLAlt      = "COPR_URL"
	envUsernameAlt = "COPR_USERNAME"
	envLoginAlt    = "COPR_LOGIN"
	envTokenAlt    = "COPR_TOKEN"
)

// EnvSource records which environment variable supplied each credential field,
// so commands can report provenance without echoing secret values.
type EnvSource struct {
	Config   string `json:"config,omitempty"`
	URL      string `json:"url,omitempty"`
	Username string `json:"username,omitempty"`
	Login    string `json:"login,omitempty"`
	Token    string `json:"token,omitempty"`
}

// Active reports whether any credential field came from the environment.
func (s EnvSource) Active() bool { return s != EnvSource{} }

// Names lists the variables that supplied credentials, in a stable order.
func (s EnvSource) Names() []string {
	var names []string
	for _, name := range []string{s.Config, s.URL, s.Username, s.Login, s.Token} {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// EnvironmentProfile assembles the ephemeral profile described by the
// environment. The third result is false when no credential variable is set,
// in which case the file configuration applies unchanged.
//
// COPRCTL_CONFIG carries a verbatim [copr-cli] credential block, the same text
// the Copr API page offers and the de-facto COPR_CONFIG convention holds. The
// per-field variables override individual block fields, so a shared block can
// be pointed at another instance or login without editing it.
//
// A token always requires a login: Copr API v3 authenticates with HTTP Basic
// login:token, so a token on its own can never be accepted.
func EnvironmentProfile() (Profile, EnvSource, bool, error) {
	var src EnvSource
	p := Profile{}

	raw, cfgVar := firstEnv(EnvConfig, envConfigAlt)
	if raw != "" {
		if looksLikeCoprctlConfig(raw) {
			return Profile{}, EnvSource{}, false, cerr.Config(cfgVar + " holds a coprctl config file, not a [copr-cli] block").WithHint(
				"pass the [copr-cli] credential block (the text the Copr API page offers), or use --config for a file path")
		}
		parsed, err := ParseLegacyBlock([]byte(raw))
		if err != nil {
			return Profile{}, EnvSource{}, false, cerr.Config("cannot parse " + cfgVar).Wrap(err)
		}
		p = parsed
		src.Config = cfgVar
	}

	if v, name := firstEnv(EnvURL, envURLAlt); v != "" {
		p.URL = strings.TrimRight(v, "/")
		src.URL = name
	}
	if v, name := firstEnv(EnvUsername, envUsernameAlt); v != "" {
		p.Username = v
		src.Username = name
	}
	if v, name := firstEnv(EnvLogin, envLoginAlt); v != "" {
		p.Login = v
		src.Login = name
	}
	if v, name := firstEnv(EnvToken, envTokenAlt); v != "" {
		p.Token = v
		src.Token = name
	}

	if p.Token == "" {
		if raw != "" {
			return Profile{}, EnvSource{}, false, cerr.Config(cfgVar + " contains no token").WithHint(
				"the block must set token, or supply one with " + EnvToken)
		}
		// URL and username alone do not select environment credentials: they
		// must never redirect file credentials to another instance.
		return Profile{}, EnvSource{}, false, nil
	}
	if p.Login == "" {
		return Profile{}, EnvSource{}, false, cerr.Config(
			"environment credentials are incomplete: a token without a login").WithHint(
			"the Copr API authenticates with a login and a token; set " +
				EnvLogin + " (or " + envLoginAlt + ")")
	}
	if p.URL == "" {
		p.URL = DefaultProductionURL
	}
	return p, src, true, nil
}

// Effective returns the profile in effect for API calls. Environment
// credentials always win over the file configuration; the returned source
// records which variables supplied them and is zero for file-sourced profiles.
// Write paths keep using Profile so ephemeral credentials are never persisted.
func (m *Manager) Effective(name string) (Profile, EnvSource, error) {
	env, src, ok, err := EnvironmentProfile()
	if err != nil {
		return Profile{}, EnvSource{}, err
	}
	if ok {
		return env, src, nil
	}
	prof, err := m.Profile(name)
	if err != nil {
		return Profile{}, EnvSource{}, err
	}
	return prof, EnvSource{}, nil
}

// firstEnv returns the value and name of the first non-empty variable.
func firstEnv(names ...string) (string, string) {
	for _, name := range names {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v, name
		}
	}
	return "", ""
}

// looksLikeCoprctlConfig reports whether raw is coprctl's own TOML config
// rather than a [copr-cli] block. Its URL key (url) differs from the block's
// (copr_url), so accepting it here would silently send a token to the default
// instance instead of the one it was issued for.
func looksLikeCoprctlConfig(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[profiles") || strings.HasPrefix(line, "default_profile") {
			return true
		}
	}
	return false
}
