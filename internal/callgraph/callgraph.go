// Package callgraph turns the VTA call graph of a loaded program into function, external and
// unresolved nodes plus calls edges (ARCHITECTURE §4.1).
package callgraph

import (
	"cmp"
	"fmt"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
)

type Result struct {
	Nodes []graph.Node
	Edges []graph.Edge
	Diags []graph.Diagnostic
	Funcs map[graph.NodeID]*ssa.Function // project function nodes → their SSA function (for the flow stage)
}

type siteRef struct {
	owner graph.NodeID
	key   string
}

// site aggregates every SSA call instruction at one source position of one project function:
// generic instances and their origin share positions, so their targets are unioned.
type site struct {
	loc      graph.Loc
	common   *ssa.CallCommon
	static   bool
	excluded bool                       // a target lies in an excluded in-module package
	targets  map[graph.NodeID]bool      // project functions
	externs  map[graph.NodeID]graph.Sym // external functions: nodes are created only if an edge uses them
}

type builder struct {
	p        *load.Program
	cg       *callgraph.Graph
	included map[string]bool // project package paths after exclusion
	nodes    map[graph.NodeID]*graph.Node
	funcs    map[graph.NodeID]*ssa.Function
	sites    map[siteRef]*site
}

func Build(p *load.Program) *Result {
	fns := ssautil.AllFunctions(p.SSA)
	b := &builder{
		p:        p,
		cg:       vta.CallGraph(fns, cha.CallGraph(p.SSA)),
		included: map[string]bool{},
		nodes:    map[graph.NodeID]*graph.Node{},
		funcs:    map[graph.NodeID]*ssa.Function{},
		sites:    map[siteRef]*site{},
	}
	for _, pkg := range p.Pkgs {
		b.included[pkg.PkgPath] = true
	}
	for fn := range fns {
		if o := owner(fn); o.Synthetic == "" && b.scope(o) == inProject {
			b.collect(fn, b.function(o))
		}
	}
	return b.emit()
}

type scope int

const (
	external scope = iota
	inProject
	excludedPkg
)

func (b *builder) scope(fn *ssa.Function) scope {
	path := pkgPath(fn)
	switch {
	case b.included[path]:
		return inProject
	case path == b.p.Module || strings.HasPrefix(path, b.p.Module+"/"):
		return excludedPkg
	}
	return external
}

// canon maps a generic instance to its origin: one node per source function.
func canon(fn *ssa.Function) *ssa.Function {
	if o := fn.Origin(); o != nil {
		return o
	}
	return fn
}

// owner is the function whose node a call site belongs to: range-over-func loop bodies are
// synthetic closures with no source literal, so their calls belong to the enclosing function.
func owner(fn *ssa.Function) *ssa.Function {
	fn = canon(fn)
	for fn.Synthetic == "range-over-func yield" && fn.Parent() != nil {
		fn = canon(fn.Parent())
	}
	return fn
}

func outermost(fn *ssa.Function) *ssa.Function {
	fn = canon(fn)
	for fn.Parent() != nil {
		fn = canon(fn.Parent())
	}
	return fn
}

func pkgPath(fn *ssa.Function) string {
	top := outermost(fn)
	switch {
	case top.Pkg != nil:
		return top.Pkg.Pkg.Path()
	case top.Object() != nil && top.Object().Pkg() != nil:
		return top.Object().Pkg().Path()
	}
	return "builtin"
}

// sym names fn; func literals are their outermost function's name plus SSA's "$n" suffix.
func sym(fn *ssa.Function) graph.Sym {
	fn = canon(fn)
	top := outermost(fn)
	s := graph.Sym{Pkg: pkgPath(top), Name: top.Name() + strings.TrimPrefix(fn.Name(), top.Name())}
	if recv := top.Signature.Recv(); recv != nil {
		s.Recv = types.TypeString(recv.Type(), func(*types.Package) string { return "" })
	}
	return s
}

func displayName(s graph.Sym) string {
	if s.Recv == "" {
		return s.Name
	}
	return "(" + s.Recv + ")." + s.Name
}

// byName qualifies types by package name, as in source ("context.Context").
func byName(own *types.Package) types.Qualifier {
	return func(p *types.Package) string {
		if p == own {
			return ""
		}
		return p.Name()
	}
}

func (b *builder) function(fn *ssa.Function) graph.NodeID {
	s := sym(fn)
	id := graph.FuncID(s)
	if _, ok := b.nodes[id]; ok {
		return id
	}
	var own *types.Package
	if top := outermost(fn); top.Pkg != nil {
		own = top.Pkg.Pkg
	}
	n := &graph.Node{
		ID: id, Kind: graph.KindFunction, Name: displayName(s), Pkg: s.Pkg, Status: graph.StatusResolved,
		Receiver: s.Recv, Signature: types.TypeString(fn.Signature, byName(own)),
	}
	if syn := fn.Syntax(); syn != nil {
		loc := graph.LocOf(b.p.Fset.Position(syn.Pos()), b.p.Root)
		loc.EndLine = b.p.Fset.Position(syn.End()).Line
		n.Loc = &loc
	}
	b.nodes[id], b.funcs[id] = n, fn
	return id
}

func (b *builder) externalNode(s graph.Sym) graph.NodeID {
	n := External(s)
	if _, ok := b.nodes[n.ID]; !ok {
		b.nodes[n.ID] = &n
	}
	return n.ID
}

// External builds the node for an external function; routes uses it for dependency middleware.
func External(s graph.Sym) graph.Node {
	id := graph.ExternalID(s)
	return graph.Node{
		ID: id, Kind: graph.KindExternal, Name: displayName(s), Pkg: s.Pkg, Status: graph.StatusResolved,
		Category: category(strings.TrimPrefix(string(id), "external:")),
	}
}

// collect records every call site in fn (or one of its instances) under the owner node.
func (b *builder) collect(fn *ssa.Function, ownerID graph.NodeID) {
	out := map[ssa.CallInstruction][]*ssa.Function{}
	if n := b.cg.Nodes[fn]; n != nil {
		for _, e := range n.Out {
			out[e.Site] = append(out[e.Site], e.Callee.Func)
		}
	}
	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			ci, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			c := ci.Common()
			if _, builtin := c.Value.(*ssa.Builtin); builtin {
				continue // len, append, …: not calls in the report (flow skips them too)
			}
			pos := ci.Pos()
			if !pos.IsValid() {
				pos = rangeFuncPos(c)
			}
			if !pos.IsValid() {
				continue
			}
			loc := graph.LocOf(b.p.Fset.Position(pos), b.p.Root)
			ref := siteRef{ownerID, graph.SiteKey(loc)}
			s := b.sites[ref]
			if s == nil {
				s = &site{loc: loc, common: c, targets: map[graph.NodeID]bool{}, externs: map[graph.NodeID]graph.Sym{}}
				b.sites[ref] = s
			}
			callees := out[ci]
			if callee := c.StaticCallee(); callee != nil {
				s.static, callees = true, []*ssa.Function{callee}
			}
			seen := map[*ssa.Function]bool{}
			for _, callee := range callees {
				b.addTargets(s, callee, seen)
			}
		}
	}
}

// rangeFuncPos: SSA gives the iterator call of `for x := range seq` no position; use the
// range statement's `for` keyword (the yield closure's syntax), which flow uses too.
func rangeFuncPos(c *ssa.CallCommon) token.Pos {
	for _, arg := range c.Args {
		if mc, ok := arg.(*ssa.MakeClosure); ok {
			arg = mc.Fn
		}
		if fn, ok := arg.(*ssa.Function); ok && fn.Synthetic == "range-over-func yield" && fn.Syntax() != nil {
			return fn.Syntax().Pos()
		}
	}
	return token.NoPos
}

// addTargets follows synthetic wrappers (bound methods, thunks, promoted-method wrappers) to the
// real functions they call; wrappers never become nodes.
func (b *builder) addTargets(s *site, fn *ssa.Function, seen map[*ssa.Function]bool) {
	if seen[fn] {
		return
	}
	seen[fn] = true
	if fn.Origin() == nil && fn.Synthetic != "" && fn.Synthetic != "range-over-func yield" && len(fn.Blocks) > 0 {
		if n := b.cg.Nodes[fn]; n != nil {
			for _, e := range n.Out {
				b.addTargets(s, e.Callee.Func, seen)
			}
		}
		return
	}
	switch fn = owner(fn); b.scope(fn) {
	case inProject:
		s.targets[b.function(fn)] = true
	case excludedPkg:
		s.excluded = true
	default:
		es := sym(fn)
		s.externs[graph.ExternalID(es)] = es
	}
}

// externalCallee names what a dynamic site calls when that is declared outside the module: an
// interface method (context.Context.Err, error.Error, …) or a named func type (context.CancelFunc).
// Which stdlib/dependency function sits behind it is noise, so such sites get one external edge.
func (b *builder) externalCallee(c *ssa.CallCommon) (graph.Sym, bool) {
	var pkg *types.Package
	var s graph.Sym
	if c.IsInvoke() {
		pkg = c.Method.Pkg()
		recv := c.Method.Type().(*types.Signature).Recv().Type()
		s = graph.Sym{Recv: types.TypeString(recv, func(*types.Package) string { return "" }), Name: c.Method.Name()}
	} else if named, ok := types.Unalias(c.Value.Type()).(*types.Named); ok {
		pkg = named.Obj().Pkg()
		s = graph.Sym{Name: named.Obj().Name()}
	} else {
		return graph.Sym{}, false
	}
	s.Pkg = "builtin"
	if pkg != nil {
		s.Pkg = pkg.Path()
	}
	if b.included[s.Pkg] || s.Pkg == b.p.Module || strings.HasPrefix(s.Pkg, b.p.Module+"/") {
		return graph.Sym{}, false
	}
	return s, true
}

func (b *builder) describe(c *ssa.CallCommon) string {
	if !c.IsInvoke() {
		return "func value call"
	}
	recv := c.Method.Type().(*types.Signature).Recv().Type()
	return "interface call " + types.TypeString(recv, byName(nil)) + "." + c.Method.Name()
}

func (b *builder) emit() *Result {
	r := &Result{Funcs: b.funcs}
	refs := slices.SortedFunc(maps.Keys(b.sites), func(x, y siteRef) int { // deterministic diagnostics order
		return cmp.Or(cmp.Compare(x.key, y.key), cmp.Compare(x.owner, y.owner))
	})
	for _, ref := range refs {
		s := b.sites[ref]
		edge := graph.Edge{Source: ref.owner, Kind: graph.EdgeCalls, Site: s.loc, SiteID: ref.key}
		ext, isExt := b.externalCallee(s.common)
		hasProject := len(s.targets) > 0
		targets := slices.Collect(maps.Keys(s.targets))
		if !isExt || hasProject {
			for _, es := range s.externs {
				targets = append(targets, b.externalNode(es))
			}
		}
		switch {
		case s.static:
			edge.Confidence, edge.Method, edge.Reason = graph.ConfHigh, graph.MethodStatic, "static call"
		case isExt && !hasProject:
			// Which stdlib/dependency type implements it is noise: point at the interface method.
			targets = []graph.NodeID{b.externalNode(ext)}
			edge.Confidence, edge.Method = graph.ConfHigh, graph.MethodStatic
			edge.Reason = b.describe(s.common) + " declared in " + ext.Pkg + "; implementation not tracked"
		case len(targets) == 0:
			targets = []graph.NodeID{b.unresolved(ref, s, r)}
			edge.Confidence, edge.Method = graph.ConfUnknown, graph.MethodUnresolved
			edge.Reason = b.nodes[targets[0]].Reason + ": " + b.describe(s.common) + "; no concrete target reaches this site"
		case len(targets) == 1:
			edge.Confidence, edge.Method = graph.ConfMedium, graph.MethodVTA
			edge.Reason = b.describe(s.common) + "; 1 concrete target reaches this site"
		default:
			edge.Confidence, edge.Method, edge.Alternatives = graph.ConfLow, graph.MethodVTA, len(targets)
			edge.Reason = fmt.Sprintf("%s; %d concrete targets reach this site", b.describe(s.common), len(targets))
		}
		for _, t := range targets {
			edge.Target = t
			r.Edges = append(r.Edges, edge)
		}
	}
	for _, n := range b.nodes {
		r.Nodes = append(r.Nodes, *n)
	}
	return r
}

func (b *builder) unresolved(ref siteRef, s *site, r *Result) graph.NodeID {
	reason := "func_value_unknown"
	name := "func value(...)"
	if s.common.IsInvoke() {
		reason = "no_concrete_type"
		recv := s.common.Method.Type().(*types.Signature).Recv().Type()
		name = types.TypeString(recv, byName(nil)) + "." + s.common.Method.Name() + "(...)"
	} else if isNamedValue(s.common.Value) {
		name = s.common.Value.Name() + "(...)"
	}
	if s.excluded {
		reason = "excluded_target"
	}
	id := graph.UnresolvedID(s.loc)
	loc := s.loc
	b.nodes[id] = &graph.Node{ID: id, Kind: graph.KindUnresolved, Name: name, Status: graph.StatusUnresolved, Reason: reason, Loc: &loc}
	if n := b.nodes[ref.owner]; n != nil {
		n.Status = graph.StatusPartiallyResolved
	}
	r.Diags = append(r.Diags, graph.Diagnostic{
		Severity: graph.SevInfo, Code: "CALL_UNRESOLVED", Loc: &loc, NodeID: id,
		Message: "call target not resolved (" + reason + "): " + name,
	})
	return id
}

// isNamedValue: parameters, free variables and globals carry their source name; SSA temporaries ("t3") don't.
func isNamedValue(v ssa.Value) bool {
	switch v.(type) {
	case *ssa.Parameter, *ssa.FreeVar, *ssa.Global:
		return true
	}
	return false
}
