package sitewiki

import (
	"strings"

	"github.com/yuin/goldmark/ast"
)

// PageSection is one indexable chunk of a page: the lead text before the
// first heading, or one H2/H3 section with its anchor id.
type PageSection struct {
	ID    string
	Title string
	Level int
	Text  string
}

// SearchEntry is one row of wiki/search-index.json. Keys stay short to
// keep the static index small.
type SearchEntry struct {
	URL     string `json:"u"` // deep link (/wiki/path.html#anchor)
	PageURL string `json:"p"` // canonical page URL (/wiki/path.html)
	Title   string `json:"t"` // section heading or page title
	Doc     string `json:"d"` // parent page title
	Section string `json:"s"` // parent bundle section (e.g. Usage)
	Content string `json:"c"` // plain section text, truncated
}

// searchEntries flattens every page section into index rows.
func (r *Renderer) searchEntries() []SearchEntry {
	var out []SearchEntry
	for _, s := range r.sections {
		for _, p := range s.Pages {
			base := "/wiki/" + pageURL(p)
			for _, sec := range p.Sections {
				u := base
				if sec.ID != "" {
					u += "#" + sec.ID
				}
				title := sec.Title
				if title == "" {
					title = p.Title
				}
				out = append(out, SearchEntry{
					URL:     u,
					PageURL: base,
					Title:   title,
					Doc:     p.Title,
					Section: s.Title,
					Content: sec.Text,
				})
			}
		}
	}
	return out
}

// sectionCollector accumulates plain text per section during the AST walk.
type sectionCollector struct {
	sections []PageSection
	cur      *PageSection
	inHead   int
}

func (c *sectionCollector) enterHeading(id, title string, level int) {
	c.finish()
	if level < 2 {
		return
	}
	c.cur = &PageSection{ID: id, Title: title, Level: level}
}

func (c *sectionCollector) addText(s string) {
	if c.cur == nil {
		c.cur = &PageSection{}
	}
	if c.cur.Text != "" {
		c.cur.Text += " "
	}
	c.cur.Text += s
}

func (c *sectionCollector) finish() {
	if c.cur == nil {
		return
	}
	c.cur.Text = normalizeSpace(c.cur.Text)
	if r := []rune(c.cur.Text); len(r) > 800 {
		c.cur.Text = string(r[:800])
	}
	c.sections = append(c.sections, *c.cur)
	c.cur = nil
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// collectSectionText reports whether n carries indexable text, and if so
// appends it to the current section. Call for every node entered outside a
// heading; code blocks contribute their raw lines.
func (c *sectionCollector) node(n ast.Node, src []byte) {
	switch t := n.(type) {
	case *ast.Text:
		c.addText(string(t.Segment.Value(src)))
	case *ast.String:
		c.addText(string(t.Value))
	case *ast.FencedCodeBlock:
		lines := t.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			c.addText(string(seg.Value(src)))
		}
	}
}
