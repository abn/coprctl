// coprctl wiki search: zero-dependency client over wiki/search-index.json.
// The index loads lazily on first use; entries carry compact keys:
// u=url, p=page url, t=title, d=page title, s=section, c=content.
(function(){
  var btn = document.getElementById("searchBtn");
  var backdrop = document.getElementById("searchBackdrop");
  var input = document.getElementById("searchInput");
  var results = document.getElementById("searchResults");
  if (!btn || !backdrop || !input || !results) return;

  var index = null, loading = false, lastFocus = null, active = -1, current = [];

  function load(){
    if (index || loading) return Promise.resolve(index || []);
    loading = true;
    return fetch("/wiki/search-index.json").then(function(res){
      return res.json();
    }).then(function(data){
      index = data; loading = false; return index;
    }).catch(function(){
      loading = false; return [];
    });
  }
  btn.addEventListener("mouseenter", load);
  btn.addEventListener("focus", load);

  function open(){
    lastFocus = document.activeElement;
    backdrop.setAttribute("aria-hidden", "false");
    backdrop.classList.add("open");
    input.value = "";
    render([], "");
    active = -1;
    load().then(function(){ input.focus(); });
  }
  function close(){
    backdrop.classList.remove("open");
    backdrop.setAttribute("aria-hidden", "true");
    if (lastFocus && lastFocus.focus) lastFocus.focus();
  }
  btn.addEventListener("click", open);
  backdrop.addEventListener("click", function(e){ if (e.target === backdrop) close(); });
  document.getElementById("searchCloseBtn").addEventListener("click", close);
  document.addEventListener("keydown", function(e){
    var typing = /^(INPUT|TEXTAREA)$/.test(document.activeElement && document.activeElement.tagName || "");
    if ((e.key === "/" && !typing) || ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k")) {
      e.preventDefault(); open();
    } else if (e.key === "Escape" && backdrop.classList.contains("open")) {
      close();
    }
  });

  function score(entry, terms){
    var t = (entry.t || "").toLowerCase();
    var d = (entry.d || "").toLowerCase();
    var s = (entry.s || "").toLowerCase();
    var c = (entry.c || "").toLowerCase();
    var total = 0;
    for (var i = 0; i < terms.length; i++) {
      var q = terms[i], hit = false;
      if (t.indexOf(terms.join(" ")) !== -1 && i === 0) { total += 80; hit = true; }
      if (t.indexOf(q) !== -1) { total += (t.indexOf(q) === 0 ? 85 : 45); hit = true; }
      else if (d.toLowerCase().indexOf(q) !== -1) { total += 25; hit = true; }
      else if (s.indexOf(q) !== -1) { total += 15; hit = true; }
      else if (c.indexOf(q) !== -1) { total += 10; hit = true; }
      if (!hit) return -1;
    }
    return total;
  }

  function esc(s){
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  function snippet(entry, terms){
    var c = entry.c || "";
    var low = c.toLowerCase(), pos = -1, qi = 0;
    for (var i = 0; i < terms.length; i++) {
      var p = low.indexOf(terms[i]);
      if (p !== -1 && (pos === -1 || p < pos)) { pos = p; qi = i; }
    }
    if (pos === -1) return esc(c.slice(0, 120));
    var from = Math.max(0, pos - 35), to = Math.min(c.length, pos + 95);
    var out = esc(c.slice(from, to));
    var q = esc(terms[qi]).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return out.replace(new RegExp("(" + q + ")", "ig"), "<mark>$1</mark>");
  }

  function render(items, q){
    current = items; active = -1;
    results.innerHTML = "";
    if (!q) return;
    if (!items.length) {
      results.innerHTML = '<p class="search-empty">No matches in the docs.</p>';
      return;
    }
    var list = document.createElement("div");
    list.setAttribute("role", "listbox");
    list.setAttribute("aria-label", "Search results");
    items.forEach(function(it, i){
      var a = document.createElement("a");
      a.className = "search-hit";
      a.setAttribute("role", "option");
      a.setAttribute("aria-selected", "false");
      a.href = it.u;
      a.innerHTML = '<div class="search-hit-title">' + esc(it.t) + '</div>' +
        '<div class="search-hit-crumb">' + esc(it.d) + ' / ' + esc(it.s) + '</div>' +
        '<div class="search-hit-snip">' + snippet(it, q) + '</div>';
      a.addEventListener("mousemove", function(){ setActive(i); });
      list.appendChild(a);
    });
    results.appendChild(list);
  }
  function setActive(i){
    var opts = results.querySelectorAll('[role="option"]');
    opts.forEach(function(o){ o.setAttribute("aria-selected", "false"); o.classList.remove("on"); });
    active = i;
    if (opts[i]) {
      opts[i].setAttribute("aria-selected", "true");
      opts[i].classList.add("on");
      opts[i].scrollIntoView({block: "nearest"});
    }
  }

  var debounce = null;
  input.addEventListener("input", function(){
    clearTimeout(debounce);
    debounce = setTimeout(function(){
      var terms = input.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
      if (!terms.length || !index) { render([], input.value); return; }
      var ranked = [];
      index.forEach(function(e){
        var sc = score(e, terms);
        if (sc >= 0) ranked.push({e: e, sc: sc});
      });
      ranked.sort(function(a, b){ return b.sc - a.sc; });
      render(ranked.slice(0, 12).map(function(r){ return r.e; }), terms);
    }, 120);
  });
  input.addEventListener("keydown", function(e){
    var opts = results.querySelectorAll('[role="option"]');
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!opts.length) return;
      var i = active + (e.key === "ArrowDown" ? 1 : -1);
      setActive((i + opts.length) % opts.length);
    } else if (e.key === "Enter" && opts[Math.max(0, active)]) {
      window.location.href = current[Math.max(0, active)].u;
    }
  });
})();
