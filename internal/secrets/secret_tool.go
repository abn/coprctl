package secrets

import (
	"os/exec"
	"strings"

	"github.com/abn/coprctl/internal/cerr"
)

// secretToolBackend uses the freedesktop Secret Service via secret-tool. The
// secret is stored under an attribute keyed by the coprctl label.
type secretToolBackend struct{}

func (b *secretToolBackend) Name() string { return "secret-tool" }

func (b *secretToolBackend) Available() error {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return err
	}
	return nil
}

func (b *secretToolBackend) attrs(key string) []string {
	return []string{"application", "coprctl", "key", key}
}

// storeAttrs includes the --label used only by store.
func (b *secretToolBackend) storeArgs(key string) []string {
	return append([]string{"store", "--label", "coprctl"}, b.attrs(key)...)
}

func (b *secretToolBackend) Get(key string) (string, error) {
	args := append([]string{"lookup"}, b.attrs(key)...)
	out, err := exec.Command("secret-tool", args...).Output()
	if err != nil {
		return "", cerr.New("secret_backend", cerr.ExitConfig,
			"secret-tool lookup failed").Wrap(err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (b *secretToolBackend) Set(key, value string) error {
	cmd := exec.Command("secret-tool", b.storeArgs(key)...)
	cmd.Stdin = strings.NewReader(value + "\n")
	if err := cmd.Run(); err != nil {
		return cerr.New("secret_backend", cerr.ExitConfig,
			"secret-tool store failed").Wrap(err)
	}
	return nil
}

func (b *secretToolBackend) Delete(key string) error {
	args := append([]string{"clear"}, b.attrs(key)...)
	if err := exec.Command("secret-tool", args...).Run(); err != nil {
		return cerr.New("secret_backend", cerr.ExitConfig,
			"secret-tool clear failed").Wrap(err)
	}
	return nil
}
