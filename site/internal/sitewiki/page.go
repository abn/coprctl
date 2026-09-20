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
		// <details> gives a no-JS collapsible, closed by default on every
		// viewport; the summary keeps it one tap away on mobile.
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
			// Trust tiers read muted until verified: a draft page must not
			// wear the confident green of a stable one.
			cls := "chip chip-status"
			if p.Status == "draft" || p.Status == "unverified" {
				cls += " chip-muted"
			}
			chips = append(chips, fmt.Sprintf(`<span class="%s">%s</span>`, cls, template.HTMLEscapeString(p.Status)))
		}
		meta = `<div class="page-meta">` + strings.Join(chips, "") + `</div>`
	}

	title := p.Title
	if title == "" {
		title = p.Slug
	}
	desc := p.Description
	if desc == "" {
		desc = "coprctl documentation"
	}

	return renderShell(r, p, tocHTML, meta, title, desc)
}

// renderShell builds the full HTML document.
func renderShell(r *Renderer, p Page, tocHTML, meta, title, desc string) string {
	nav := r.sidebarHTML(p)
	breadcrumb := r.breadcrumbHTML(p)
	body := p.Body
	body = stripLeadingH1(body, title)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s · coprctl</title>
<meta name="description" content="%s">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:type" content="article">
<link rel="stylesheet" href="/wiki/wiki.css">
</head>
<body>
<a class="skip" href="#content">Skip to content</a>
<header>
  <nav class="wrap">
    <div class="brand"><a href="/wiki/"><span class="dollar">$</span>copr<b>ctl</b></a></div>
    <div class="nav-right">
    <ul>
      <li><a href="/">Home</a></li>
      <li><a href="/wiki/">Docs</a></li>
      <li><a href="https://github.com/abn/coprctl" target="_blank" rel="noopener">GitHub ↗</a></li>
    </ul>
    <div class="nav-search"><button class="search-btn" id="searchBtn" aria-label="Search documentation" title="Search ( / )"><svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><circle cx="7" cy="7" r="5"/><line x1="11.2" y1="11.2" x2="14.5" y2="14.5"/></svg></button></div>
    </div>
  </nav>
</header>
<div class="drawer-backdrop" id="drawerBackdrop"></div>
<div class="layout wrap">
  <aside class="sidebar" id="sideDrawer" style="transform:translateX(-100%%)">%s</aside>
  <main class="content" id="content">
    <div class="crumb-bar">
      <button class="menu-btn" id="menuBtn" aria-label="Toggle navigation" aria-expanded="false" aria-controls="sideDrawer">☰</button>
      <nav class="breadcrumb crumb-page" aria-label="Breadcrumb">%s</nav>
    </div>
    <article>
      <h1>%s</h1>
      %s
      %s
      %s
    </article>
  </main>
</div>
<script>
  // Slide-out navigation drawer with focus trap: while open, Tab cycles
  // inside the drawer; closing returns focus to the invoking control.
  var menuBtn = document.getElementById('menuBtn');
  var drawer = document.getElementById('sideDrawer');
  var backdrop = document.getElementById('drawerBackdrop');
  var lastFocus = null;
  function drawerItems() {
    return drawer.querySelectorAll('a[href],button:not([disabled])');
  }
  function openDrawer() {
    lastFocus = document.activeElement;
    drawer.classList.add('open');
    backdrop.classList.add('open');
    menuBtn.setAttribute('aria-expanded', 'true');
    var items = drawerItems();
    if (items.length) items[0].focus();
  }
  function closeDrawer() {
    if (!drawer.classList.contains('open')) return;
    drawer.classList.remove('open');
    backdrop.classList.remove('open');
    menuBtn.setAttribute('aria-expanded', 'false');
    if (lastFocus && lastFocus.focus) lastFocus.focus();
  }
  menuBtn.addEventListener('click', function () {
    drawer.classList.contains('open') ? closeDrawer() : openDrawer();
  });
  backdrop.addEventListener('click', closeDrawer);
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') { closeDrawer(); return; }
    if (e.key !== 'Tab' || !drawer.classList.contains('open')) return;
    var items = drawerItems();
    if (!items.length) return;
    var first = items[0], last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  });
</script>
<div class="search-backdrop" id="searchBackdrop" aria-hidden="true">
  <div class="search-modal" role="dialog" aria-modal="true" aria-label="Search documentation">
    <div class="search-head">
      <input type="search" id="searchInput" class="search-input" placeholder="Search docs..." autocomplete="off" aria-label="Search docs">
      <button type="button" id="searchCloseBtn" class="search-close" aria-label="Close search"><kbd>ESC</kbd></button>
    </div>
    <div class="search-results" id="searchResults"></div>
    <div class="search-foot"><span><kbd>/</kbd> or <kbd>Ctrl K</kbd> to search</span><span><kbd>Enter</kbd> to open</span></div>
  </div>
</div>
<script src="/wiki/search.js" defer></script>
<footer>
  <div class="wrap">
    <div>coprctl documentation</div>
    <div class="mono">OKF v0.2</div>
  </div>
</footer>
</body>
</html>`,
		template.HTMLEscapeString(title),
		template.HTMLEscapeString(desc),
		template.HTMLEscapeString(title),
		template.HTMLEscapeString(desc),
		nav, breadcrumb,
		template.HTMLEscapeString(title), meta, tocHTML, body)
}

// sidebarHTML builds the left navigation grouped by section. Only the
// active section unfolds; the rest stay collapsed until opened.
func (r *Renderer) sidebarHTML(p Page) string {
	var b strings.Builder
	b.WriteString(`<nav class="side" aria-label="Sections">`)
	for _, s := range r.sections {
		b.WriteString(`<div class="side-sec">`)
		href := "/wiki/"
		if s.ID != "" {
			href = "/wiki/" + s.ID + "/"
		}
		open := ""
		if s.ID == p.Section {
			open = " open"
		}
		fmt.Fprintf(&b, `<details class="side-fold"%s><summary class="side-title"><a href="%s">%s</a></summary>`, open, href, template.HTMLEscapeString(s.Title))
		b.WriteString(`<ul>`)
		for _, pg := range s.Pages {
			// The section header already links to the index page; do not list
			// it again in the section's page list.
			if pg.Slug == "index" || strings.HasSuffix(pg.Slug, "/index") {
				continue
			}
			href := "/wiki/" + pageURL(pg)
			cls := ""
			if pg.Slug == p.Slug {
				cls = ` class="active"`
			}
			fmt.Fprintf(&b, `<li%s><a href="%s">%s</a></li>`, cls, href, template.HTMLEscapeString(pg.Title))
		}
		b.WriteString(`</ul></details>`)
		b.WriteString(`</div>`)
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
