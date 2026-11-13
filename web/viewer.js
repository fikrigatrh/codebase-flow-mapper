// CFM viewer. Graph strings are untrusted (they come from the analyzed repo): text goes in only
// through textContent or Cytoscape data, never innerHTML.
(function () {
  "use strict";
  var root = document.documentElement;
  var $ = function (id) { return document.getElementById(id); };
  var status = $("status");
  var MAX_ROWS = 500; // ponytail: plain list; virtualize if a repo has far more matches

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }
  function show(text, isError) {
    status.textContent = text;
    status.className = isError ? "error" : "";
    status.hidden = false;
  }

  // ---- theme ----
  function setTheme(t) {
    root.dataset.theme = t;
    $("theme").textContent = t === "dark" ? "Light theme" : "Dark theme";
  }
  try { setTheme(localStorage.getItem("cfm-theme") || "dark"); } catch (e) { setTheme("dark"); }

  // ---- data ----
  var g;
  try {
    g = JSON.parse($("cfm-graph").textContent);
  } catch (e) {
    show("The embedded graph could not be read: " + e.message, true);
    return;
  }
  $("module").textContent = g.module;
  var s = g.stats;
  $("stats").textContent = s.endpoints + " endpoints · " + s.functions + " functions · " + s.edges +
    " edges · " + s.unresolved + " unresolved · " + g.diagnostics.length + " diagnostics";

  var byId = {}, out = {}, inc = {};
  g.nodes.forEach(function (n) { byId[n.id] = n; out[n.id] = []; inc[n.id] = []; });
  g.edges.forEach(function (e) { out[e.source].push(e); inc[e.target].push(e); });

  function label(n) { return n.kind === "endpoint" ? n.method + " " + n.path : n.name; }
  function where(loc) { return loc ? loc.file + ":" + loc.line : ""; }

  // ---- editor links (FR-17): only when the report was built with --editor ----
  var editor = root.dataset.editor;
  function editorURL(loc) {
    if (!g.editorRoot || !editor || !loc) return "";
    var abs = g.editorRoot + "/" + loc.file, line = loc.line, col = loc.col || 1;
    if (editor === "vscode") return "vscode://file" + encodeURI(abs) + ":" + line + ":" + col;
    if (editor === "zed") return "zed://file" + encodeURI(abs) + ":" + line + ":" + col;
    if (editor === "idea") return "idea://open?file=" + encodeURIComponent(abs) + "&line=" + line;
    return "";
  }
  function locNode(loc) {
    var span = el("span", "loc mono", where(loc));
    var url = editorURL(loc);
    if (!url) return span;
    var a = el("a", "loc mono", where(loc));
    a.href = url;
    a.title = "Open in " + editor;
    return a;
  }

  // ---- list + search ----
  var kind = "endpoint", rows = [], active = -1;
  var list = $("list"), search = $("search");
  var haystack = {};
  g.nodes.forEach(function (n) {
    haystack[n.id] = (label(n) + " " + (n.pkg || "") + " " + where(n.loc)).toLowerCase();
  });

  function renderList() {
    var q = search.value.trim().toLowerCase();
    var all = g.nodes.filter(function (n) { return n.kind === kind && haystack[n.id].indexOf(q) !== -1; });
    rows = all.slice(0, MAX_ROWS);
    list.replaceChildren();
    rows.forEach(function (n, i) {
      var li = el("li");
      li.id = "row-" + i;
      li.setAttribute("role", "option");
      li.tabIndex = -1;
      li.dataset.i = i;
      if (n.kind === "endpoint") {
        var head = el("span", "row-head mono");
        head.append(el("span", "verb", n.method), " ", el("span", "", n.path));
        li.append(head);
      } else {
        li.append(el("span", "row-head mono", n.name));
      }
      li.append(el("span", "row-sub mono", where(n.loc) || n.pkg));
      if (n.id === selected) li.setAttribute("aria-selected", "true");
      list.append(li);
    });
    var noun = kind === "endpoint" ? "endpoints" : "functions";
    $("count").textContent = all.length === 0 ? (q ? "No " + noun + " match “" + search.value.trim() + "”." : "No " + noun + " in this module.")
      : all.length > rows.length ? "Showing " + rows.length + " of " + all.length + " " + noun + ". Refine the search to see the rest."
      : all.length + " " + noun;
    active = -1;
    if (rows.length) setActive(0, false);
  }
  function setActive(i, focus) {
    if (i < 0 || i >= rows.length) return;
    if (active >= 0 && list.children[active]) list.children[active].tabIndex = -1;
    active = i;
    var li = list.children[i];
    li.tabIndex = 0;
    if (focus) { li.focus(); li.scrollIntoView({ block: "nearest" }); }
  }
  function setKind(k) {
    kind = k;
    ["endpoint", "function"].forEach(function (x) { $("tab-" + x).setAttribute("aria-selected", String(x === k)); });
    renderList();
  }
  $("tab-endpoint").addEventListener("click", function () { setKind("endpoint"); });
  $("tab-function").addEventListener("click", function () { setKind("function"); });
  search.addEventListener("input", renderList);
  search.addEventListener("keydown", function (e) {
    if (e.key === "ArrowDown" && rows.length) { e.preventDefault(); setActive(0, true); }
    if (e.key === "Enter" && rows.length) { e.preventDefault(); select(rows[0].id); }
    if (e.key === "Escape") { search.value = ""; renderList(); }
  });
  list.addEventListener("click", function (e) {
    var li = e.target.closest("li");
    if (li) { setActive(+li.dataset.i, true); select(rows[+li.dataset.i].id); }
  });
  list.addEventListener("keydown", function (e) {
    var moves = { ArrowDown: active + 1, ArrowUp: active - 1, Home: 0, End: rows.length - 1 };
    if (e.key in moves) { e.preventDefault(); if (e.key === "ArrowUp" && active === 0) search.focus(); else setActive(moves[e.key], true); }
    if (e.key === "Enter" || e.key === " ") { e.preventDefault(); select(rows[active].id); }
  });
  document.addEventListener("keydown", function (e) {
    var t = e.target.tagName;
    if (e.key === "/" && t !== "INPUT" && t !== "TEXTAREA") { e.preventDefault(); search.focus(); search.select(); }
  });

  // ---- detail panel ----
  var detail = $("detail"), selected = null;
  function dl(pairs) {
    var d = el("dl");
    pairs.forEach(function (p) {
      if (p[1] == null || p[1] === "") return;
      d.append(el("dt", "", p[0]));
      var dd = el("dd");
      dd.append(typeof p[1] === "string" || typeof p[1] === "number" ? el("span", p[2] ? "mono" : "", String(p[1])) : p[1]);
      d.append(dd);
    });
    return d;
  }
  function nodeButton(id) {
    var n = byId[id];
    var b = el("button", "link mono", n ? label(n) : id);
    b.type = "button";
    b.addEventListener("click", function () { select(id); });
    return b;
  }
  function edgeList(title, edges, other) {
    var sec = el("section");
    sec.append(el("h3", "", title + " (" + edges.length + ")"));
    if (!edges.length) { sec.append(el("p", "muted", "None.")); return sec; }
    var ul = el("ul", "edges");
    edges.forEach(function (e) {
      var li = el("li");
      var b = el("button", "link edge", "");
      b.type = "button";
      var n = byId[e[other]];
      b.append(el("span", "mono", n ? label(n) : e[other]), " ", el("span", "conf conf-" + e.confidence, e.kind + " · " + e.confidence));
      b.addEventListener("click", function () { showEdge(e); });
      li.append(b);
      ul.append(li);
    });
    sec.append(ul);
    return sec;
  }
  function diagList(id) {
    var ds = g.diagnostics.filter(function (d) { return d.nodeId === id; });
    if (!ds.length) return null;
    var sec = el("section");
    sec.append(el("h3", "", "Diagnostics (" + ds.length + ")"));
    ds.forEach(function (d) { sec.append(el("p", "diag", d.severity + " " + d.code + ": " + d.message)); });
    return sec;
  }
  // select comes from the list or the detail panel: it re-roots the tree. A tap inside the tree
  // only shows details (showNode), so the tree the user is reading stays put.
  function select(id) {
    if (!byId[id]) return;
    setRoot(id);
    showNode(id);
  }
  function showNode(id) {
    var n = byId[id];
    if (!n) return;
    selected = id;
    Array.prototype.forEach.call(list.children, function (li) {
      li.setAttribute("aria-selected", String(rows[+li.dataset.i].id === id));
    });
    detail.replaceChildren(
      el("h2", "mono", label(n)),
      dl([["Kind", n.kind], ["Status", n.status], ["Location", n.loc ? locNode(n.loc) : ""],
        ["Package", n.pkg, true], ["Framework", n.framework], ["Receiver", n.receiver, true],
        ["Signature", n.signature, true], ["Category", n.category], ["Reason", n.reason], ["ID", n.id, true]]),
      edgeList("Outgoing", out[id], "target"),
      edgeList("Incoming", inc[id], "source"));
    var dg = diagList(id);
    if (dg) detail.append(dg);
  }
  function showStep(st, fn) {
    var f = byId[fn], loc = f.loc ? { file: f.loc.file, line: st.line } : null;
    detail.replaceChildren(
      el("h2", "", st.k === "loop" ? "Loop" : st.k === "if" ? "If" : st.k === "select" ? "Select" : "Switch"),
      dl([[st.k === "if" ? "Condition" : st.k === "loop" ? "Header" : "Tag", st.cond || st.head || st.tag, true],
        ["Cases", st.cases ? st.cases.map(function (c) { return c.label; }).join(" · ") : "", true],
        ["Location", loc ? locNode(loc) : ""], ["In", nodeButton(fn)]]));
  }
  function showEdge(e) {
    var back = el("button", "link", "Back to " + (byId[selected] ? label(byId[selected]) : "node"));
    back.type = "button";
    var from = selected;
    back.addEventListener("click", function () { showNode(from); });
    detail.replaceChildren(
      el("h2", "", "Edge: " + e.kind),
      dl([["Source", nodeButton(e.source)], ["Target", nodeButton(e.target)], ["Confidence", e.confidence],
        ["Method", e.method], ["Reason", e.reason], ["Site", locNode(e.site)], ["Site ID", e.siteId, true],
        ["Order", e.order], ["Alternatives", e.alternatives]]),
      back);
    back.focus();
  }

  // ---- blockers (FR-19) ----
  if (s.blockers && s.blockers.length) {
    var ban = $("blockers");
    ban.append(el("strong", "", "Results are incomplete until generated code exists."));
    s.blockers.forEach(function (i) {
      var d = g.diagnostics[i], p = el("p");
      p.append(d.message + ". Run ", el("code", "", (d.details || {}).fix || "the generator"), " in the module, then run cfm again.");
      ban.append(p);
    });
    ban.hidden = false;
  }

  // ---- route map (overview, shown until something is selected) ----
  function nodeEl(id, l, extra) {
    var d = { id: id, ref: extra.ref, label: l, kind: extra.kind, w: l.length * 7.3 + 16, h: 22 }; // 12px monospace ≈ 7.3px per char
    return { data: d, classes: extra.cls || "" };
  }
  var routeEls = [], used = {};
  g.edges.forEach(function (e) {
    if (e.kind !== "routes_to") return;
    [e.source, e.target].forEach(function (id) {
      if (used[id]) return;
      used[id] = true;
      routeEls.push(nodeEl(id, label(byId[id]), { ref: id, kind: byId[id].kind }));
    });
    routeEls.push({ data: { source: e.source, target: e.target, conf: e.confidence, label: "" } });
  });

  // ---- call tree ----
  var MAX_TREE = 400; // ponytail: hard cap keeps dagre fast; lift it if real trees need more
  var DEPTH = 3;
  var CONF = { high: 0, medium: 1, low: 2, unknown: 3 };
  var tree = { root: null, mode: "callees", open: {} };
  var filters = { external: true, low: true, unresolved: true };
  var pkgOn = {}; // user choices survive re-rooting
  function isHelper(pkg) { return /^(shared|error.*|errors|utils|response|logger)$/.test((pkg || "").split("/").pop()); }
  function pkgShown(pkg) { return !pkg || (pkg in pkgOn ? pkgOn[pkg] : !isHelper(pkg)); }

  // Edges from id toward the next level, grouped by the node on the other end (×N per parent).
  function groups(id) {
    var callers = tree.mode === "callers", seen = {}, gs = [];
    (callers ? inc[id] : out[id]).forEach(function (e) {
      var other = callers ? e.source : e.target;
      if (!seen[other]) { seen[other] = { id: other, edges: [] }; gs.push(seen[other]); }
      seen[other].edges.push(e);
    });
    gs.forEach(function (gr) { gr.edges.sort(function (a, b) { return CONF[a.confidence] - CONF[b.confidence]; }); });
    return gs;
  }
  function kept(gr) {
    var n = byId[gr.id], c = gr.edges[0].confidence;
    if (!n) return false;
    if (!filters.external && n.kind === "external") return false;
    if (!filters.unresolved && n.kind === "unresolved") return false;
    return filters.low || (c !== "low" && c !== "unknown");
  }

  function buildTree() {
    var els = [], pkgs = {}, count = 0, cut = false, callers = tree.mode === "callers";
    function link(parent, child, edges) {
      var e = edges[0], l = e.kind === "uses_middleware" ? "middleware · " + e.confidence : e.confidence;
      els.push({ data: callers ? { source: child, target: parent } : { source: parent, target: child } });
      var d = els[els.length - 1].data;
      d.conf = e.confidence; d.label = l; d.edge = e;
    }
    function walk(inst, id, depth, path) {
      var n = byId[id], gs = groups(id).filter(kept), l = label(n);
      gs.forEach(function (gr) { if (byId[gr.id].pkg) pkgs[byId[gr.id].pkg] = true; });
      // Callers mode walks all the way up to endpoints (they have no callers); callees stop at DEPTH.
      var showKids = gs.length && (callers || depth < DEPTH || tree.open[inst]);
      if (gs.length && !showKids) l += "  +" + gs.length;
      els.push(nodeEl(inst, l, { ref: id, kind: n.kind, cls: gs.length && !showKids ? "more" : "" }));
      count++;
      if (!showKids) return;
      var hidden = 0;
      gs.forEach(function (gr, i) {
        var c = byId[gr.id], child = inst + "/" + i;
        if (!pkgShown(c.pkg)) { hidden += gr.edges.length; return; }
        if (count >= MAX_TREE) { cut = true; return; }
        var times = gr.edges.length > 1 ? "  ×" + gr.edges.length : "";
        if (path[gr.id]) {
          els.push(nodeEl(child, "↺ " + label(c) + times, { ref: gr.id, kind: c.kind, cls: "backref" }));
          count++;
        } else {
          var at = els.length; // walk pushes the child's own node first
          path[gr.id] = true;
          walk(child, gr.id, depth + 1, path);
          delete path[gr.id];
          els[at].data.label += times;
          els[at].data.w += times.length * 7.3;
        }
        link(inst, child, gr.edges);
      });
      if (hidden) {
        var hid = inst + "/helpers", hl = "… " + hidden + " helper call" + (hidden > 1 ? "s" : "");
        els.push(nodeEl(hid, hl, { kind: "helper", cls: "helper" }));
        els.push({ data: callers ? { source: hid, target: inst, label: "" } : { source: inst, target: hid, label: "" }, classes: "helper" });
      }
    }
    var path = {};
    path[tree.root] = true;
    walk("r", tree.root, 0, path);
    renderPkgs(pkgs);
    var kids = els.filter(function (x) { return x.data.source; }).length;
    $("tree-note").textContent = cut ? "Showing the first " + MAX_TREE + " nodes. Hide packages or turn off filters to see the rest."
      : !kids ? (callers ? "Nothing calls this." : "No outgoing calls (or all of them are filtered out).") : "";
    return els;
  }
  function renderPkgs(pkgs) {
    var names = Object.keys(pkgs).sort(), box = $("pkg-list"), off = 0;
    box.replaceChildren();
    names.forEach(function (p) {
      var lab = el("label", "check"), cb = el("input");
      cb.type = "checkbox";
      cb.checked = pkgShown(p);
      if (!cb.checked) off++;
      cb.addEventListener("change", function () { pkgOn[p] = cb.checked; draw(); });
      lab.append(cb, p);
      box.append(lab);
    });
    if (!names.length) box.append(el("p", "muted", "No packages in this tree."));
    $("pkgs-sum").textContent = "Packages (" + names.length + (off ? ", " + off + " hidden" : "") + ")";
  }

  // ---- flowchart (ARCHITECTURE §5) ----
  var view = "tree", flowRoot = null, flowOpen = {};
  var siteEdges = {};
  g.edges.forEach(function (e) {
    if (e.siteId) (siteEdges[e.source + "|" + e.siteId] = siteEdges[e.source + "|" + e.siteId] || []).push(e);
  });
  function short(t) { return t.length > 40 ? t.slice(0, 39) + "…" : t; }
  function hasFlow(id) { return !!(byId[id] && byId[id].flow && byId[id].flow.length); }
  // An endpoint has no body of its own; its flowchart is its handler's.
  function flowFn(id) {
    var n = byId[id];
    if (n && n.kind === "endpoint") {
      var r = out[id].filter(function (e) { return e.kind === "routes_to"; })[0];
      return r ? r.target : null;
    }
    return id;
  }

  function buildFlow(fnId) {
    var els = [], seq = 0, cut = false;
    function node(l, kind, parent, extra) {
      var id = "f" + seq++, d = extra || {};
      d.id = id; d.kind = kind; d.parent = parent;
      if (kind === "diamond") {
        // Text only fits the inner half of a diamond, so labels stop at two lines; the full text is in the detail panel.
        l = l.length > 36 ? l.slice(0, 35) + "…" : l;
        var lines = l.length > 18 ? 2 : 1;
        d.w = Math.min(l.length, 18) * 6.7 * 1.9 + 24;
        d.h = lines * 15 * 1.9 + 14;
      } else {
        d.w = l.length * 7.3 + 16;
        d.h = 22;
      }
      d.label = l;
      els.push({ data: d });
      if (seq >= MAX_TREE) cut = true;
      return id;
    }
    function link(entries, to, cls) {
      entries.forEach(function (en) {
        els.push({ data: { source: en.from, target: to, label: short(en.label || "") }, classes: cls || "" });
      });
    }
    // steps returns the open exits ({from, label}) that the next step connects to.
    function steps(list, entries, ctx) {
      list.forEach(function (st) { if (!cut) entries = step(st, entries, ctx); });
      return entries;
    }
    function step(st, entries, ctx) {
      var id, exits;
      switch (st.k) {
      case "call": case "go": case "defer":
        var es = siteEdges[ctx.fn + "|" + st.site] || [], names = [];
        es.forEach(function (e) { var l = label(byId[e.target]); if (names.indexOf(l) === -1) names.push(l); });
        var callee = es.length === 1 && byId[es[0].target].kind === "function" ? es[0].target : null;
        var key = ctx.key + "/" + st.site, canOpen = st.k === "call" && hasFlow(callee) && !ctx.path[callee];
        var l = (st.k === "call" ? "" : st.k + " ") + (names.slice(0, 2).join(" | ") || st.site) + (names.length > 2 ? " | …" : "") + "()";
        id = node(l + (canOpen && !flowOpen[key] ? "  +" : ""), "step", ctx.parent, { edge: es[0], key: canOpen ? key : "" });
        link(entries, id);
        if (!canOpen || !flowOpen[key]) return [{ from: id }];
        var cluster = node(label(byId[callee]), "cluster", ctx.parent, { ref: callee });
        var path = Object.create(ctx.path);
        path[callee] = true;
        return steps(byId[callee].flow, [{ from: id }], { fn: callee, parent: cluster, path: path, key: key, inner: true });
      case "return":
        id = node("return", "term", ctx.parent);
        link(entries, id);
        return ctx.inner ? [{ from: id }] : []; // inside an expanded callee, return goes back to the caller
      case "panic":
        link(entries, node("panic", "term", ctx.parent));
        return [];
      case "if":
        id = node(st.cond, "diamond", ctx.parent, { step: st, fn: ctx.fn });
        link(entries, id);
        exits = steps(st.then || [], [{ from: id, label: "true" }], ctx);
        return exits.concat(st.else ? steps(st.else, [{ from: id, label: "false" }], ctx) : [{ from: id, label: "false" }]);
      case "switch": case "select":
        id = node(st.k === "select" ? "select" : st.tag || "switch", "diamond", ctx.parent, { step: st, fn: ctx.fn });
        link(entries, id);
        exits = [];
        var dflt = false;
        (st.cases || []).forEach(function (c) {
          if (c.label === "default") dflt = true;
          exits = exits.concat(steps(c.body || [], [{ from: id, label: c.label === "default" ? "default" : "case " + c.label }], ctx));
        });
        // Cases without calls are pruned, so with no default there may be paths the outline does not show.
        return dflt ? exits : exits.concat([{ from: id, label: "other" }]);
      case "loop":
        id = node(st.head || "for", "diamond", ctx.parent, { step: st, fn: ctx.fn });
        link(entries, id);
        steps(st.body || [], [{ from: id, label: "loop" }], ctx).forEach(function (en) {
          els.push({ data: { source: en.from, target: id, label: "↺" }, classes: "back" });
        });
        return [{ from: id, label: "exit" }];
      }
      return entries;
    }
    var start = node(label(byId[fnId]), "term", undefined, { ref: fnId });
    var ends = steps(byId[fnId].flow, [{ from: start }], { fn: fnId, path: (function () { var p = {}; p[fnId] = true; return p; })(), key: "r", inner: false });
    if (ends.length) link(ends, node("end", "term"));
    $("tree-note").textContent = cut ? "Showing the first " + MAX_TREE + " steps. Collapse expanded calls to see the rest." : "";
    return els;
  }
  function flowEls() {
    var fn = flowFn(flowRoot), n = byId[fn];
    var why = !fn ? "This endpoint has no resolved handler, so there is no body to outline."
      : n.kind !== "function" ? "External and unresolved targets have no body in this module to outline."
      : !hasFlow(fn) ? "Nothing to outline: the body has no calls, returns or panics." : "";
    if (why) { $("tree-note").textContent = why; return []; }
    return buildFlow(fn);
  }

  function draw() {
    if (!cy) return;
    var els = !tree.root ? routeEls : view === "flow" ? flowEls() : buildTree();
    $("toolbar").hidden = !tree.root;
    $("tree-controls").hidden = view !== "tree";
    if (tree.root) status.hidden = true;
    $("graph").setAttribute("aria-label", !tree.root ? "Route map: endpoints and their handlers"
      : view === "flow" ? "Flowchart of " + label(byId[flowRoot])
      : (tree.mode === "callers" ? "Callers of " : "Calls from ") + label(byId[tree.root]));
    cy.batch(function () { cy.elements().remove(); cy.add(els); });
    var flow = tree.root && view === "flow";
    cy.layout({ name: "dagre", rankDir: flow ? "TB" : "LR", nodeSep: flow ? 24 : 8, rankSep: flow ? 36 : tree.root ? 60 : 80 }).run();
    if (tree.root && !flow) {
      var keep = cy.nodes().filter(function (x) { return x.data("ref") === selected; })[0]; // the selection survives a tab switch
      (keep || cy.getElementById("r")).select();
    }
  }
  function setRoot(id) {
    if (id !== tree.root) tree.open = {};
    if (id !== flowRoot) flowOpen = {};
    tree.root = flowRoot = id;
    draw();
  }
  function setView(v) {
    view = v;
    if (v === "flow" && selected !== flowRoot) { flowRoot = selected || tree.root; flowOpen = {}; }
    ["tree", "flow"].forEach(function (x) { $("view-" + x).setAttribute("aria-selected", String(x === v)); });
    draw();
  }
  $("view-tree").addEventListener("click", function () { setView("tree"); });
  $("view-flow").addEventListener("click", function () { setView("flow"); });
  function setMode(m) {
    tree.mode = m;
    tree.open = {};
    ["callees", "callers"].forEach(function (x) { $("mode-" + x).setAttribute("aria-pressed", String(x === m)); });
    draw();
  }
  $("mode-callees").addEventListener("click", function () { setMode("callees"); });
  $("mode-callers").addEventListener("click", function () { setMode("callers"); });
  $("overview").addEventListener("click", function () { tree.root = null; draw(); });
  ["external", "low", "unresolved"].forEach(function (f) {
    $("f-" + f).addEventListener("change", function (e) { filters[f] = e.target.checked; draw(); });
  });

  var cy = null;
  function css(name) { return getComputedStyle(root).getPropertyValue(name).trim(); }
  function style() {
    return [
      { selector: "node", style: {
        label: "data(label)", shape: "round-rectangle", width: "data(w)", height: "data(h)",
        "font-family": "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace", "font-size": 12,
        "text-valign": "center", color: css("--text"), "background-color": css("--panel"),
        "border-width": 1, "border-color": css("--line") } },
      { selector: "node[kind = 'endpoint']", style: { "border-color": css("--muted") } },
      { selector: "node[kind = 'external'], node[kind = 'unresolved'], node.helper", style: { color: css("--muted") } },
      { selector: "node.more", style: { "border-color": css("--muted"), "border-style": "double", "border-width": 3 } },
      { selector: "node.backref", style: { "border-style": "dashed" } },
      { selector: "node.helper", style: { "border-width": 0, "background-opacity": 0 } },
      { selector: "node[kind = 'diamond']", style: { shape: "diamond", "text-wrap": "wrap", "text-max-width": 130, "font-size": 11 } },
      { selector: "node[kind = 'term']", style: { shape: "round-rectangle", "corner-radius": 11 } },
      { selector: "node[kind = 'cluster']", style: {
        "background-color": css("--bg"), "border-style": "dashed", "text-valign": "top", "text-halign": "center",
        "text-margin-y": -4, color: css("--muted"), padding: 12 } },
      { selector: "node:selected", style: { "border-color": css("--accent"), "border-width": 2 } },
      { selector: "edge", style: {
        width: 1, "curve-style": "bezier", "target-arrow-shape": "triangle",
        "line-color": css("--muted"), "target-arrow-color": css("--muted"),
        label: "data(label)", "font-size": 10, color: css("--muted"), 
        "text-background-color": css("--bg"), "text-background-opacity": 1, "text-background-padding": 2 } },
      { selector: "edge[conf = 'high']", style: { label: "" } }, // solid line already says high; label only the doubtful ones
      { selector: "edge[conf = 'medium']", style: { "line-style": "dashed" } },
      { selector: "edge[conf = 'low'], edge[conf = 'unknown']", style: { "line-style": "dotted" } },
      { selector: "edge.helper", style: { "line-style": "dotted", "target-arrow-shape": "none" } }
    ];
  }

  renderList();
  if (typeof cytoscape !== "function") {
    show("The graph library failed to load. Lists and details still work.", true);
  } else {
    status.hidden = true;
    cy = cytoscape({ container: $("graph"), style: style(), maxZoom: 1.25 }); // a 3-node graph would otherwise fit to giant labels
    cy.on("tap", "node", function (ev) {
      var d = ev.target.data();
      if (!tree.root) { select(d.ref); return; }
      if (view === "flow") {
        if (d.edge) showEdge(d.edge); else if (d.step) showStep(d.step, d.fn); else if (d.ref) showNode(d.ref);
        if (d.key) { flowOpen[d.key] = !flowOpen[d.key]; draw(); }
        return;
      }
      if (!d.ref) return; // the "… N helper calls" leaf: the Packages menu controls it
      showNode(d.ref);
      if (ev.target.hasClass("more") || tree.open[d.id]) {
        tree.open[d.id] = !tree.open[d.id];
        draw();
        cy.$(":selected").unselect();
        cy.getElementById(d.id).select();
      }
    });
    cy.on("tap", "edge", function (ev) { if (ev.target.data("edge")) showEdge(ev.target.data("edge")); });
    draw();
    if (!routeEls.length) show("No endpoints were found. Pick a function on the left to see its call tree, and check the diagnostics in the JSON for unsupported routers or load errors.", false);
  }
  if (!g.nodes.some(function (n) { return n.kind === "endpoint"; }) && g.nodes.length) setKind("function");

  $("theme").addEventListener("click", function () {
    var next = root.dataset.theme === "dark" ? "light" : "dark";
    setTheme(next);
    try { localStorage.setItem("cfm-theme", next); } catch (e) { /* private mode: not remembered */ }
    if (cy) cy.style(style());
  });
})();
