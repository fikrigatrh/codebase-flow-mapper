// Package report writes the single-file HTML report: viewer CSS/JS, vendored libraries and the
// graph JSON, all inline, so it opens from file:// with no network.
package report

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"

	"github.com/fikrigatrh/codebase-flow-mapper/web"
)

// The CSP allows only these exact inline scripts (by hash) and no network at all, so even a
// string that escaped the JSON encoding could neither run nor phone home.
var page = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en" data-theme="dark" data-editor="{{.Editor}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; script-src {{.ScriptSrc}}">
<title>cfm · {{.Module}}</title>
<style>{{.CSS}}</style>
</head>
<body>
<header>
<span class="brand mono">cfm</span>
<span class="module mono" id="module"></span>
<span class="stats" id="stats"></span>
<button type="button" id="theme">Light theme</button>
</header>
<div id="blockers" class="banner" role="alert" hidden></div>
<div class="layout">
<nav class="side" aria-label="Endpoints and functions">
<div class="tabs" role="tablist">
<button type="button" role="tab" id="tab-endpoint" aria-selected="true" aria-controls="list">Endpoints</button>
<button type="button" role="tab" id="tab-function" aria-selected="false" aria-controls="list">Functions</button>
</div>
<input id="search" type="search" autocomplete="off" spellcheck="false" placeholder="Search path, name or file (press /)" aria-label="Search by path, name or file">
<p id="count" class="muted"></p>
<ul id="list" role="listbox" aria-label="Results"></ul>
</nav>
<main>
<div class="toolbar" id="toolbar" hidden>
<button type="button" id="overview">All routes</button>
<div class="seg" role="tablist" aria-label="View">
<button type="button" role="tab" id="view-tree" aria-selected="true" aria-controls="graph">Call Tree</button>
<button type="button" role="tab" id="view-flow" aria-selected="false" aria-controls="graph">Flowchart</button>
</div>
<div id="tree-controls" class="controls">
<div class="seg" role="group" aria-label="Direction">
<button type="button" id="mode-callees" aria-pressed="true">Callees</button>
<button type="button" id="mode-callers" aria-pressed="false">Callers</button>
</div>
<label class="check"><input type="checkbox" id="f-external" checked> External</label>
<label class="check"><input type="checkbox" id="f-low" checked> Low confidence</label>
<label class="check"><input type="checkbox" id="f-unresolved" checked> Unresolved</label>
<details id="pkgs"><summary id="pkgs-sum">Packages</summary><div id="pkg-list"></div></details>
<span class="legend muted">Lines: solid high · dashed medium · dotted low or unknown</span>
</div>
<p id="tree-note" class="muted"></p>
</div>
<div class="canvas">
<div id="graph" role="img" aria-label="Call graph"></div>
<p id="status">Loading graph…</p>
</div>
</main>
<aside id="detail" aria-label="Details" aria-live="polite"><p class="muted">Select an endpoint or function to see its details.</p></aside>
</div>
<script id="cfm-graph" type="application/json">{{.Graph}}</script>
{{range .Scripts}}<script>{{.}}</script>
{{end}}</body>
</html>
`))

// unsafeInline would end or confuse a <script>/<style> element if it appeared inside one.
var unsafeInline = regexp.MustCompile(`(?i)</script|</style|<!--`)

// Write takes graphJSON from graph.Encode (whose HTML escaping turns every < > & into <),
// module for the title, and editor from the CLI allowlist ("" when unset).
func Write(w io.Writer, graphJSON []byte, module, editor string) error {
	inline := map[string]string{"graph": string(graphJSON), "viewer.css": web.CSS, "cytoscape": web.Cytoscape,
		"cytoscape-dagre": web.CytoscapeDagre, "viewer.js": web.JS}
	for name, s := range inline {
		if unsafeInline.MatchString(s) {
			return fmt.Errorf("report: %s contains a sequence that breaks inline embedding", name)
		}
	}
	scripts := []template.JS{template.JS(web.Cytoscape), template.JS(web.CytoscapeDagre), template.JS(web.JS)}
	var hashes []string
	for _, s := range scripts {
		sum := sha256.Sum256([]byte(s))
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	return page.Execute(w, map[string]any{
		"Editor":    editor,
		"Module":    module,
		"ScriptSrc": strings.Join(hashes, " "),
		"CSS":       template.CSS(web.CSS),
		"Graph":     template.JS(graphJSON),
		"Scripts":   scripts,
	})
}
