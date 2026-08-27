// Package cli builds the Cobra command registry, the single source of truth
// for the CLI, help, completions, JSON schema, MCP tools, docs, and the agent
// skill.
package cli

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/config"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/state"
)

// App carries the shared dependencies for all commands.
type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	Cfg     *config.Manager
	profile string
	cfgPath string
	legacy  string
	client  *copr.Client
}

// NewApp builds an App with default streams.
func NewApp() *App {
	cfgPath, legacyPath := config.DefaultPaths()
	return &App{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Cfg:     config.New(cfgPath, legacyPath),
		cfgPath: cfgPath,
		legacy:  legacyPath,
	}
}

// Client returns the configured API client, building it lazily from the
// selected profile. It also installs the chroot-catalog predicate used to
// disambiguate three-segment references.
func (a *App) Client() (*copr.Client, error) {
	if a.client != nil {
		return a.client, nil
	}
	// Rebuild the manager if the effective config paths differ from the ones
	// it was constructed with (e.g. overridden by --config/--legacy-config).
	if a.Cfg == nil || !a.Cfg.Matches(a.cfgPath, a.legacy) {
		a.Cfg = config.New(a.cfgPath, a.legacy)
	}
	prof, err := a.Cfg.Profile(a.profile)
	if err != nil {
		return nil, err
	}
	login, token := prof.Auth()
	c := copr.New(prof.BaseURL(), copr.TokenAuth(login, token))
	a.client = c
	a.installChrootCatalog()
	return c, nil
}

// installChrootCatalog wires the cached catalog into the reference parser so
// three-segment references can distinguish a package from a chroot offline.
func (a *App) installChrootCatalog() {
	if cacheDir, err := state.CacheDir(a.profile); err == nil {
		cache := state.NewChrootCache(cacheDir)
		ref.SetChrootCatalog(func(name string) bool {
			if ch, ok := cache.Load(); ok {
				_, found := (*ch)[name]
				return found
			}
			return false
		})
	}
}

// Ctx returns a background context for command execution.
func Ctx() context.Context { return context.Background() }

// Root builds the root command tree.
func Root(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "coprctl",
		Short:         "A reimagined CLI for the Fedora Copr build system",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&app.profile, "profile", "", "instance profile")
	root.PersistentFlags().StringVar(&app.cfgPath, "config", app.cfgPath, "config file path")
	root.PersistentFlags().StringVar(&app.legacy, "legacy-config", app.legacy, "legacy config path (read-only)")

	root.AddCommand(
		newProjectCmd(app),
		newPackageCmd(app),
		newChrootCmd(app),
		newBuildCmd(app),
		newLogCmd(app),
		newMonitorCmd(app),
		newStatusCmd(app),
		newCompatCmd(app),
		newConfigCmd(app),
		newSkillCmd(app),
		newMCPCmd(app),
		newApplyCmd(app),
		newExportCmd(app),
		newDiffCmd(app),
		newValidateCmd(app),
		newIntegrationCmd(app),
		newDetectCmd(app),
		newInitCmd(app),
		newSyncCmd(app),
		newTryCmd(app),
		newUICmd(app),
		newCompletionCmd(),
		newDoctorCmd(app),
		newSchemaCmd(app),
		newVersionCmd(),
	)
	return root
}
