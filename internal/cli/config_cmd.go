package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/config"
	"github.com/abn/coprctl/internal/render"
)

func newConfigCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration and profiles",
	}
	out.bind(cmd)
	cmd.AddCommand(
		newConfigShowCmd(app, &out),
		newConfigMigrateCmd(app, &out),
	)
	return cmd
}

func newConfigShowCmd(app *App, out *outFlags) *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:   "show [--reveal]",
		Short: "Show the effective configuration and where each value came from",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
				app.Cfg = config.New(app.cfgPath, app.legacy)
			}
			prof, err := app.Cfg.Profile(app.profile)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("KEY", "VALUE", "SOURCE")
				addRow := func(k, v, src string) {
					if !reveal && (k == "token" || k == "login") {
						v = "****"
					}
					t.Add(k, v, src)
				}
				addRow("url", prof.BaseURL(), config.FromLegacy)
				addRow("username", prof.Username, config.FromLegacy)
				addRow("login", prof.Login, config.FromLegacy)
				if reveal {
					addRow("token", prof.Token, config.FromLegacy)
				} else {
					addRow("token", "present", config.FromLegacy)
				}
				if prof.TokenExpiry != "" {
					addRow("token_expiry", prof.TokenExpiry, config.FromLegacy)
				}
				addRow("config file", app.cfgPath, config.FromDefault)
				addRow("legacy config", app.legacy, config.FromDefault)
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, map[string]any{
				"profile":       app.profile,
				"url":           prof.BaseURL(),
				"username":      prof.Username,
				"login":         prof.Login,
				"token_present": prof.Token != "" || prof.TokenCommand != "",
				"token_expiry":  prof.TokenExpiry,
				"config_file":   app.cfgPath,
				"legacy_config": app.legacy,
				"source":        config.FromLegacy,
			})
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "reveal secrets")
	return cmd
}

func newConfigMigrateCmd(app *App, out *outFlags) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Import the legacy ~/.config/copr config into a named profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			legacy, err := app.Cfg.LoadLegacy()
			if err != nil {
				return fmt.Errorf("no legacy config found at %s: %w", app.legacy, err)
			}
			if profile == "" {
				profile = "default"
			}
			// Refuse to clobber an existing config file.
			if _, err := os.Stat(app.cfgPath); err == nil {
				return fmt.Errorf("config file %s already exists; refusing to overwrite (merge manually or remove it first)", app.cfgPath)
			}
			dir := filepath.Dir(app.cfgPath)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			content := fmt.Sprintf("default_profile = %q\n\n[profiles.%s]\n", profile, profile)
			content += fmt.Sprintf("url = %q\n", legacy.BaseURL())
			if legacy.Username != "" {
				content += fmt.Sprintf("username = %q\n", legacy.Username)
			}
			if legacy.Login != "" {
				content += fmt.Sprintf("login = %q\n", legacy.Login)
			}
			if legacy.Token != "" {
				content += fmt.Sprintf("token = %q\n", legacy.Token)
			}
			if legacy.TokenExpiry != "" {
				content += fmt.Sprintf("token_expiry = %q\n", legacy.TokenExpiry)
			}
			if err := os.WriteFile(app.cfgPath, []byte(content), 0o600); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{
				"migrated":    true,
				"profile":     profile,
				"config_file": app.cfgPath,
			})
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "default", "profile name to create")
	return cmd
}
