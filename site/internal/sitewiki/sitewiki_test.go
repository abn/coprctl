package sitewiki

import (
	"encoding/json"
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
		"wiki/search.js",
		"wiki/search-index.json",
		"sitemap.xml",
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
		`<meta name="description" content="First steps.">`,
		`<aside class="sidebar"`,
		`<nav class="breadcrumb crumb-page"`,
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

	// Relay-style folding nav: only the active section unfolds.
	if got := strings.Count(q, `<details class="side-fold" open>`); got != 1 {
		t.Errorf("exactly one sidebar section should unfold, got %d", got)
	}
	if !strings.Contains(q, `<li class="active"><a href="/wiki/usage/quickstart.html">`) {
		t.Errorf("quickstart sidebar should mark the current page active")
	}
	// The drawer holds the nav and the page holds the breadcrumb; a swapped
	// shell argument once put each in the other.
	aside := q[strings.Index(q, "<aside"):]
	aside = aside[:strings.Index(aside, "</aside>")]
	if !strings.Contains(aside, `<nav class="side"`) || strings.Contains(aside, "crumb-sep") {
		t.Errorf("drawer aside should hold the section nav, not the breadcrumb")
	}
}

func TestSearchIndex(t *testing.T) {
	root := writeFixture(t)
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if err := r.RenderAll(root, out); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(out, "wiki/search-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []SearchEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("search-index.json is not valid JSON: %v", err)
	}
	var sectionHit, leadHit bool
	for _, e := range entries {
		if e.URL == "/wiki/usage/quickstart.html#install" && e.Doc == "Quick start" {
			sectionHit = true
			if !strings.Contains(e.Content, "coprctl") {
				t.Errorf("install chunk missing expected text: %q", e.Content)
			}
		}
		if e.URL == "/wiki/usage/instances.html" {
			leadHit = true
		}
		if e.URL == "" || e.Title == "" || e.Content == "" {
			t.Errorf("incomplete index row: %+v", e)
		}
	}
	if !sectionHit {
		t.Errorf("no section-anchored row for quickstart#install")
	}
	if !leadHit {
		t.Errorf("no lead-text row for instances")
	}

	js, err := os.ReadFile(filepath.Join(out, "wiki/search.js"))
	if err != nil || len(js) == 0 {
		t.Errorf("search.js missing or empty")
	}
}

func TestSitemap(t *testing.T) {
	root := writeFixture(t)
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if err := r.RenderAll(root, out); err != nil {
		t.Fatal(err)
	}
	sm := read(t, filepath.Join(out, "sitemap.xml"))
	for _, want := range []string{
		"https://coprctl.abn.is/wiki/usage/quickstart.html",
		"https://coprctl.abn.is/wiki/",
	} {
		if !strings.Contains(sm, want) {
			t.Errorf("sitemap.xml missing %q", want)
		}
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
		{"index", "../CHANGELOG.md", "/wiki/changelog.html"},
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
