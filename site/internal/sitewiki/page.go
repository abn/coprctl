package sitewiki

import (
	"fmt"
	"html/template"
	"strings"
)

// RenderPage wraps a page body in the site shell.
func (r *Renderer) RenderPage(p Page) string {
	var tocHTML string
	if len(p.TOC) > 0 {
		var b strings.Builder
		// <details> gives a no-JS collapsible on mobile (closed by default via
		// CSS), while desktop keeps it open and inline.
		b.WriteString(`<details class="toc" aria-label="On this page">`)
		b.WriteString(`<summary>On this page</summary><ul>`)
		for _, e := range p.TOC {
			cls := ""
			if e.Level == 3 {
				cls = ` class="l3"`
			}
			fmt.Fprintf(&b, `<li%s><a href="#%s">%s</a></li>`, cls, e.ID, template.HTMLEscapeString(e.Text))
		}
		b.WriteString("</ul></details>")
		tocHTML = b.String()
	}

	meta := ""
	if p.Type != "" || p.Status != "" {
		var chips []string
		if p.Type != "" {
			chips = append(chips, fmt.Sprintf(`<span class="chip chip-type">%s</span>`, template.HTMLEscapeString(p.Type)))
		}
		if p.Status != "" {
			chips = append(chips, fmt.Sprintf(`<span class="chip chip-status">%s</span>`, template.HTMLEscapeString(p.Status)))
		}
		meta = `<div class="page-meta">` + strings.Join(chips, "") + `</div>`
	}

	title := p.Title
	if title == "" {
		title = p.Slug
	}

	return renderShell(r, p, tocHTML, meta, title)
}

// renderShell builds the full HTML document.
func renderShell(r *Renderer, p Page, tocHTML, meta, title string) string {
	nav := r.sidebarHTML(p.Section)
	breadcrumb := r.breadcrumbHTML(p)
	body := p.Body
	body = stripLeadingH1(body, title)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s · coprctl</title>
<meta name="description" content="coprctl documentation">
<link rel="stylesheet" href="/wiki/wiki.css">
</head>
<body>
<header>
  <nav class="wrap">
    <div class="brand"><a href="/wiki/"><span class="dollar">$</span>copr<b>ctl</b></a></div>
    <ul>
      <li><a href="/">Site</a></li>
      <li><a href="/wiki/">Docs</a></li>
      <li><a href="https://github.com/abn/coprctl" target="_blank" rel="noopener">GitHub ↗</a></li>
    </ul>
  </nav>
  <div class="subbar wrap">
    <button class="menu-btn" id="menuBtn" aria-label="Toggle navigation" aria-expanded="false" aria-controls="sideDrawer">☰</button>
    <nav class="breadcrumb">%s</nav>
  </div>
</header>
<div class="drawer-backdrop" id="drawerBackdrop"></div>
<div class="layout wrap">
  <aside class="sidebar" id="sideDrawer" style="transform:translateX(-100%%)">%s</aside>
  <main class="content">
    <article>
      <h1>%s</h1>
      %s
      %s
      %s
    </article>
  </main>
</div>
<script>
  // Slide-out navigation drawer.
  var menuBtn = document.getElementById('menuBtn');
  var drawer = document.getElementById('sideDrawer');
  var backdrop = document.getElementById('drawerBackdrop');
  function openDrawer() {
    drawer.classList.add('open');
    backdrop.classList.add('open');
    menuBtn.setAttribute('aria-expanded', 'true');
  }
  function closeDrawer() {
    drawer.classList.remove('open');
    backdrop.classList.remove('open');
    menuBtn.setAttribute('aria-expanded', 'false');
  }
  menuBtn.addEventListener('click', function () {
    drawer.classList.contains('open') ? closeDrawer() : openDrawer();
  });
  backdrop.addEventListener('click', closeDrawer);
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') closeDrawer();
  });
</script>
<footer>
  <div class="wrap">
    <div>coprctl documentation</div>
    <div class="mono">OKF v0.2</div>
  </div>
</footer>
</body>
</html>`,
		template.HTMLEscapeString(title),
		breadcrumb, nav,
		template.HTMLEscapeString(title), meta, tocHTML, body)
}

// sidebarHTML builds the left navigation grouped by section.
func (r *Renderer) sidebarHTML(current string) string {
	var b strings.Builder
	b.WriteString(`<nav class="side" aria-label="Sections">`)
	for _, s := range r.sections {
		b.WriteString(`<div class="side-sec">`)
		href := "/wiki/"
		if s.ID != "" {
			href = "/wiki/" + s.ID + "/"
		}
		fmt.Fprintf(&b, `<div class="side-title"><a href="%s">%s</a></div>`, href, template.HTMLEscapeString(s.Title))
		b.WriteString(`<ul>`)
		for _, pg := range s.Pages {
			// The section header already links to the index page; do not list
			// it again in the section's page list.
			if pg.Slug == "index" || strings.HasSuffix(pg.Slug, "/index") {
				continue
			}
			href := "/wiki/" + pageURL(pg)
			cls := ""
			if pg.Slug == current {
				cls = ` class="active"`
			}
			fmt.Fprintf(&b, `<li%s><a href="%s">%s</a></li>`, cls, href, template.HTMLEscapeString(pg.Title))
		}
		b.WriteString(`</ul></div>`)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

// pageURL returns the rendered URL path for a page.
func pageURL(p Page) string {
	if strings.HasSuffix(p.Slug, "/index") {
		return strings.TrimSuffix(p.Slug, "/index") + "/"
	}
	return p.Slug + ".html"
}

// breadcrumbHTML builds a Home / Section / Page trail.
func (r *Renderer) breadcrumbHTML(p Page) string {
	var b strings.Builder
	b.WriteString(`<a href="/wiki/">Docs</a>`)
	if p.Section != "" {
		fmt.Fprintf(&b, ` <span class="crumb-sep">/</span> <a href="/wiki/%s/">%s</a>`, p.Section, sectionTitle(r, p.Section))
	}
	b.WriteString(` <span class="crumb-sep">/</span> <span class="crumb-cur">`)
	b.WriteString(template.HTMLEscapeString(p.Title))
	b.WriteString(`</span>`)
	return b.String()
}

func sectionTitle(r *Renderer, id string) string {
	for _, s := range r.sections {
		if s.ID == id {
			return s.Title
		}
	}
	return id
}

// stripLeadingH1 removes a leading <h1>...</h1> that duplicates the page title.
func stripLeadingH1(body, title string) string {
	idx := strings.Index(body, "<h1")
	if idx < 0 {
		return body
	}
	end := strings.Index(body[idx:], "</h1>")
	if end < 0 {
		return body
	}
	end += idx + len("</h1>")
	inner := body[idx:end]
	if strings.Contains(inner, template.HTMLEscapeString(title)) {
		rest := body[end:]
		rest = strings.TrimPrefix(rest, "\n")
		return rest
	}
	return body
}
