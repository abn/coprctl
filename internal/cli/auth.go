package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/config"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/render"
)

// warnThreshold is how far before expiry to warn (one month).
const warnThreshold = 30 * 24 * time.Hour

func newAuthCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect and manage credentials",
	}
	out.bind(cmd)
	cmd.AddCommand(
		newAuthLoginCmd(app, &out),
		newAuthStatusCmd(app, &out),
		newAuthTokenCmd(app, &out),
	)
	return cmd
}

// newAuthLoginCmd opens the instance API page in a browser and guides the user
// through pasting the [copr-cli] block from it, then writes a profile.
func newAuthLoginCmd(app *App, out *outFlags) *cobra.Command {
	var url, profile string
	var noOpen, interactive bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Open the instance API page and import a fresh token",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the instance URL: flag, then the current profile, then
			// production.
			base := strings.TrimRight(url, "/")
			if base == "" {
				if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
					app.Cfg = config.New(app.cfgPath, app.legacy)
				}
				if prof, err := app.Cfg.Profile(app.profile); err == nil && prof.BaseURL() != "" {
					base = prof.BaseURL()
				}
			}
			if base == "" {
				base = config.DefaultProductionURL
			}
			apiURL := base + "/api/"

			// Open the browser, unless the user asked not to.
			if !noOpen {
				if err := openBrowser(apiURL); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open a browser: %v\n", err)
				}
			}
			// Informational messages go to stderr so stdout stays
			// machine-parseable when the user asked for JSON output.
			fmt.Fprintf(cmd.ErrOrStderr(), "Visit %s, generate a token, and paste the [copr-cli] block below.\n", apiURL)
			fmt.Fprintln(cmd.ErrOrStderr(), "(paste the whole block, then press Enter; press Ctrl-D on a blank line to finish)")

			// Read the pasted block. In interactive mode read multiple lines
			// until EOF; otherwise read a pasted block from stdin.
			var p config.Profile
			if interactive || isTTY(cmd) {
				block := readMultiline(cmd)
				if strings.TrimSpace(block) != "" {
					if parsed, err := config.ParseLegacyBlock([]byte(block)); err == nil {
						p = parsed
					}
				}
				interactiveImport(cmd, &p)
			} else {
				if pasted := readPasted(cmd); pasted != "" {
					parsed, err := config.ParseLegacyBlock([]byte(pasted))
					if err != nil {
						return err
					}
					p = parsed
				}
			}

			if p.Token == "" || p.Login == "" {
				return fmt.Errorf("no credentials received: paste the [copr-cli] block from %s, or run with -i to enter values by hand", apiURL)
			}
			if p.URL == "" {
				p.URL = base
			}
			// Write the profile without emitting a separate import result.
			name := profile
			if name == "" {
				name = config.DetectInstance(p.BaseURL())
			}
			if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
				app.Cfg = config.New(app.cfgPath, app.legacy)
			}
			if existing, err := app.Cfg.Profile(name); err == nil && profile == "" && existing.Username != p.Username && existing.Username != "" {
				return fmt.Errorf("profile %q already exists for a different user; pass --profile to update it explicitly", name)
			}
			if err := app.Cfg.SetProfile(name, p); err != nil {
				return err
			}
			// Emit a single result: profile, instance, and expiry status.
			w := expiryWarning{Profile: name, Expiry: p.TokenExpiry}
			if p.TokenExpiry != "" {
				if exp, perr := parseExpiry(p.TokenExpiry); perr == nil {
					rem := time.Until(exp)
					switch {
					case rem < 0:
						w.Status = "expired"
						w.Remaining = "expired"
					case rem < warnThreshold:
						w.Status = "warning"
						w.Remaining = roundDuration(rem)
					default:
						w.Status = "ok"
						w.Remaining = roundDuration(rem)
					}
				}
			}
			if w.Status == "" {
				w.Status = "unknown"
			}
			return renderResult(cmd, out, map[string]any{
				"logged_in":   true,
				"profile":     name,
				"instance":    config.DetectInstance(p.BaseURL()),
				"expiry":      w.Expiry,
				"status":      w.Status,
				"remaining":   w.Remaining,
				"config_file": app.cfgPath,
			})
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "instance base URL (default: production or current profile)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name (default: instance name)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open a browser; print the URL instead")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "prompt for each credential instead of pasting a block")
	return cmd
}

// readMultiline reads until EOF, returning the pasted block.
func readMultiline(cmd *cobra.Command) string {
	r := bufio.NewReader(os.Stdin)
	var b strings.Builder
	for {
		line, err := r.ReadString('\n')
		b.WriteString(line)
		if err != nil {
			break
		}
	}
	return b.String()
}

// expiryWarning describes the token expiry state for a profile.
type expiryWarning struct {
	Profile   string `json:"profile"`
	Expiry    string `json:"expiry,omitempty"`
	Status    string `json:"status"` // valid | warning | expired | invalid | error
	Remaining string `json:"remaining,omitempty"`
	Username  string `json:"username,omitempty"`
}

func newAuthStatusCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show who you are and whether the token is near expiry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
				app.Cfg = newConfigManager(app.cfgPath, app.legacy)
			}
			prof, err := app.Cfg.Profile(app.profile)
			if err != nil {
				return err
			}
			login, token, aerr := prof.AuthErr()
			if aerr != nil {
				return aerr
			}
			if login == "" || token == "" {
				return cerr.Auth("no credentials configured for this profile")
			}
			w := expiryWarning{Profile: profileName(app), Expiry: prof.TokenExpiry, Username: prof.Username}
			// Live check: is the token accepted by the instance?
			c := copr.New(prof.BaseURL(), copr.TokenAuth(login, token))
			if live, err := c.AuthCheck(cmd.Context()); err == nil {
				if live.Name != "" {
					w.Username = live.Name
				}
				w.Status = "valid"
			} else {
				if cerr.ExitCodeFor(err) == cerr.ExitAuth {
					w.Status = "invalid"
					w.Remaining = "token rejected by the instance; run 'coprctl auth token rotate'"
				} else {
					w.Status = "error"
					w.Remaining = fmt.Sprintf("could not reach the instance: %v", err)
				}
			}
			// Overlay config-derived expiry when present.
			if prof.TokenExpiry != "" {
				if exp, perr := parseExpiry(prof.TokenExpiry); perr == nil {
					rem := time.Until(exp)
					switch {
					case rem < 0:
						w.Status = "expired"
						w.Remaining = "expired"
					case rem < warnThreshold && w.Status == "valid":
						w.Status = "warning"
						w.Remaining = fmt.Sprintf("%s", roundDuration(rem))
					default:
						w.Remaining = fmt.Sprintf("%s", roundDuration(rem))
					}
				}
			}
			if w.Status == "" {
				w.Status = "unknown"
			}
			if err := renderHumanOr(cmd, out, w, func() *render.Table {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("Profile", w.Profile)
				t.Add("Username", w.Username)
				t.Add("Login", prof.Login)
				t.Add("Expiry", w.Expiry)
				t.Add("Status", w.Status)
				if w.Remaining != "" {
					t.Add("Remaining", w.Remaining)
				}
				return t
			}); err != nil {
				return err
			}
			// Exit non-zero with code 13 when expired, per the precondition rule.
			if w.Status == "expired" {
				return cerr.New("token_expired", cerr.ExitPrecondition,
					"the API token has expired; run 'coprctl auth token rotate'")
			}
			if w.Status == "invalid" {
				return cerr.New("token_invalid", cerr.ExitAuth,
					"the API token was rejected by the instance; run 'coprctl auth token rotate'")
			}
			return nil
		},
	}
	return cmd
}

func newAuthTokenCmd(app *App, out *outFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate the API token and update the profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !*yes {
				return confirmRequired("--yes")
			}
			if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
				app.Cfg = newConfigManager(app.cfgPath, app.legacy)
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			nt, err := c.RotateAPIToken(cmd.Context())
			if err != nil {
				return err
			}
			name := profileName(app)
			// When the effective profile comes from the legacy config file
			// (no coprctl profile), write the rotated credentials back to the
			// legacy file so the user's source of truth reflects the rotation.
			legacySourced := legacySourced(app)
			if legacySourced {
				if err := app.Cfg.WriteLegacyCredentials(nt.APILogin, nt.APIToken, nt.Expiration); err != nil {
					return err
				}
			} else {
				if err := app.Cfg.SetProfileToken(name, nt.APILogin, nt.APIToken, nt.Expiration); err != nil {
					return err
				}
			}
			// Invalidate the cached client so subsequent calls use the new token.
			app.ResetClient()
			return renderResult(cmd, out, map[string]any{
				"rotated":        true,
				"profile":        name,
				"expiration":     nt.Expiration,
				"config_file":    app.cfgPath,
				"legacy_sourced": legacySourced,
			})
		},
	}
	yes = addYesFlag(cmd, yesHelp, true)
	return cmd
}

func roundDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// parseExpiry accepts a date-only ("2006-01-02") or a full RFC3339 timestamp,
// since the API token endpoint may return either.
func parseExpiry(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized expiry %q", s)
}

// legacySourced reports whether the effective profile comes from the legacy
// config file rather than a coprctl profile. This is the case when no coprctl
// config file exists (or it has no profile), so the legacy file is the only
// source of credentials.
func legacySourced(app *App) bool {
	if app.Cfg == nil {
		return false
	}
	name := profileName(app)
	return app.Cfg.HasProfile(name) == false
}

// openBrowser opens a URL in the platform default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
