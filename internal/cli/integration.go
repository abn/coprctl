package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/forge"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/state"
)

// githubToken resolves a GitHub token from GITHUB_TOKEN or GH_TOKEN.
func githubToken() string {
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		return v
	}
	return os.Getenv("GH_TOKEN")
}

func newIntegrationCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "integration",
		Short: "Configure forge webhook integrations",
	}
	out.bind(cmd)
	github := &cobra.Command{
		Use:   "github",
		Short: "GitHub webhook integration",
	}
	github.AddCommand(newIntegrationGithubEnableCmd(app, &out))
	cmd.AddCommand(
		github,
		newIntegrationURLCmd(app, &out),
		newIntegrationRotateCmd(app, &out),
	)
	return cmd
}

// webhookURL builds the Copr webhook URL. The upstream-documented shape is
// https://<instance>/webhooks/<forge>/<project_id>/<secret>/. Project id is
// fetched live; the secret comes from local state (never invented).
func webhookURL(ctx context.Context, app *App, r ref.Ref) (string, error) {
	c, err := app.Client()
	if err != nil {
		return "", err
	}
	proj, err := c.GetProject(ctx, r.Owner, r.Project)
	if err != nil {
		return "", err
	}
	store, err := state.NewStore(mustStateDir(app.profile))
	if err != nil {
		return "", err
	}
	secret, err := store.GetSecret(r.Owner, r.Project)
	if err != nil || secret == "" {
		return "", fmt.Errorf("no webhook secret known for %s; run 'integration rotate-secret' first", r.String())
	}
	return fmt.Sprintf("%s/webhooks/github/%d/%s/", profileURL(app), proj.ID, secret), nil
}

func newIntegrationURLCmd(app *App, out *outFlags) *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:   "url REF",
		Short: "Print the Copr webhook URL for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			u, err := webhookURL(cmd.Context(), app, r)
			if err != nil {
				return err
			}
			if !reveal {
				u = maskSecret(u)
			}
			fmt.Fprintln(cmd.OutOrStdout(), u)
			return nil
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "print the secret in the URL")
	return cmd
}

func newIntegrationGithubEnableCmd(app *App, out *outFlags) *cobra.Command {
	var repo, events string
	var reveal, tagOnly bool
	cmd := &cobra.Command{
		Use:   "enable REF --repo OWNER/REPO",
		Short: "Enable a GitHub webhook for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			token := githubToken()
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN or GH_TOKEN is required for the GitHub integration")
			}
			// Default to tag-only triggers (GitHub `create` fires on tag
			// creation). `--events` overrides for full control.
			evs := defaultHookEvents(tagOnly, events)
			gh := forge.NewGitHub(token)
			u, err := webhookURL(cmd.Context(), app, r)
			if err != nil {
				return err
			}
			owner, repoName := splitRepo(repo)
			// Idempotent: reuse an existing hook pointing at this instance.
			hooks, err := gh.ListHooks(cmd.Context(), owner, repoName)
			if err != nil {
				return err
			}
			var hook *forge.Hook
			for i := range hooks {
				// Match on the configured webhook destination, not the API
				// resource URL (hooks[i].URL is the GitHub API endpoint).
				if strings.Contains(hooks[i].Config.URL, "/webhooks/") {
					hook = &hooks[i]
					break
				}
			}
			if hook != nil {
				if err := gh.UpdateHook(cmd.Context(), owner, repoName, hook.ID, u, evs); err != nil {
					return err
				}
			} else {
				hook, err = gh.CreateHook(cmd.Context(), owner, repoName, u, evs)
				if err != nil {
					return err
				}
			}
			// Persist the hook id for reconcile-on-rotate.
			if store, serr := state.NewStore(mustStateDir(app.profile)); serr == nil {
				_ = store.SetHookID(r.Owner, r.Project, hook.ID)
			}
			// Verify with a ping.
			code, err := gh.PingHook(cmd.Context(), owner, repoName, hook.ID)
			if err != nil {
				return err
			}
			if !reveal {
				u = maskSecret(u)
			}
			return renderResult(cmd, out, map[string]any{
				"enabled": true, "repo": repo, "hook_id": hook.ID,
				"url": u, "ping_status": code, "events": evs,
			})
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo owner/name")
	cmd.Flags().StringVar(&events, "events", "", "comma-separated events (overrides tag-only default)")
	cmd.Flags().BoolVar(&tagOnly, "tag-only", true, "trigger only on push of a tag (GitHub create event)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "reveal the secret in output")
	return cmd
}

// defaultHookEvents resolves the hook events. The default is tag-only (the
// GitHub `create` event fires when a tag is created, which drives Copr's
// tag-triggered rebuilds). An explicit --events value overrides it; setting
// --tag-only=false opts back in to branch pushes.
func defaultHookEvents(tagOnly bool, events string) []string {
	if events != "" {
		return splitComma(events)
	}
	if tagOnly {
		return []string{"create"}
	}
	return []string{"push", "create"}
}

func newIntegrationRotateCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rotate-secret REF",
		Short: "Generate a new webhook secret and cache it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			if !yes {
				return confirmRequired("--yes")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			secret, err := c.GenerateWebhookSecret(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			store, err := state.NewStore(mustStateDir(app.profile))
			if err != nil {
				return err
			}
			if err := store.SetSecret(r.Owner, r.Project, secret); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"rotated": true, "stored": true})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}

// helpers

func mustStateDir(profile string) string {
	dir, err := state.StateDir(profile)
	if err != nil {
		dir = ".coprctl-state"
	}
	return dir
}

func profileURL(app *App) string {
	prof, err := app.Cfg.Profile(app.profile)
	if err != nil {
		return ""
	}
	return prof.BaseURL()
}

func maskSecret(u string) string {
	// webhooks/github/<id>/<secret>/ -> replace secret with ****
	i := strings.Index(u, "/webhooks/")
	if i < 0 {
		return u
	}
	prefix := u[:i]
	parts := strings.Split(u[i:], "/")
	// parts: ["", "webhooks", "github", id, secret, ""]
	for idx := range parts {
		if parts[idx] == "github" || parts[idx] == "gitlab" || parts[idx] == "pagure" {
			if idx+2 < len(parts) {
				parts[idx+2] = "****"
			}
			break
		}
	}
	return prefix + strings.Join(parts, "/")
}

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitRepo(s string) (string, string) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return s, s
	}
	return parts[0], parts[1]
}
