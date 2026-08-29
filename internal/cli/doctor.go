package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/config"
)

// newDoctorCmd diagnoses the environment: config presence, auth, and
// connectivity. Each check is reported as a line; the command exits non-zero
// (with a structured error) when any check fails, so it works as a probe.
func newDoctorCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose environment issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			failed := 0
			// Reconcile config paths from flags before checking.
			if app.Cfg == nil || !app.Cfg.Matches(app.cfgPath, app.legacy) {
				app.Cfg = config.New(app.cfgPath, app.legacy)
			}
			prof, err := app.Cfg.Profile(app.profile)
			if err != nil {
				fmt.Fprintf(out, "FAIL config: %v\n", err)
				return err
			}
			fmt.Fprintln(out, "ok   config: profile loaded")

			login, tok, aerr := prof.AuthErr()
			if aerr != nil {
				fmt.Fprintf(out, "FAIL auth: %v\n", aerr)
				return aerr
			}
			if login == "" || tok == "" {
				fmt.Fprintln(out, "WARN auth: no credentials configured")
			} else {
				fmt.Fprintln(out, "ok   auth: credentials present")
			}
			if prof.TokenExpiry != "" {
				fmt.Fprintf(out, "ok   auth: token expiry %s\n", prof.TokenExpiry)
			}

			c, cerrr := app.ReadClient()
			if cerrr != nil {
				fmt.Fprintf(out, "FAIL network: %v\n", cerrr)
				return cerrr
			}
			_, err = c.ListMockChroots(cmd.Context())
			if err != nil {
				fmt.Fprintf(out, "FAIL network: cannot reach instance: %v\n", err)
				failed++
			} else {
				fmt.Fprintln(out, "ok   network: instance reachable")
			}

			if failed > 0 {
				return cerr.New("doctor_failed", cerr.ExitGeneric, "doctor found environment problems")
			}
			return nil
		},
	}
	return cmd
}
