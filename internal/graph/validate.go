package graph

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
)

// allowed (source kind, target kind) pairs per edge kind — GRAPH_SCHEMA §7.2.
var edgeShapes = map[EdgeKind]struct{ from, to []NodeKind }{
	EdgeRoutesTo:       {[]NodeKind{KindEndpoint}, []NodeKind{KindFunction, KindExternal, KindUnresolved}},
	EdgeUsesMiddleware: {[]NodeKind{KindEndpoint}, []NodeKind{KindFunction, KindExternal, KindUnresolved}},
	EdgeCalls:          {[]NodeKind{KindFunction}, []NodeKind{KindFunction, KindExternal, KindUnresolved}},
}

var (
	validKinds   = []NodeKind{KindEndpoint, KindFunction, KindExternal, KindUnresolved}
	validStatus  = []Status{StatusResolved, StatusPartiallyResolved, StatusUnresolved}
	validConf    = []Confidence{ConfHigh, ConfMedium, ConfLow, ConfUnknown}
	validMethods = []Method{MethodStatic, MethodVTA, MethodFrameworkRegistration, MethodWrapperHeuristic, MethodUnresolved}
)

// Validate checks GRAPH_SCHEMA §7. A failure is a CFM bug, never a property of the analyzed code.
func Validate(g *Graph) error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	nodes := make(map[NodeID]*Node, len(g.Nodes))
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if _, dup := nodes[n.ID]; dup {
			bad("duplicate node id %q", n.ID)
		}
		nodes[n.ID] = n
		if !slices.Contains(validKinds, n.Kind) || !slices.Contains(validStatus, n.Status) {
			bad("node %q: invalid kind %q or status %q", n.ID, n.Kind, n.Status)
		}
		if i > 0 && g.Nodes[i-1].ID >= n.ID {
			bad("nodes not sorted at %q", n.ID)
		}
		if n.Loc != nil {
			checkPath(g, n.Loc.File, bad)
		}
	}

	sitesWithEdges := map[NodeID]map[string]bool{}
	for i := range g.Edges {
		e := &g.Edges[i]
		if i > 0 && !edgeLess(&g.Edges[i-1], e) {
			bad("edges not sorted or duplicate at %s→%s", e.Source, e.Target)
		}
		checkPath(g, e.Site.File, bad)
		if !slices.Contains(validConf, e.Confidence) || !slices.Contains(validMethods, e.Method) {
			bad("edge %s→%s: invalid confidence %q or method %q", e.Source, e.Target, e.Confidence, e.Method)
		}
		src, tgt := nodes[e.Source], nodes[e.Target]
		if src == nil || tgt == nil {
			bad("edge %s→%s: dangling endpoint", e.Source, e.Target)
			continue
		}
		shape, ok := edgeShapes[e.Kind]
		if !ok || !slices.Contains(shape.from, src.Kind) || !slices.Contains(shape.to, tgt.Kind) {
			bad("edge %s→%s: kind %q not allowed from %s to %s", e.Source, e.Target, e.Kind, src.Kind, tgt.Kind)
		}
		switch e.Kind {
		case EdgeCalls:
			if e.SiteID == "" {
				bad("calls edge %s→%s: missing siteId", e.Source, e.Target)
			}
			if sitesWithEdges[e.Source] == nil {
				sitesWithEdges[e.Source] = map[string]bool{}
			}
			sitesWithEdges[e.Source][e.SiteID] = true
		case EdgeUsesMiddleware:
			if e.Order == nil {
				bad("middleware edge %s→%s: missing order", e.Source, e.Target)
			}
		}
	}

	// §7.3: every calls edge has a flow step, and every call/go/defer step has an edge.
	for _, n := range g.Nodes {
		if n.Kind != KindFunction {
			continue
		}
		steps := map[string]bool{}
		WalkSites(n.Flow, func(site string) { steps[site] = true })
		for site := range steps {
			if !sitesWithEdges[n.ID][site] {
				bad("function %q: flow site %s has no calls edge", n.ID, site)
			}
		}
		for site := range sitesWithEdges[n.ID] {
			if !steps[site] {
				bad("function %q: calls edge site %s has no flow step", n.ID, site)
			}
		}
	}

	for i := range g.Diagnostics {
		if l := g.Diagnostics[i].Loc; l != nil {
			checkPath(g, l.File, bad)
		}
	}
	for _, b := range g.Stats.Blockers {
		if b < 0 || b >= len(g.Diagnostics) {
			bad("stats.blockers: index %d out of range", b)
		}
	}
	return errors.Join(errs...)
}

// §7.5: no absolute paths unless the user opted in with --editor.
func checkPath(g *Graph, file string, bad func(string, ...any)) {
	if g.EditorRoot == nil && (filepath.IsAbs(file) || !filepath.IsLocal(filepath.FromSlash(file))) {
		bad("non-relative path %q", file)
	}
}

// WalkSites visits the site of every call/go/defer step in a flow tree.
func WalkSites(steps []Step, visit func(string)) {
	for _, s := range steps {
		if s.K == StepCall || s.K == StepGo || s.K == StepDefer {
			visit(s.Site)
		}
		WalkSites(s.Then, visit)
		WalkSites(s.Else, visit)
		WalkSites(s.Body, visit)
		for _, c := range s.Cases {
			WalkSites(c.Body, visit)
		}
	}
}
