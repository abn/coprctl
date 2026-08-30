package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/chroot"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/state"
)

func newChrootCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "chroot",
		Short: "The instance chroot catalog (mock chroots)",
	}
	out.bind(cmd)
	cmd.AddCommand(newChrootListCmd(app, &out))
	return cmd
}

func newChrootListCmd(app *App, out *outFlags) *cobra.Command {
	var distro, arch, filter, stateFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the chroot catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			cacheDir, _ := state.CacheDir(app.profile)
			chroots, warn, err := loadChrootCatalog(cmd.Context(), c, cacheDir)
			if err != nil {
				return err
			}
			if warn != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), warn)
			}
			names := filterChroots(chroots.ChrootNames(), filter, distro, arch)
			// EOL state is derived from the name against a maintained table;
			// a name in the catalog is by definition not Deleted.
			var states []chroot.Info
			for _, n := range names {
				states = append(states, chroot.Info{
					Name: n, State: chroot.Classify(n, true),
				})
			}
			if stateFilter != "" {
				states = filterByState(states, stateFilter)
			}
			rows := make([][]string, 0, len(states))
			for _, s := range states {
				rows = append(rows, []string{s.Name, string(s.State), chroots[s.Name]})
			}
			filtered := copr.MockChroots{}
			for _, s := range states {
				filtered[s.Name] = chroots[s.Name]
			}
			return renderTableRows(cmd, out, []string{"CHROOT", "STATE", "COMMENT"}, rows, filtered)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "glob filter on chroot name")
	cmd.Flags().StringVar(&distro, "distro", "", "distro filter")
	cmd.Flags().StringVar(&arch, "arch", "", "arch filter")
	cmd.Flags().StringVar(&stateFilter, "state", "", "filter by lifecycle state (active, preserved)")
	return cmd
}

// loadChrootCatalog fetches the mock-chroot catalog, caching it on success.
// When the fetch fails, a fresh cached copy is returned instead with a warning
// line for the caller to print; without a cache the fetch error is returned
// unchanged.
func loadChrootCatalog(ctx context.Context, c *copr.Client, cacheDir string) (copr.MockChroots, string, error) {
	chroots, err := c.ListMockChroots(ctx)
	if err == nil {
		_ = state.NewChrootCache(cacheDir).Store(chroots)
		return chroots, "", nil
	}
	cache := state.NewChrootCache(cacheDir)
	if cached, ok := cache.Load(); ok {
		return *cached, fmt.Sprintf(
			"warning: could not reach the instance (%v); using cached chroot catalog from %s",
			err, filepath.Join(cacheDir, "chroots.json")), nil
	}
	return nil, "", err
}

func filterByState(states []chroot.Info, want string) []chroot.Info {
	out := states[:0]
	for _, s := range states {
		if string(s.State) == want {
			out = append(out, s)
		}
	}
	return out
}

// warnIfInactive prints a warning when a target chroot is not active. The
// catalog is consulted when available; without it, only the EOL table decides.
func warnIfInactive(cmd *cobra.Command, name string, catalog copr.MockChroots) {
	inCatalog := true
	if _, ok := catalog[name]; !ok {
		inCatalog = len(catalog) == 0
	}
	if !chroot.IsActive(name, inCatalog) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: chroot %q is %s and will not accept new builds\n",
			name, chroot.Classify(name, inCatalog))
	}
}

// warnInactiveChroots loads the catalog (when reachable) and warns for every
// targeted chroot that is not active. A catalog fetch failure is non-fatal: the
// EOL table alone still decides.
func warnInactiveChroots(cmd *cobra.Command, names []string, app *App) {
	if len(names) == 0 {
		return
	}
	var catalog copr.MockChroots
	if c, err := app.ReadClient(); err == nil {
		if chroots, err := c.ListMockChroots(cmd.Context()); err == nil {
			catalog = chroots
		}
	}
	for _, n := range names {
		warnIfInactive(cmd, n, catalog)
	}
}
