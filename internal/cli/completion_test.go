package cli

import (
	"os"
	"testing"

	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/state"
)

func TestCachedChrootNames(t *testing.T) {
	// Isolate the cache under a temp XDG_CACHE_HOME.
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	app := NewApp()

	cacheDir := dir + "/coprctl"
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := state.NewChrootCache(cacheDir)
	catalog := copr.MockChroots{
		"fedora-43-x86_64":      "",
		"fedora-rawhide-x86_64": "",
		"epel-9-x86_64":         "",
	}
	if err := cache.Store(catalog); err != nil {
		t.Fatal(err)
	}

	names := cachedChrootNames(app)
	if len(names) != 3 {
		t.Fatalf("got %d chroots: %v", len(names), names)
	}
	for _, n := range names {
		if _, ok := catalog[n]; !ok {
			t.Errorf("unexpected chroot %q", n)
		}
	}
}

func TestChrootCompleterMatches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	app := NewApp()

	cacheDir := dir + "/coprctl"
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := state.NewChrootCache(cacheDir)
	_ = cache.Store(copr.MockChroots{
		"fedora-43-x86_64":      "",
		"fedora-rawhide-x86_64": "",
		"epel-9-x86_64":         "",
	})

	completer := chrootCompleter(app)
	out, directive := completer(nil, nil, "fedora")
	if directive != 4 { // ShellCompDirectiveNoFileComp
		t.Errorf("directive = %d", directive)
	}
	if len(out) != 2 {
		t.Errorf("matches = %v", out)
	}
	for _, m := range out {
		if m != "fedora-43-x86_64" && m != "fedora-rawhide-x86_64" {
			t.Errorf("unexpected match %q", m)
		}
	}
}
