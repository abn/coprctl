package sitewiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture creates a minimal docs tree under a temp dir.
func writeFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "docs")
	must := func(path, content string) {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("index.md", "---\nokf_version: \"0.2\"\n---\n\n# Docs\n\n* [Usage](usage/index.md)\n* [Quickstart](usage/quickstart.md)\n")
	must("overview.md", "# Overview\n\nSee the [usage](usage/index.md) section.\n")
	must("usage/index.md", "# Usage\n\nGuides.\n")
	must("usage/quickstart.md", "---\ntype: Guide\ntitle: Quick start\ndescription: First steps.\nstatus: stable\n---\n\n# Quick start\n\n## Install\n\nRun `coprctl`.\n\n## Next\n\nSee [instances](instances.md).\n")
	must("usage/instances.md", "---\ntype: Guide\ntitle: Instances\nstatus: draft\n---\n\n# Instances\n\nStaging guide.\n")
	must("log.md", "# Log\n\nNot a page.\n")
	return root
}

func TestRenderAll(t *testing.T) {
	root := writeFixture(t)
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if err := r.RenderAll(root, out); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"wiki/index.html",
		"wiki/overview.html",
		"wiki/usage/index.html",
		"wiki/usage/quickstart.html",
		"wiki/usage/instances.html",
		"wiki/wiki.css",
	} {
		if _, err := os.Stat(filepath.Join(out, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "wiki/log.html")); !os.IsNotExist(err) {
		t.Errorf("log.md should not render a page")
	}
}

func TestRenderPageContent(t *testing.T) {
	root := writeFixture(t)
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if err := r.RenderAll(root, out); err != nil {
		t.Fatal(err)
	}

	q := read(t, filepath.Join(out, "wiki/usage/quickstart.html"))
	for _, want := range []string{
		"<h1>Quick start</h1>",
		`class="chip chip-type">Guide`,
		`class="chip chip-status">stable`,
		`href="#install"`,
		`href="/wiki/usage/instances.html"`,
	} {
		if !strings.Contains(q, want) {
			t.Errorf("quickstart.html missing %q", want)
		}
	}

	idx := read(t, filepath.Join(out, "wiki/index.html"))
	if !strings.Contains(idx, `href="/wiki/usage/quickstart.html"`) {
		t.Errorf("index cross-link to quickstart not rewritten")
	}
}

func TestLinkPath(t *testing.T) {
	cases := []struct{ from, href, want string }{
		{"index", "overview.md", "/wiki/overview.html"},
		{"usage/quickstart", "instances.md", "/wiki/usage/instances.html"},
		{"overview", "design/index.md", "/wiki/design/"},
		{"index", "usage/index.md", "/wiki/usage/"},
		{"usage/quickstart", "#install", "#install"},
		{"usage/quickstart", "https://example.com", "https://example.com"},
	}
	for _, c := range cases {
		if got := linkPath(c.from, c.href); got != c.want {
			t.Errorf("linkPath(%q,%q) = %q, want %q", c.from, c.href, got, c.want)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
