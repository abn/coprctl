package cli

import (
	"context"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/state"
)

// chrootCompleter returns a flag-completion function for --chroot flags. It
// completes from the cached chroot catalog (with a live fetch fallback) so
// tab-completion works offline.
func chrootCompleter(app *App) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names := cachedChrootNames(app)
		if len(names) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, n := range names {
			if strings.HasPrefix(n, toComplete) {
				out = append(out, n)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// cachedChrootNames returns the chroot catalog names from the cache, refreshing
// it from the API when stale or missing.
func cachedChrootNames(app *App) []string {
	cacheDir, err := state.CacheDir(app.profile)
	if err != nil {
		return nil
	}
	cache := state.NewChrootCache(cacheDir)
	if ch, ok := cache.Load(); ok {
		return ch.ChrootNames()
	}
	// Live fallback: fetch and cache the catalog.
	c, err := app.ReadClient()
	if err != nil {
		return nil
	}
	ch, err := c.ListMockChroots(context.Background())
	if err != nil {
		return nil
	}
	_ = cache.Store(ch)
	return ch.ChrootNames()
}

// projectRefCompleter completes owner/project refs for positional arguments.
// It lists the configured user's projects via the API.
func projectRefCompleter(app *App) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		c, err := app.ReadClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		// Resolve the owner: use the authenticated user when possible.
		owner := ""
		if prof, err := app.Cfg.Profile(app.profile); err == nil {
			owner = prof.Username
		}
		projects, err := c.ListProjects(cmd.Context(), owner, 100)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, p := range projects {
			ref := p.FullName
			if ref == "" {
				ref = p.Ownername + "/" + p.Name
			}
			if strings.HasPrefix(ref, toComplete) {
				out = append(out, ref)
			}
		}
		sort.Strings(out)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// bindChrootCompletion attaches chroot completion to a --chroot flag.
func bindChrootCompletion(app *App, cmd *cobra.Command, flag string) {
	_ = cmd.RegisterFlagCompletionFunc(flag, chrootCompleter(app))
}

// bindRefCompletion attaches project-ref completion to the first positional
// argument.
func bindRefCompletion(app *App, cmd *cobra.Command) {
	cmd.ValidArgsFunction = projectRefCompleter(app)
}
