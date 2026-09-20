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

	Cfg      *config.Manager
	profile  string
	cfgPath  string
	legacy   string
	client   *copr.Client
	username string
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
// effective profile (environment credentials win over the file configuration).
// It also installs the chroot-catalog predicate used to disambiguate
// three-segment references.
func (a *App) Client() (*copr.Client, error) {
	if a.client != nil {
		return a.client, nil
	}
	prof, _, err := a.profileForUse()
	if err != nil {
		return nil, err
	}
	return a.installClient(prof), nil
}

// installClient caches a client for the given profile and wires the chroot
// catalog into the reference parser.
func (a *App) installClient(prof config.Profile) *copr.Client {
	login, token := prof.Auth()
	return a.installRaw(prof.BaseURL(), copr.TokenAuth(login, token))
}

// installRaw caches a client built from raw inputs. A nil auth func leaves the
// request anonymous.
func (a *App) installRaw(baseURL string, auth copr.AuthFunc) *copr.Client {
	c := copr.New(baseURL, auth)
	a.client = c
	a.installChrootCatalog()
	return c
}

// ResetClient drops the cached API client so the next Client() call rebuilds
// it with the current profile credentials.
func (a *App) ResetClient() { a.client = nil }

// ReadClient returns an API client for read-only operations. It uses the
// configured profile when one exists, and otherwise falls back to an anonymous
// production client, because browsing, monitoring, and log reading are all
// anonymous operations that should not require configuration. Environment
// credentials are authoritative: when they are set and malformed the error is
// returned rather than silently degraded to an anonymous client.
func (a *App) ReadClient() (*copr.Client, error) {
	if a.client != nil {
		return a.client, nil
	}
	if a.Cfg == nil || !a.Cfg.Matches(a.cfgPath, a.legacy) {
		a.Cfg = config.New(a.cfgPath, a.legacy)
	}
	if env, _, ok, err := config.EnvironmentProfile(); err != nil {
		return nil, err
	} else if ok {
		return a.installClient(env), nil
	}
	prof, err := a.Cfg.Profile(a.profile)
	if err != nil {
		// No profile or legacy config: use anonymous production reads.
		return a.installRaw(config.DefaultProductionURL, nil), nil
	}
	return a.installClient(prof), nil
}

// profileForUse returns the effective profile for API calls, rebuilding the
// manager when the config paths changed via flags, and reporting the
// environment source when credentials came from the environment.
func (a *App) profileForUse() (config.Profile, config.EnvSource, error) {
	if a.Cfg == nil || !a.Cfg.Matches(a.cfgPath, a.legacy) {
		a.Cfg = config.New(a.cfgPath, a.legacy)
	}
	return a.Cfg.Effective(a.profile)
}

// Username returns the effective account username. A configured username wins;
// otherwise the identity is resolved once from the credentials via auth-check
// and cached for the lifetime of the process. It returns "" when the username
// cannot be determined, which keeps anonymous read paths working unchanged.
func (a *App) Username(ctx context.Context) string {
	if a.username != "" {
		return a.username
	}
	prof, _, err := a.profileForUse()
	if err != nil {
		return ""
	}
	if prof.Username != "" {
		return prof.Username
	}
	// Resolving the identity needs credentials; skip the round trip otherwise.
	if prof.Token == "" && prof.TokenCommand == "" && prof.SecretHandler == "" {
		return ""
	}
	c, err := a.Client()
	if err != nil {
		return ""
	}
	identity, err := c.AuthCheck(ctx)
	if err != nil || identity.Name == "" {
		return ""
	}
	a.username = identity.Name
	return a.username
}

// profileName returns the effective profile name (flag or default).
func profileName(a *App) string {
	if a.profile != "" {
		return a.profile
	}
	return a.Cfg.DefaultProfileName()
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
		newAuthCmd(app),
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
		newCacheCmd(app),
		newRpmCmd(app),
		newVersionCmd(),
	)
	return root
}
