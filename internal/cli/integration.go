package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/copr"
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
// webhookURL returns the Copr webhook URL for a project. When pkgName is
// non-empty the URL is package-scoped, which lets Copr match the tag to that
// package by name (so a bare v<semver> tag works regardless of the package
// name). The secret comes from local state (never invented).
func webhookURL(ctx context.Context, app *App, r ref.Ref, pkgName string) (string, error) {
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
	u := fmt.Sprintf("%s/webhooks/github/%d/%s/", profileURL(app), proj.ID, secret)
	if pkgName != "" {
		u += pkgName + "/"
	}
	return u, nil
}

func newIntegrationURLCmd(app *App, out *outFlags) *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:   "url REF",
		Short: "Print the Copr webhook URL for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			u, err := webhookURL(cmd.Context(), app, r, "")
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
	var reveal, tagOnly, noAutoRebuild bool
	cmd := &cobra.Command{
		Use:   "enable REF --repo OWNER/REPO",
		Short: "Enable a GitHub webhook for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
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
			owner, repoName := splitRepo(repo)
			// Resolve the project's SCM packages that belong to this repo, so
			// the webhook can be package-scoped (letting Copr match a bare
			// v<semver> tag by package name) and auto-rebuild can be enabled.
			pkgs, err := scmPackages(cmd.Context(), app, r, "https://github.com/"+repo)
			if err != nil {
				return err
			}
			pkgScope := ""
			if len(pkgs) > 0 {
				pkgScope = pkgs[0].Name
			}
			u, err := webhookURL(cmd.Context(), app, r, pkgScope)
			if err != nil {
				return err
			}
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
			store, err := state.NewStore(mustStateDir(app.profile))
			if err != nil {
				return fmt.Errorf("state store: %w", err)
			}
			if err := store.SetHookID(r.Owner, r.Project, hook.ID); err != nil {
				return fmt.Errorf("record forge hook id: %w", err)
			}
			// Verify with a ping.
			code, err := gh.PingHook(cmd.Context(), owner, repoName, hook.ID)
			if err != nil {
				return err
			}
			// Enabling a webhook implies the tag pushes should rebuild the
			// package. Enable webhook auto-rebuild on the project's packages
			// unless the user opts out.
			autoRebuilt := false
			if !noAutoRebuild {
				autoRebuilt, err = enableAutoRebuild(cmd, app, r, pkgs)
				if err != nil {
					return err
				}
			}
			if !reveal {
				u = maskSecret(u)
			}
			return renderResult(cmd, out, map[string]any{
				"enabled": true, "repo": repo, "hook_id": hook.ID,
				"url": u, "ping_status": code, "events": evs,
				"auto_rebuild": autoRebuilt, "package": pkgScope,
			})
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo owner/name")
	cmd.Flags().StringVar(&events, "events", "", "comma-separated events (overrides tag-only default)")
	cmd.Flags().BoolVar(&tagOnly, "tag-only", true, "trigger only on push of a tag (GitHub create event)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "reveal the secret in output")
	cmd.Flags().BoolVar(&noAutoRebuild, "no-auto-rebuild", false, "do not enable webhook auto-rebuild on the project's packages")
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
	var yes *bool
	cmd := &cobra.Command{
		Use:   "rotate-secret REF",
		Short: "Generate a new webhook secret and cache it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			if !*yes {
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
	yes = addYesFlag(cmd, yesHelp, true)
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

// scmPackages returns the SCM packages in a project that belong to the given
// GitHub clone URL (or all SCM packages when cloneURL is empty).
func scmPackages(ctx context.Context, app *App, r ref.Ref, cloneURL string) ([]copr.Package, error) {
	c, err := app.Client()
	if err != nil {
		return nil, err
	}
	pkgs, err := c.ListPackages(ctx, r.Owner, r.Project)
	if err != nil {
		return nil, err
	}
	normalized := normalizeCloneURL(cloneURL)
	var out []copr.Package
	for _, p := range pkgs {
		if p.SourceType != copr.SourceSCM {
			continue
		}
		if normalized != "" && normalizeCloneURL(p.SourceDict["clone_url"]) != normalized {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// enableAutoRebuild turns on webhook auto-rebuild for the given packages, so
// tag pushes trigger Copr rebuilds. It returns whether any package was updated.
func enableAutoRebuild(cmd *cobra.Command, app *App, r ref.Ref, pkgs []copr.Package) (bool, error) {
	c, err := app.Client()
	if err != nil {
		return false, err
	}
	updated := false
	for _, p := range pkgs {
		if p.AutoRebuild {
			continue
		}
		src := make(map[string]any, len(p.SourceDict))
		for k, v := range p.SourceDict {
			src[k] = v
		}
		if err := c.EditPackage(cmd.Context(), copr.PackageCreate{
			Owner: r.Owner, Project: r.Project, Name: p.Name,
			SourceType: p.SourceType, Source: src,
			AutoRebuild: true, SetAutoRebuild: true,
		}); err != nil {
			return updated, err
		}
		updated = true
	}
	return updated, nil
}

// normalizeCloneURL normalizes a GitHub clone URL for comparison, producing
// the canonical github.com/OWNER/REPO form.
func normalizeCloneURL(u string) string {
	u = strings.TrimSuffix(strings.TrimSpace(u), ".git")
	if strings.HasPrefix(u, "git@github.com:") {
		u = "github.com/" + strings.TrimPrefix(u, "git@github.com:")
	}
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.Trim(u, "/")
}

func splitRepo(s string) (string, string) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return s, s
	}
	return parts[0], parts[1]
}
