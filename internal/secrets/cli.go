package secrets

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/abn/coprctl/internal/cerr"
)

// cliBackend is a backend driven by a CLI binary with get/set/delete verbs.
type cliBackend struct {
	name string
	bin  string
	get  []string
	set  []string
	del  []string
}

func (b *cliBackend) Name() string { return b.name }

func (b *cliBackend) Available() error {
	if _, err := exec.LookPath(b.bin); err != nil {
		return err
	}
	return nil
}

func (b *cliBackend) run(args ...string) (string, error) {
	cmd := exec.Command(b.bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", cerr.New("secret_backend", cerr.ExitConfig,
			b.name+" failed").Wrap(err)
	}
	return strings.TrimSpace(out.String()), nil
}

func (b *cliBackend) Get(key string) (string, error) {
	args := append(append([]string{}, b.get...), key)
	return b.run(args...)
}

func (b *cliBackend) Set(key, value string) error {
	// Most CLIs read the value from stdin on insert.
	args := append(append([]string{}, b.set...), key)
	cmd := exec.Command(b.bin, args...)
	cmd.Stdin = strings.NewReader(value + "\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return cerr.New("secret_backend", cerr.ExitConfig, b.name+" failed").Wrap(err)
	}
	return nil
}

func (b *cliBackend) Delete(key string) error {
	args := append(append([]string{}, b.del...), key)
	_, err := b.run(args...)
	return err
}
