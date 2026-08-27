package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
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
		newAuthStatusCmd(app, &out),
		newAuthTokenCmd(app, &out),
	)
	return cmd
}

// expiryWarning describes the token expiry state for a profile.
type expiryWarning struct {
	Profile   string `json:"profile"`
	Expiry    string `json:"expiry,omitempty"`
	Status    string `json:"status"` // ok | warning | expired
	Remaining string `json:"remaining,omitempty"`
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
			login, token := prof.Auth()
			if login == "" || token == "" {
				return cerr.Auth("no credentials configured for this profile")
			}
			w := expiryWarning{Profile: profileName(app), Expiry: prof.TokenExpiry}
			if prof.TokenExpiry != "" {
				if exp, perr := parseExpiry(prof.TokenExpiry); perr == nil {
					rem := time.Until(exp)
					switch {
					case rem < 0:
						w.Status = "expired"
						w.Remaining = "expired"
					case rem < warnThreshold:
						w.Status = "warning"
						w.Remaining = fmt.Sprintf("%s", roundDuration(rem))
					default:
						w.Status = "ok"
						w.Remaining = fmt.Sprintf("%s", roundDuration(rem))
					}
				}
			}
			if w.Status == "" {
				w.Status = "unknown"
			}
			if isHuman(out.format) {
				t := render.NewTable("FIELD", "VALUE")
				t.Add("Profile", w.Profile)
				t.Add("Username", prof.Username)
				t.Add("Login", prof.Login)
				t.Add("Expiry", w.Expiry)
				t.Add("Status", w.Status)
				if w.Remaining != "" {
					t.Add("Remaining", w.Remaining)
				}
				if err := renderResult(cmd, out, t); err != nil {
					return err
				}
			} else if err := renderResult(cmd, out, w); err != nil {
				return err
			}
			// Exit non-zero with code 13 when expired, per the precondition rule.
			if w.Status == "expired" {
				return cerr.New("token_expired", cerr.ExitPrecondition,
					"the API token has expired; run 'coprctl auth token rotate'")
			}
			return nil
		},
	}
	return cmd
}

func newAuthTokenCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate the API token and update the profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
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
			if err := app.Cfg.SetProfileToken(name, nt.APILogin, nt.APIToken, nt.Expiration); err != nil {
				return err
			}
			// Invalidate the cached client so subsequent calls use the new token.
			app.ResetClient()
			return renderResult(cmd, out, map[string]any{
				"rotated":     true,
				"profile":     name,
				"expiration":  nt.Expiration,
				"config_file": app.cfgPath,
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
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
