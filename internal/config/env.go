package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

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
// COPRCTL_CONFIG carries the contents of a credential file: either a [copr-cli]
// block, the text the Copr API page offers and the de-facto COPR_CONFIG
// convention holds, or a coprctl TOML config. A TOML config resolves its
// profile the way the file would: requested names it when given, otherwise its
// default_profile applies, otherwise the sole profile. The per-field variables
// override individual fields, so a shared file can be pointed at another
// instance or login without editing it.
//
// A token always requires a login: Copr API v3 authenticates with HTTP Basic
// login:token, so a token on its own can never be accepted.
func EnvironmentProfile(requested string) (Profile, EnvSource, bool, error) {
	var src EnvSource
	p := Profile{}

	raw, cfgVar := firstEnv(EnvConfig, envConfigAlt)
	if raw != "" {
		parsed, err := profileFromConfig(raw, requested, cfgVar)
		if err != nil {
			return Profile{}, EnvSource{}, false, err
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

	if !p.HasTokenSource() {
		if raw != "" {
			return Profile{}, EnvSource{}, false, cerr.Config(cfgVar + " contains no token").WithHint(
				"it must set token, token_command, or secret_handler; or supply one with " + EnvToken)
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

// profileFromConfig reads credentials from the contents of a credential file.
// A coprctl TOML config is recognised by yielding profiles; anything else is
// read as a [copr-cli] block. Profiles are checked first because the block
// parser is line-based and would otherwise read login and token out of a TOML
// profile section while losing its url.
func profileFromConfig(raw, requested, cfgVar string) (Profile, error) {
	var file File
	err := toml.Unmarshal([]byte(raw), &file)
	if err == nil && len(file.Profiles) > 0 {
		prof, _, perr := selectProfile(file, requested, cfgVar)
		return prof, perr
	}
	if mentionsCoprctlTOML(raw) {
		// It claims to be a coprctl config but yielded no readable profiles.
		// Refuse rather than fall through, so a token is never sent to the
		// default instance by accident.
		return Profile{}, cerr.Config(cfgVar + " looks like a coprctl config but has no readable profiles").WithHint(
			"check the TOML syntax, or pass a [copr-cli] block")
	}
	parsed, perr := ParseLegacyBlock([]byte(raw))
	if perr != nil {
		return Profile{}, cerr.Config("cannot parse " + cfgVar).Wrap(perr)
	}
	return parsed, nil
}

// selectProfile picks the profile a TOML config describes, following the same
// rule as the file: the requested name, then default_profile, then the implicit
// "default", with a sole profile accepted when nothing named one. The detail
// lives in the message and hint, because machine output drops wrapped causes.
func selectProfile(file File, requested, cfgVar string) (Profile, string, error) {
	names := make([]string, 0, len(file.Profiles))
	for name := range file.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	available := strings.Join(names, ", ")

	name := requested
	if name == "" {
		name = file.DefaultProfile
		if name == "" {
			// Manager.Load treats an empty default_profile as "default".
			name = "default"
		}
	}
	if prof, ok := file.Profiles[name]; ok {
		return prof, name, nil
	}
	// Nothing named a profile explicitly and there is only one, so it is
	// unambiguous.
	if requested == "" && file.DefaultProfile == "" && len(names) == 1 {
		return file.Profiles[names[0]], names[0], nil
	}
	switch {
	case requested != "":
		return Profile{}, "", cerr.Config(fmt.Sprintf("%s has no profile %q (available: %s)", cfgVar, requested, available)).WithHint(
			"name one of the available profiles with --profile")
	case file.DefaultProfile != "":
		return Profile{}, "", cerr.Config(fmt.Sprintf("%s default_profile %q is not in the config (available: %s)", cfgVar, file.DefaultProfile, available)).WithHint(
			"fix default_profile, or name a profile with --profile")
	default:
		return Profile{}, "", cerr.Config(fmt.Sprintf("%s holds several profiles and no default_profile (available: %s)", cfgVar, available)).WithHint(
			"name one with --profile")
	}
}

// Effective returns the profile in effect for API calls. Environment
// credentials always win over the file configuration; the returned source
// records which variables supplied them and is zero for file-sourced profiles.
// Write paths keep using Profile so ephemeral credentials are never persisted.
func (m *Manager) Effective(name string) (Profile, EnvSource, error) {
	env, src, ok, err := EnvironmentProfile(name)
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

// mentionsCoprctlTOML reports whether raw carries coprctl TOML markers. It
// never accepts a value, only refuses a malformed one explicitly, so it matches
// a section header or an assignment rather than any mention: a comment or a
// value containing "profiles" must not turn a valid [copr-cli] block away. The
// "[" is optional so a config whose header lost its bracket is still refused
// instead of being line-parsed without its instance URL.
func mentionsCoprctlTOML(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.TrimPrefix(line, "["), "profiles") {
			return true
		}
		if strings.HasPrefix(line, "default_profile") {
			return true
		}
	}
	return false
}
