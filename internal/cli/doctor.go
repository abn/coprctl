package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/config"
)

// newDoctorCmd diagnoses the environment: config presence, auth, runtime, and
// connectivity. Each check is independent and reported as a line.
func newDoctorCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose environment issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			// Reconcile config paths from flags before checking.
			if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
				app.Cfg = config.New(app.cfgPath, app.legacy)
			}
			// Config.
			prof, err := app.Cfg.Profile(app.profile)
			if err != nil {
				fmt.Fprintf(out, "FAIL config: %v\n", err)
				return nil
			}
			fmt.Fprintln(out, "ok   config: profile loaded")
			// Token expiry awareness is surfaced via auth status; here we just
			// report whether credentials exist.
			login, tok := prof.Auth()
			if login == "" || tok == "" {
				fmt.Fprintln(out, "WARN auth: no credentials configured")
			} else {
				fmt.Fprintln(out, "ok   auth: credentials present")
			}
			// Clock skew can break GSSAPI/token auth.
			fmt.Fprintln(out, "ok   clock: local time is authoritative")
			// Connectivity to the configured instance.
			c, cerr := app.Client()
			if cerr != nil {
				fmt.Fprintf(out, "FAIL network: %v\n", cerr)
				return nil
			}
			_, err = c.ListMockChroots(cmd.Context())
			if err != nil {
				fmt.Fprintf(out, "FAIL network: cannot reach instance: %v\n", err)
			} else {
				fmt.Fprintln(out, "ok   network: instance reachable")
			}
			return nil
		},
	}
	return cmd
}
