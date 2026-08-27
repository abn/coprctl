package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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
		newConfigImportCmd(app, &out),
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
			src := config.FromLegacy
			if _, statErr := os.Stat(app.cfgPath); statErr == nil {
				src = config.FromFile
			}
			if isHuman(out.format) {
				t := render.NewTable("KEY", "VALUE", "SOURCE")
				addRow := func(k, v string) {
					if !reveal && (k == "token" || k == "login") {
						v = "****"
					}
					t.Add(k, v, src)
				}
				addRow("profile", profileName(app))
				addRow("url", prof.BaseURL())
				addRow("instance", config.DetectInstance(prof.BaseURL()))
				addRow("username", prof.Username)
				addRow("login", prof.Login)
				if reveal {
					addRow("token", prof.Token)
				} else {
					addRow("token", "present")
				}
				if prof.TokenExpiry != "" {
					addRow("token_expiry", prof.TokenExpiry)
				}
				addRow("config file", app.cfgPath)
				addRow("legacy config", app.legacy)
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, map[string]any{
				"profile":       profileName(app),
				"url":           prof.BaseURL(),
				"instance":      config.DetectInstance(prof.BaseURL()),
				"username":      prof.Username,
				"login":         prof.Login,
				"token_present": prof.Token != "" || prof.TokenCommand != "",
				"token_expiry":  prof.TokenExpiry,
				"config_file":   app.cfgPath,
				"legacy_config": app.legacy,
				"source":        src,
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
			return importProfile(cmd, app, out, profile, legacy)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name (default: instance name)")
	return cmd
}

func newConfigImportCmd(app *App, out *outFlags) *cobra.Command {
	var profile, token, username, login, url string
	var interactive bool
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import credentials from pasted copr config, flags, or prompts",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build a profile from the available sources. Explicit flags
			// override pasted values; interactive prompts fill any gaps.
			var p config.Profile

			// Pasted block from stdin (non-TTY) or from a file.
			pasted := readPasted(cmd)
			if pasted != "" {
				parsed, err := config.ParseLegacyBlock([]byte(pasted))
				if err != nil {
					return err
				}
				p = parsed
			}

			// Explicit flags override pasted values.
			if token != "" {
				p.Token = token
			}
			if username != "" {
				p.Username = username
			}
			if login != "" {
				p.Login = login
			}
			if url != "" {
				p.URL = strings.TrimRight(url, "/")
			}

			// Interactive prompts fill any remaining gaps on a TTY.
			if interactive || isTTY(cmd) {
				interactiveImport(cmd, &p)
			}

			if p.Token == "" || p.Login == "" {
				return fmt.Errorf("incomplete credentials: need login and token (paste the copr config, pass --token/--username, or run interactively)")
			}
			// Default the instance to production when no URL is given.
			if p.URL == "" {
				p.URL = config.DefaultProductionURL
			}
			return importProfile(cmd, app, out, profile, p)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name (default: instance name)")
	cmd.Flags().StringVar(&token, "token", "", "API token")
	cmd.Flags().StringVar(&username, "username", "", "account username")
	cmd.Flags().StringVar(&login, "login", "", "API login")
	cmd.Flags().StringVar(&url, "url", "", "instance base URL (default: production)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "prompt for missing credentials")
	return cmd
}

// importProfile writes a profile into the config file, naming it by the
// instance when no profile name is given. It refuses to clobber existing
// profiles unless the profile name is explicitly the same.
func importProfile(cmd *cobra.Command, app *App, out *outFlags, profile string, p config.Profile) error {
	name := profile
	if name == "" {
		name = config.DetectInstance(p.BaseURL())
	}
	if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
		app.Cfg = config.New(app.cfgPath, app.legacy)
	}
	// If the profile already exists under a different username, refuse to
	// overwrite it unless the profile name was given explicitly.
	if existing, err := app.Cfg.Profile(name); err == nil && profile == "" && existing.Username != p.Username && existing.Username != "" {
		return fmt.Errorf("profile %q already exists for a different user; pass --profile to update it explicitly", name)
	}
	if err := app.Cfg.SetProfile(name, p); err != nil {
		return err
	}
	return renderResult(cmd, out, map[string]any{
		"imported":    true,
		"profile":     name,
		"instance":    config.DetectInstance(p.BaseURL()),
		"config_file": app.cfgPath,
	})
}

// readPasted returns a pasted legacy config block: from a file path argument
// if given, else from stdin when it is not a terminal.
func readPasted(cmd *cobra.Command) string {
	// A single positional arg is treated as a file path.
	args := cmd.Flags().Args()
	if len(args) == 1 {
		data, err := os.ReadFile(args[0])
		if err == nil {
			return string(data)
		}
		return ""
	}
	if !isTTY(cmd) {
		data, err := os.ReadFile("/dev/stdin")
		if err == nil {
			return string(data)
		}
	}
	return ""
}

func interactiveImport(cmd *cobra.Command, p *config.Profile) {
	r := bufio.NewReader(os.Stdin)
	if p.Username == "" {
		fmt.Fprint(cmd.OutOrStdout(), "Username: ")
		if s, _ := r.ReadString('\n'); s != "" {
			p.Username = strings.TrimSpace(s)
		}
	}
	if p.Login == "" {
		fmt.Fprint(cmd.OutOrStdout(), "API login: ")
		if s, _ := r.ReadString('\n'); s != "" {
			p.Login = strings.TrimSpace(s)
		}
	}
	if p.Token == "" {
		fmt.Fprint(cmd.OutOrStdout(), "API token: ")
		if pw, err := term.ReadPassword(int(os.Stdin.Fd())); err == nil {
			p.Token = strings.TrimSpace(string(pw))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if p.URL == "" {
		fmt.Fprint(cmd.OutOrStdout(), "Instance URL [default production]: ")
		if s, _ := r.ReadString('\n'); s != "" {
			if u := strings.TrimSpace(s); u != "" {
				p.URL = strings.TrimRight(u, "/")
			} else {
				p.URL = config.DefaultProductionURL
			}
		}
	}
}
