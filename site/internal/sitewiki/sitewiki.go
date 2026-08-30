// Package sitewiki renders the docs/ OKF bundle into static HTML for the
// Cloudflare Workers site. Workers serves whatever static files land in site/,
// so this runs at build time (make site) and emits site/wiki/**.
package sitewiki

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// Section is a top-level docs/ directory (usage, design, adr, ...).
type Section struct {
	ID    string // directory name
	Title string // display title
	Pages []Page
}

// Page is one rendered doc page.
type Page struct {
	Section string
	Slug    string // relative path without extension, e.g. usage/quickstart
	Title   string
	Type    string
	Status  string
	Body    string // rendered HTML body (without H1, which is the title)
	TOC     []TOCEntry
}

// TOCEntry is a heading in the page body.
type TOCEntry struct {
	ID    string
	Level int
	Text  string
}

// Renderer renders the docs bundle.
type Renderer struct {
	md       goldmark.Markdown
	sections []Section
}

// New builds a renderer. root is the docs/ directory.
func New(root string) (*Renderer, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	r := &Renderer{md: md}
	if err := r.load(root); err != nil {
		return nil, err
	}
	return r, nil
}

// Sections returns the sections in canonical order.
func (r *Renderer) Sections() []Section { return r.sections }

// RenderAll renders every page to site/wiki/. out is the site dir root.
func (r *Renderer) RenderAll(docsRoot, out string) error {
	for _, s := range r.sections {
		for _, p := range s.Pages {
			rel := p.Slug + ".html"
			if p.Slug == s.ID {
				rel = s.ID + "/index.html"
			}
			dst := filepath.Join(out, "wiki", filepath.Dir(rel))
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			html := r.RenderPage(p)
			if err := os.WriteFile(filepath.Join(out, "wiki", rel), []byte(html), 0o644); err != nil {
				return err
			}
		}
	}
	// Write the shared stylesheet.
	css, err := CSS()
	if err != nil {
		return err
	}
	dir := filepath.Join(out, "wiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "wiki.css"), css, 0o644)
}

// load walks the docs tree, building sections and pages.
func (r *Renderer) load(root string) error {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	sectionTitles := map[string]string{
		"":             "Documentation",
		"usage":        "Usage",
		"design":       "Design",
		"architecture": "Architecture",
		"adr":          "Decisions",
		"contribution": "Contribution",
		"reference":    "Reference",
	}
	order := []string{"", "usage", "design", "architecture", "adr", "contribution", "reference"}
	// Add any dirs not in the canonical order, sorted.
	var extra []string
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		found := false
		for _, o := range order {
			if o == d.Name() {
				found = true
				break
			}
		}
		if !found {
			extra = append(extra, d.Name())
		}
	}
	sort.Strings(extra)

	for _, name := range append(order, extra...) {
		if name == "" {
			if err := r.loadTopLevel(root); err != nil {
				return err
			}
			continue
		}
		dir := filepath.Join(root, name)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		sec, err := r.loadSection(dir, name, sectionTitles[name])
		if err != nil {
			return err
		}
		r.sections = append(r.sections, sec)
	}
	return nil
}

func (r *Renderer) loadTopLevel(root string) error {
	sec := Section{ID: "", Title: "Documentation"}
	files, _ := filepath.Glob(filepath.Join(root, "*.md"))
	sort.Strings(files)
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".md")
		if base == "log" {
			continue // evolution log is not a page
		}
		p, err := r.parsePage("", base, f)
		if err != nil {
			return err
		}
		sec.Pages = append(sec.Pages, p)
	}
	if len(sec.Pages) > 0 {
		sort.Slice(sec.Pages, func(i, j int) bool {
			return sec.Pages[i].Slug < sec.Pages[j].Slug
		})
		r.sections = append(r.sections, sec)
	}
	return nil
}

func (r *Renderer) loadSection(dir, id, title string) (Section, error) {
	sec := Section{ID: id, Title: title}
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	sort.Strings(files)
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".md")
		p, err := r.parsePage(id, id+"/"+base, f)
		if err != nil {
			return sec, err
		}
		sec.Pages = append(sec.Pages, p)
	}
	sort.Slice(sec.Pages, func(i, j int) bool {
		if sec.Pages[i].Slug == id+"/index" {
			return true
		}
		if sec.Pages[j].Slug == id+"/index" {
			return false
		}
		return sec.Pages[i].Title < sec.Pages[j].Title
	})
	return sec, nil
}

// frontmatter holds the OKF fields we render.
type frontmatter struct {
	Title       string `yaml:"title"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"`
	OKFVersion  string `yaml:"okf_version"`
}

func (r *Renderer) parsePage(section, slug, file string) (Page, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Page{}, err
	}
	fm, body := splitFrontmatter(data)

	p := Page{Section: section, Slug: slug}
	p.Type = fm.Type
	p.Status = fm.Status
	p.Title = fm.Title

	doc := r.md.Parser().Parse(text.NewReader(body))
	src := body
	usedIDs := map[string]int{}
	var h1 string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch nn := n.(type) {
		case *ast.Link:
			nn.Destination = []byte(linkPath(p.Slug, string(nn.Destination)))
		}
		if h := headingText(n, src); h != "" && p.Title == "" && isH1(n) {
			h1 = h
			p.Title = h
		}
		if isHeading(n) && !isH1(n) {
			h := n.(*ast.Heading)
			// Regenerate the heading id from clean visible text so TOC anchors
			// and labels are readable (goldmark's auto-id includes link URLs).
			text := headingText(n, src)
			id := slugify(text)
			if id == "" {
				id = "section"
			}
			seenID := usedIDs[id]
			usedIDs[id]++
			if seenID > 0 {
				id = fmt.Sprintf("%s-%d", id, seenID)
			}
			h.SetAttributeString("id", []byte(id))
			p.TOC = append(p.TOC, TOCEntry{ID: id, Level: h.Level, Text: text})
		}
		return ast.WalkContinue, nil
	})
	if p.Title == "" {
		p.Title = h1
	}
	if p.Title == "" {
		p.Title = strings.Title(strings.ReplaceAll(filepath.Base(file), "-", " "))
	}

	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, body, doc); err != nil {
		return Page{}, err
	}
	p.Body = buf.String()
	return p, nil
}

func isH1(n ast.Node) bool {
	h, ok := n.(*ast.Heading)
	return ok && h.Level == 1
}

func isHeading(n ast.Node) bool {
	_, ok := n.(*ast.Heading)
	return ok
}

func headingText(n ast.Node, src []byte) string {
	h, ok := n.(*ast.Heading)
	if !ok {
		return ""
	}
	var b strings.Builder
	for c := h.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.Link:
			// Include link text (e.g. the version "0.5.0" in a release heading)
			// but not the destination URL.
			for lc := t.FirstChild(); lc != nil; lc = lc.NextSibling() {
				if lt, ok := lc.(*ast.Text); ok {
					b.Write(lt.Segment.Value(src))
				}
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// slugify converts heading text to a url-friendly id.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "section"
	}
	return out
}

// splitFrontmatter returns (frontmatter, body). Body keeps the H1 if present.
func splitFrontmatter(data []byte) (frontmatter, []byte) {
	var fm frontmatter
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return fm, data
	}
	end := bytes.Index(data[4:], []byte("\n---"))
	if end < 0 {
		return fm, data
	}
	end += 4
	_ = yaml.Unmarshal(data[4:end], &fm)
	return fm, data[end+4:]
}

// linkPath rewrites a docs-relative markdown link to a wiki HTML path.
func linkPath(from, href string) string {
	if href == "" || strings.HasPrefix(href, "#") {
		return href
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "mailto:") {
		return href
	}
	href = strings.SplitN(href, "#", 2)[0]
	if href == "" {
		return href
	}
	base := path.Dir(from)
	target := path.Clean(path.Join(base, href))
	if strings.HasSuffix(target, ".md") {
		target = strings.TrimSuffix(target, ".md")
	}
	if target == "" || target == "." {
		return ""
	}
	url := "/wiki/" + target + ".html"
	if strings.HasSuffix(target, "/index") {
		url = "/wiki/" + strings.TrimSuffix(target, "/index") + "/"
	}
	return url
}
