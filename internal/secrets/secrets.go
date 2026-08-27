// Package secrets abstracts storing and retrieving secret values in a system
// secret handler (pass, gopass, secret-tool/libsecret, 1Password), so tokens
// never need to live in the config file. When no handler is configured or
// available, values fall back to plaintext in the config.
package secrets

// Backend is a secret handler.
type Backend interface {
	// Name is the backend name (pass, gopass, secret-tool, op).
	Name() string
	// Available reports whether the backend is usable.
	Available() error
	// Get retrieves the secret for key.
	Get(key string) (string, error)
	// Set stores the secret for key.
	Set(key, value string) error
	// Delete removes the secret for key.
	Delete(key string) error
}

// handler is the configured/auto-detected backend.
type handler struct {
	backend Backend
}

// Detect returns a Backend for the preferred name, or the first available one.
// When prefer is empty, it auto-detects in a stable order. The op (1Password)
// backend is not yet supported because its item references and stdin handling
// differ; pass, gopass, and secret-tool are the supported handlers.
func Detect(prefer string) Backend {
	candidates := []Backend{}
	if prefer != "" {
		candidates = append(candidates, newNamed(prefer))
	} else {
		candidates = []Backend{newNamed("pass"), newNamed("gopass"), newNamed("secret-tool")}
	}
	for _, c := range candidates {
		if c != nil && c.Available() == nil {
			return c
		}
	}
	return nil
}

func newNamed(name string) Backend {
	switch name {
	case "pass":
		return &cliBackend{name: "pass", bin: "pass",
			get: []string{"show"}, set: []string{"insert", "-f"}, del: []string{"rm", "-f"}}
	case "gopass":
		return &cliBackend{name: "gopass", bin: "gopass",
			get: []string{"show"}, set: []string{"insert", "-f"}, del: []string{"rm", "-f"}}
	case "secret-tool":
		return &secretToolBackend{}
	}
	return nil
}
