// Package web holds the viewer assets that internal/report inlines into the HTML report.
package web

import _ "embed"

var (
	//go:embed viewer.css
	CSS string
	//go:embed viewer.js
	JS string
	//go:embed vendor/cytoscape.min.js
	Cytoscape string
	//go:embed vendor/cytoscape-dagre.min.js
	CytoscapeDagre string
)
