// Command coprctl is a reimagined CLI and agent interface for the Fedora Copr
// build system.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/cli"
)

func main() {
	app := cli.NewApp()
	root := cli.Root(app)
	if err := root.Execute(); err != nil {
		reportError(err)
		os.Exit(cerr.ExitCodeFor(err))
	}
}

// reportError prints a structured error object to stderr when the output is not
// an interactive terminal, and a human-readable line otherwise.
func reportError(err error) {
	var ce *cerr.Error
	if errors.As(err, &ce) && !term.IsTerminal(int(os.Stderr.Fd())) {
		enc := json.NewEncoder(os.Stderr)
		_ = enc.Encode(ce)
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
}
