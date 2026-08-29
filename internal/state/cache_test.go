package state

import (
	"path/filepath"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestChrootCacheStoreLoadClear(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	c := NewChrootCache(dir)

	if _, ok := c.Load(); ok {
		t.Fatal("empty cache should not load")
	}

	want := copr.MockChroots{"fedora-44-x86_64": "Fedora 44 x86_64"}
	if err := c.Store(want); err != nil {
		t.Fatal(err)
	}

	got, ok := c.Load()
	if !ok {
		t.Fatal("expected cache to load after store")
	}
	if len(*got) != 1 || (*got)["fedora-44-x86_64"] != "Fedora 44 x86_64" {
		t.Fatalf("loaded cache = %v, want %v", got, want)
	}

	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Load(); ok {
		t.Fatal("cache should be empty after clear")
	}
}

func TestChrootCacheClearMissingIsNoop(t *testing.T) {
	c := NewChrootCache(filepath.Join(t.TempDir(), "cache"))
	if err := c.Clear(); err != nil {
		t.Fatalf("clearing a missing cache should not error: %v", err)
	}
}
