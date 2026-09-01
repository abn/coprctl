package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
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

// gitlabToken resolves a GitLab token from GITLAB_TOKEN.
func gitlabToken() string {
	return os.Getenv("GITLAB_TOKEN")
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
	gitlab := &cobra.Command{
		Use:   "gitlab",
		Short: "GitLab webhook integration",
	}
	gitlab.AddCommand(newIntegrationGitlabEnableCmd(app, &out))
	cmd.AddCommand(
		github,
		gitlab,
		newIntegrationURLCmd(app, &out),
		newIntegrationDisableCmd(app, &out),
		newIntegrationRotateCmd(app, &out),
	)
	return cmd
}

// webhookURL returns the Copr webhook URL for a project and forge. The shape
// is the same for github, gitlab, and bitbucket: /webhooks/<forge>/<project_id>/
// <secret>[/<pkg>/] (webhooks_general.py). custom requires the package name;
// the server rejects the bare route with PACKAGE_NAME_REQUIRED. When pkgName is
// set the URL is package-scoped, which lets Copr match a tag to that package
// by name. The secret comes from local state (never invented).
func webhookURL(ctx context.Context, app *App, r ref.Ref, forge, pkgName string) (string, error) {
	if forge == "" {
		forge = "github"
	}
	switch forge {
	case "github", "gitlab", "bitbucket", "custom":
	default:
		return "", cerr.Usage(fmt.Sprintf("unsupported webhook forge %q", forge))
	}
	if forge == "custom" && pkgName == "" {
		return "", cerr.Usage("the custom webhook needs a package name; pass --package PKG")
	}
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
	u := fmt.Sprintf("%s/webhooks/%s/%d/%s/", profileURL(app), forge, proj.ID, secret)
	if pkgName != "" {
		u += pkgName + "/"
	}
	return u, nil
}

func newIntegrationURLCmd(app *App, out *outFlags) *cobra.Command {
	var forgeName, pkg string
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
			u, err := webhookURL(cmd.Context(), app, r, forgeName, pkg)
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
	cmd.Flags().StringVar(&forgeName, "forge", "github", "webhook forge: github, gitlab, bitbucket, custom")
	cmd.Flags().StringVar(&pkg, "package", "", "scope the URL to a package (required for custom)")
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
			// creation). `--events` overrides for full control; forge.HookEvents
			// owns the tag-only to event-list mapping.
			opts := forge.HookOptions{TagOnly: tagOnly}
			if events != "" {
				opts.Events = splitComma(events)
			}
			gh := forge.NewGitHub(token)
			res, err := enableWebhook(cmd, app, r, repo, "github", "https://github.com/", gh,
				opts, noAutoRebuild, reveal, gh.PingHook)
			if err != nil {
				return err
			}
			res["events"] = forge.HookEvents(opts)
			return renderResult(cmd, out, res)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo owner/name")
	cmd.Flags().StringVar(&events, "events", "", "comma-separated events (overrides tag-only default)")
	cmd.Flags().BoolVar(&tagOnly, "tag-only", true, "trigger only on push of a tag (GitHub create event)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "reveal the secret in output")
	cmd.Flags().BoolVar(&noAutoRebuild, "no-auto-rebuild", false, "do not enable webhook auto-rebuild on the project's packages")
	return cmd
}

func newIntegrationGitlabEnableCmd(app *App, out *outFlags) *cobra.Command {
	var repo string
	var reveal, tagOnly, noAutoRebuild bool
	cmd := &cobra.Command{
		Use:   "enable REF --repo GROUP/PROJECT",
		Short: "Enable a GitLab webhook for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			token := gitlabToken()
			if token == "" {
				return fmt.Errorf("GITLAB_TOKEN is required for the GitLab integration")
			}
			gl := forge.NewGitLab(token, os.Getenv("GITLAB_API_URL"))
			res, err := enableWebhook(cmd, app, r, repo, "gitlab", "https://gitlab.com/", gl,
				forge.HookOptions{TagOnly: tagOnly}, noAutoRebuild, reveal,
				func(ctx context.Context, owner, repo string, id int64) (int, error) {
					// GitLab test hooks have no delivery readback; the test
					// endpoint is newer than most self-hosted instances, so a
					// missing route must not undo the enable. Auth and rate-limit
					// failures are real problems and should not be hidden.
					trigger := "tag_push_events"
					if !tagOnly {
						trigger = "push_events"
					}
					status, err := gl.TestHook(ctx, owner, repo, id, trigger)
					if err != nil {
						if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
							return 0, nil
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: gitlab webhook test failed: %v\n", err)
						return 0, nil
					}
					return status, nil
				})
			if err != nil {
				return err
			}
			return renderResult(cmd, out, res)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitLab group/project")
	cmd.Flags().BoolVar(&tagOnly, "tag-only", true, "trigger only on push of a tag (GitLab tag push event)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "reveal the secret in output")
	cmd.Flags().BoolVar(&noAutoRebuild, "no-auto-rebuild", false, "do not enable webhook auto-rebuild on the project's packages")
	return cmd
}

// enableWebhook performs the enable flow shared by the github and gitlab
// commands: resolve the SCM package scope, compute the Copr receiver URL, list
// hooks on the repo and create or update the one aimed at this instance, record
// the hook id, run the forge delivery check, enable auto-rebuild, and assemble
// the result map. forgeSegment is the webhook URL segment; cloneBase the https
// clone prefix used to match SCM packages; ping, when non-nil, runs the
// forge-specific delivery check. The caller renders and can add forge-specific
// result fields.
func enableWebhook(cmd *cobra.Command, app *App, r ref.Ref, repo, forgeSegment, cloneBase string,
	mgr forge.HookManager, opts forge.HookOptions, noAutoRebuild, reveal bool,
	ping func(ctx context.Context, owner, repo string, id int64) (int, error)) (map[string]any, error) {
	owner, repoName := splitRepo(repo)
	// Resolve the project's SCM packages that belong to this repo, so the
	// webhook can be package-scoped (letting Copr match a bare v<semver> tag by
	// package name) and auto-rebuild can be enabled.
	pkgs, err := scmPackages(cmd.Context(), app, r, cloneBase+repo)
	if err != nil {
		return nil, err
	}
	pkgScope := ""
	if len(pkgs) > 0 {
		pkgScope = pkgs[0].Name
	}
	u, err := webhookURL(cmd.Context(), app, r, forgeSegment, pkgScope)
	if err != nil {
		return nil, err
	}
	// Idempotent: reuse an existing hook pointing at this instance.
	hooks, err := mgr.ListHooks(cmd.Context(), owner, repoName)
	if err != nil {
		return nil, err
	}
	var hook *forge.Hook
	// Match on the receiver prefix (forge and Copr project id), which is stable
	// across secret rotations, rather than the loose /webhooks/ substring that
	// could repoint a hook aimed at another project or instance.
	prefix := receiverPrefix(u)
	for i := range hooks {
		if prefix != "" && strings.Contains(hooks[i].DestinationURL(), prefix) {
			hook = &hooks[i]
			break
		}
	}
	if hook != nil {
		if err := mgr.UpdateHook(cmd.Context(), owner, repoName, hook.ID, u, opts); err != nil {
			return nil, err
		}
	} else {
		hook, err = mgr.CreateHook(cmd.Context(), owner, repoName, u, opts)
		if err != nil {
			return nil, err
		}
	}
	// Persist the hook id so disable can reconcile.
	store, err := state.NewStore(mustStateDir(app.profile))
	if err != nil {
		return nil, fmt.Errorf("state store: %w", err)
	}
	if err := store.SetHookID(r.Owner, r.Project, hook.ID); err != nil {
		return nil, fmt.Errorf("record forge hook id: %w", err)
	}
	pingStatus := 0
	if ping != nil {
		pingStatus, err = ping(cmd.Context(), owner, repoName, hook.ID)
		if err != nil {
			return nil, err
		}
	}
	// Enabling a webhook implies the tag pushes should rebuild the package.
	// Enable webhook auto-rebuild on the project's packages unless the user
	// opts out.
	autoRebuilt := false
	if !noAutoRebuild {
		autoRebuilt, err = enableAutoRebuild(cmd, app, r, pkgs)
		if err != nil {
			return nil, err
		}
	}
	if !reveal {
		u = maskSecret(u)
	}
	return map[string]any{
		"enabled": true, "repo": repo, "hook_id": hook.ID,
		"url": u, "ping_status": pingStatus,
		"auto_rebuild": autoRebuilt, "package": pkgScope,
	}, nil
}

// hookManager builds the forge client for a named forge, enforcing the token
// requirement. Used by disable; the enable commands build their own concrete
// clients because they need the forge-specific delivery check.
func hookManager(name string) (forge.HookManager, error) {
	switch name {
	case "github":
		token := githubToken()
		if token == "" {
			return nil, fmt.Errorf("GITHUB_TOKEN or GH_TOKEN is required for the GitHub integration")
		}
		return forge.NewGitHub(token), nil
	case "gitlab":
		token := gitlabToken()
		if token == "" {
			return nil, fmt.Errorf("GITLAB_TOKEN is required for the GitLab integration")
		}
		return forge.NewGitLab(token, os.Getenv("GITLAB_API_URL")), nil
	default:
		return nil, cerr.Usage(fmt.Sprintf("unsupported forge %q (github|gitlab)", name))
	}
}

// cloneBase returns the https clone URL prefix used to match SCM packages on a
// forge. Self-hosted GitLab (GITLAB_API_URL) is not reflected here: package
// scoping is best-effort and the receiver URL is independent of the clone host.
func cloneBase(forge string) string {
	if forge == "gitlab" {
		return "https://gitlab.com/"
	}
	return "https://github.com/"
}

// newIntegrationDisableCmd removes a forge webhook, verifying before it deletes
// that the hook's destination matches the expected Copr receiver URL for this
// forge and project. The stored hook id is a hint only and never the sole basis
// for the delete.
func newIntegrationDisableCmd(app *App, out *outFlags) *cobra.Command {
	var forgeName, repo string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "disable REF --forge github|gitlab --repo OWNER/REPO",
		Short: "Disable a forge webhook for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(app, args[0])
			if err != nil {
				return err
			}
			if !*yes {
				return confirmRequired("--yes")
			}
			mgr, err := hookManager(forgeName)
			if err != nil {
				return err
			}
			owner, repoName := splitRepo(repo)
			// Recompute the same receiver the enable path configured, so the
			// match is exact: forge segment, project id, and optional package
			// scope.
			pkgs, err := scmPackages(cmd.Context(), app, r, cloneBase(forgeName)+repo)
			if err != nil {
				return err
			}
			pkgScope := ""
			if len(pkgs) > 0 {
				pkgScope = pkgs[0].Name
			}
			u, err := webhookURL(cmd.Context(), app, r, forgeName, pkgScope)
			if err != nil {
				return err
			}
			hooks, err := mgr.ListHooks(cmd.Context(), owner, repoName)
			if err != nil {
				return err
			}
			var target *forge.Hook
			for i := range hooks {
				if hooks[i].DestinationURL() == u {
					target = &hooks[i]
					break
				}
			}
			if target == nil {
				return fmt.Errorf("no %s webhook on %s/%s points at %s; nothing to disable",
					forgeName, owner, repoName, maskSecret(u))
			}
			if err := mgr.DeleteHook(cmd.Context(), owner, repoName, target.ID); err != nil {
				return err
			}
			store, err := state.NewStore(mustStateDir(app.profile))
			if err != nil {
				return fmt.Errorf("state store: %w", err)
			}
			if err := store.ClearHookID(r.Owner, r.Project); err != nil {
				return fmt.Errorf("state store: %w", err)
			}
			// disable does NOT rotate the Copr webhook secret: the secret is
			// project-scoped and shared across every hook, and rotation would
			// break other consumers. A future change must not "fix" this.
			return renderResult(cmd, out, map[string]any{
				"disabled": true, "forge": forgeName, "repo": repo,
				"hook_id": target.ID, "stored_hook_id": 0, "url": maskSecret(u),
			})
		},
	}
	cmd.Flags().StringVar(&forgeName, "forge", "", "forge: github or gitlab")
	cmd.Flags().StringVar(&repo, "repo", "", "forge repo owner/name")
	yes = addYesFlag(cmd, "delete the forge webhook", true)
	return cmd
}

// receiverPrefix returns the path prefix that identifies a hook aimed at this
// Copr project and forge, independent of the secret and package scope. A
// rotated secret changes the stored destination, but the prefix survives.
func receiverPrefix(u string) string {
	i := strings.Index(u, "/webhooks/")
	if i < 0 {
		return ""
	}
	parts := strings.Split(u[i:], "/")
	if len(parts) < 5 {
		return ""
	}
	return strings.Join(parts[:4], "/") + "/"
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
	// webhooks/<forge>/<id>/<secret>/[...] masks the segment after the id for
	// every forge using that shape (github, gitlab, bitbucket, custom). The
	// masking is positional on that shape; a future custom-dir receiver would
	// need a different mask.
	i := strings.Index(u, "/webhooks/")
	if i < 0 {
		return u
	}
	parts := strings.Split(u[i:], "/")
	// parts: ["", "webhooks", forge, id, secret, pkg?, ""]
	if len(parts) >= 6 {
		parts[4] = "****"
	}
	return u[:i] + strings.Join(parts, "/")
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
// clone URL (or all SCM packages when cloneURL is empty).
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

// normalizeCloneURL canonicalizes a clone URL for comparison, producing the
// host/owner/repo form for the https and git@host:path ssh forms on any host.
func normalizeCloneURL(u string) string {
	u = strings.TrimSuffix(strings.TrimSpace(u), ".git")
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	// git@host:path scp-like form -> host/path. ssh://git@host:port/path drops
	// the port the same way.
	if at := strings.Index(u, "@"); at >= 0 {
		u = u[at+1:]
		if c := strings.Index(u, ":"); c >= 0 {
			rest := u[c+1:]
			if slash := strings.Index(rest, "/"); rest != "" && rest[0] >= '0' && rest[0] <= '9' && slash >= 0 {
				u = u[:c] + rest[slash:]
			} else {
				u = u[:c] + "/" + rest
			}
		}
	}
	return strings.Trim(u, "/")
}

func splitRepo(s string) (string, string) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return s, s
	}
	return parts[0], parts[1]
}
