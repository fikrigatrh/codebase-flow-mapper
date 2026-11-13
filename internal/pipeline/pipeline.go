// Package pipeline runs load → callgraph → routes → flow and merges the result into one graph.
package pipeline

import (
	"context"
	"slices"
	"time"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/callgraph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/flow"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/routes"
)

type Options struct {
	Dir        string
	Exclude    []string
	Version    string // analyzerVersion
	EditorRoot bool   // embed the absolute module root (--editor)
}

var blockerCodes = []string{"WIRE_GEN_MISSING", "WIRE_GEN_STALE", "GENERATED_PKG_MISSING"}

// Run returns an error only for unusable input (see load.Load). The graph is not yet validated:
// graph.Encode does that before anything is written.
func Run(ctx context.Context, opts Options) (*graph.Graph, error) {
	start := time.Now()
	p, loadDiags, err := load.Load(ctx, load.Options{Dir: opts.Dir, Exclude: opts.Exclude})
	if err != nil {
		return nil, err
	}
	cg := callgraph.Build(p)
	rt := routes.Build(p, cg)
	fl := flow.Build(p, cg)

	g := &graph.Graph{
		SchemaVersion:   graph.SchemaVersion,
		AnalyzerVersion: opts.Version,
		Module:          p.Module,
		GoVersion:       p.GoVersion,
		GeneratedAt:     start.UTC().Format(time.RFC3339),
		Edges:           slices.Concat(cg.Edges, rt.Edges),
		Diagnostics:     slices.Concat(loadDiags, cg.Diags, rt.Diags, fl.Diags),
	}
	if opts.EditorRoot {
		g.EditorRoot = &p.Root
	}

	// Both stages emit external/unresolved nodes for the same callee; IDs are canonical, so first wins.
	seen := map[graph.NodeID]bool{}
	for _, n := range slices.Concat(cg.Nodes, rt.Nodes) {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		n.Flow, n.Body = fl.Steps[n.ID], fl.Bodies[n.ID]
		g.Nodes = append(g.Nodes, n)
		switch n.Kind {
		case graph.KindFunction:
			g.Stats.Functions++
		case graph.KindEndpoint:
			g.Stats.Endpoints++
		case graph.KindUnresolved:
			g.Stats.Unresolved++
		}
	}
	for i, d := range g.Diagnostics {
		if slices.Contains(blockerCodes, d.Code) {
			g.Stats.Blockers = append(g.Stats.Blockers, i)
		}
	}
	g.Stats.Packages = len(p.Pkgs)
	g.Stats.Edges = len(g.Edges)
	g.Stats.ElapsedMs = time.Since(start).Milliseconds()
	return g, nil
}

// HasErrors reports whether the analysis itself was degraded (exit code 1): an error diagnostic,
// or target code that did not load or type-check.
func HasErrors(g *graph.Graph) bool {
	return slices.ContainsFunc(g.Diagnostics, func(d graph.Diagnostic) bool {
		return d.Severity == graph.SevError || d.Code == "GO_LOAD_ERROR" || d.Code == "GO_TYPE_ERROR"
	})
}
