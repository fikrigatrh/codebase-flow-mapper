package graph

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

// Normalize sorts nodes and edges and replaces nil slices with empty ones, so output is byte-stable.
func Normalize(g *Graph) {
	slices.SortFunc(g.Nodes, func(a, b Node) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(g.Edges, func(a, b Edge) int { return edgeCmp(&a, &b) })
	if g.Nodes == nil {
		g.Nodes = []Node{}
	}
	if g.Edges == nil {
		g.Edges = []Edge{}
	}
	if g.Diagnostics == nil {
		g.Diagnostics = []Diagnostic{}
	}
	if g.Stats.Blockers == nil {
		g.Stats.Blockers = []int{}
	}
}

// Encode normalizes, validates and writes indented JSON. json's default HTML escaping (<, >, &)
// is kept on purpose: the same bytes are embedded in a <script> tag in the report.
func Encode(w io.Writer, g *Graph) error {
	Normalize(g)
	if err := Validate(g); err != nil {
		return fmt.Errorf("graph validation failed (CFM bug): %w", err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(g)
}

// edge identity order: (source, target, kind, siteId, order) — GRAPH_SCHEMA §3.
func edgeCmp(a, b *Edge) int {
	return cmp.Or(
		cmp.Compare(a.Source, b.Source),
		cmp.Compare(a.Target, b.Target),
		cmp.Compare(a.Kind, b.Kind),
		cmp.Compare(a.SiteID, b.SiteID),
		cmp.Compare(orderOf(a), orderOf(b)),
	)
}

func edgeLess(a, b *Edge) bool { return edgeCmp(a, b) < 0 }

func orderOf(e *Edge) int {
	if e.Order == nil {
		return -1
	}
	return *e.Order
}
