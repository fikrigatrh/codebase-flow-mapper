// Package routes finds HTTP endpoints by walking the AST with types.Info (ARCHITECTURE §4.2):
// router values carry a prefix and middleware chain per types.Object, propagated into functions
// that take a router parameter up to a fixed point.
package routes

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/callgraph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
)

const maxDepth = 8

type Result struct {
	Nodes []graph.Node // endpoint, plus external/unresolved nodes for handlers and middleware
	Edges []graph.Edge // routes_to, uses_middleware
	Diags []graph.Diagnostic
}

// target is a resolved handler or middleware.
type target struct {
	id     graph.NodeID
	conf   graph.Confidence
	method graph.Method
	scope  string // middleware only: applies to paths with this prefix ("" = all), Fiber Use("/api", mw)
}

// state is what is known about a router value: prefix "?" means unknown.
type state struct {
	fw     *framework
	prefix string
	chain  []target
}

func (s *state) with(extra []target) *state {
	return &state{fw: s.fw, prefix: s.prefix, chain: append(slices.Clone(s.chain), extra...)}
}

func (s *state) key() string {
	var b strings.Builder
	b.WriteString(s.fw.name + "|" + s.prefix)
	for _, t := range s.chain {
		b.WriteString("|" + string(t.id))
	}
	return b.String()
}

type opKind int

const (
	opRoot opKind = iota + 1
	opGroup
	opUse
	opRegister
)

// rcall is a framework call split into the parts the core needs. methodArg/pathArg are -1 when absent.
type rcall struct {
	kind      opKind
	methods   []string // fixed methods; nil means read methodArg
	methodArg int
	pathArg   int
	handlers  []ast.Expr // register: last is the handler, earlier ones route middleware
	implicit  bool       // register on the framework's default router (http.HandleFunc)
	usePrefix bool       // use: a leading string (or []string) argument is a path prefix, not a handler
	route     bool       // group: the argument after the path is a func(router) receiving the group
}

// argsFrom is args[i:], or nil when there are fewer arguments.
func argsFrom(args []ast.Expr, i int) []ast.Expr {
	if i > len(args) {
		return nil
	}
	return args[i:]
}

type framework struct {
	name     string
	pkg      string
	root     string   // prefix of a fresh router
	types    []string // router type names in pkg
	classify func(fn *types.Func, call *ast.CallExpr) (rcall, bool)
	join     func(prefix, path string) string
	pattern  func(p string) (methods []string, path string) // nil: the path argument is only a path
	unwrap   bool                                           // w(inner) handler calls are middleware wrappers
}

var frameworks []*framework

// init breaks the cycle classify → routerType → frameworks.
func init() { frameworks = []*framework{ginFramework, netHTTPFramework, fiberFramework} }

func frameworkOf(fn *types.Func) *framework {
	if fn.Pkg() == nil {
		return nil
	}
	for _, fw := range frameworks {
		if fn.Pkg().Path() == fw.pkg {
			return fw
		}
	}
	return nil
}

// routerType reports the framework whose router type t is (pointers unwrapped).
func routerType(t types.Type) *framework {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return nil
	}
	for _, fw := range frameworks {
		if named.Obj().Pkg().Path() == fw.pkg && slices.Contains(fw.types, named.Obj().Name()) {
			return fw
		}
	}
	return nil
}

type funcDecl struct {
	decl *ast.FuncDecl
	info *types.Info
	obj  *types.Func
}

type job struct {
	fd     *funcDecl
	params map[types.Object]*state
	depth  int
}

type analyzer struct {
	p      *load.Program
	byObj  map[*types.Func]graph.NodeID
	byLit  map[*ast.FuncLit]graph.NodeID
	decls  map[*types.Func]*funcDecl
	order  []*funcDecl // source order, for determinism
	fields map[*types.Var][]*state
	frozen map[*types.Var]*state // pass 2: fields assigned exactly once
	pass   int

	queue    []job
	seen     map[string]bool
	received map[*funcDecl]bool
	depthHit map[*funcDecl]bool

	nodes    map[graph.NodeID]*graph.Node
	out      Result
	diagSeen map[string]bool
}

func Build(p *load.Program, cg *callgraph.Result) *Result {
	a := &analyzer{p: p, byObj: map[*types.Func]graph.NodeID{}, byLit: map[*ast.FuncLit]graph.NodeID{},
		decls: map[*types.Func]*funcDecl{}, fields: map[*types.Var][]*state{}}
	for id, fn := range cg.Funcs {
		if obj, ok := fn.Object().(*types.Func); ok {
			a.byObj[obj] = id
		}
		if lit, ok := fn.Syntax().(*ast.FuncLit); ok {
			a.byLit[lit] = id
		}
	}
	for _, pkg := range p.Pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					if obj, ok := pkg.TypesInfo.Defs[fd.Name].(*types.Func); ok {
						d := &funcDecl{decl: fd, info: pkg.TypesInfo, obj: obj}
						a.decls[obj] = d
						a.order = append(a.order, d)
					}
				}
			}
		}
	}
	slices.SortFunc(a.order, func(x, y *funcDecl) int { return a.cmpPos(x.decl.Pos(), y.decl.Pos()) })

	// Pass 1 only learns struct-field assignments (rule 5); pass 2 is the real run.
	a.run(1)
	a.frozen = map[*types.Var]*state{}
	for f, sts := range a.fields {
		if len(sts) == 1 {
			a.frozen[f] = sts[0]
		}
	}
	a.run(2)
	a.unsupported()
	for _, n := range a.nodes {
		a.out.Nodes = append(a.out.Nodes, *n)
	}
	return &a.out
}

// unsupportedRouters are module paths whose routes CFM does not extract (PRD scope).
var unsupportedRouters = []string{"github.com/gofiber/fiber/v3", "github.com/labstack/echo", "github.com/go-chi/chi",
	"github.com/gorilla/mux", "github.com/julienschmidt/httprouter"}

// unsupported reports the first import of each unsupported router, even in packages that failed to load.
func (a *analyzer) unsupported() {
	first := map[string]*ast.ImportSpec{}
	for _, pkg := range a.p.Pkgs {
		for _, f := range pkg.Syntax {
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, u := range unsupportedRouters {
					if path == u || strings.HasPrefix(path, u+"/") {
						if old := first[u]; old == nil || a.cmpPos(imp.Pos(), old.Pos()) < 0 {
							first[u] = imp
						}
					}
				}
			}
		}
	}
	for _, u := range slices.Sorted(maps.Keys(first)) {
		a.diag(graph.SevWarning, "ROUTER_UNSUPPORTED", u+" is not a supported router: its routes are not extracted", first[u].Pos())
	}
}

func (a *analyzer) cmpPos(x, y token.Pos) int {
	px, py := a.p.Fset.Position(x), a.p.Fset.Position(y)
	if c := strings.Compare(px.Filename, py.Filename); c != 0 {
		return c
	}
	return px.Offset - py.Offset
}

func (a *analyzer) run(pass int) {
	a.pass, a.out = pass, Result{}
	a.nodes, a.diagSeen = map[graph.NodeID]*graph.Node{}, map[string]bool{}
	a.seen, a.received, a.depthHit = map[string]bool{}, map[*funcDecl]bool{}, map[*funcDecl]bool{}
	for _, fd := range a.order {
		if len(a.routerParams(fd)) == 0 {
			a.queue = append(a.queue, job{fd: fd})
		}
	}
	a.drain()
	// Functions taking a router that no tracked call reached: run them with an unknown prefix.
	for _, fd := range a.order {
		if ps := a.routerParams(fd); len(ps) > 0 && !a.received[fd] {
			params := map[types.Object]*state{}
			for v, fw := range ps {
				params[v] = &state{fw: fw, prefix: "?"}
			}
			a.diag(graph.SevWarning, "ROUTE_PREFIX_UNKNOWN", "router parameter of "+fd.obj.Name()+" is never passed a tracked router: prefix unknown", fd.decl.Name.Pos())
			a.queue = append(a.queue, job{fd: fd, params: params})
			a.drain()
		}
	}
}

func (a *analyzer) routerParams(fd *funcDecl) map[types.Object]*framework {
	ps := map[types.Object]*framework{}
	sig := fd.obj.Type().(*types.Signature)
	for i := range sig.Params().Len() {
		if v := sig.Params().At(i); routerType(v.Type()) != nil {
			ps[v] = routerType(v.Type())
		}
	}
	return ps
}

func (a *analyzer) drain() {
	for len(a.queue) > 0 {
		j := a.queue[0]
		a.queue = a.queue[1:]
		a.walk(j)
	}
}

func (a *analyzer) walk(j job) {
	env := map[types.Object]*state{}
	for v, s := range j.params {
		env[v] = s
	}
	info := j.fd.info
	ast.Inspect(j.fd.decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) == len(n.Rhs) {
				for i := range n.Lhs {
					a.bind(env, info, n.Lhs[i], n.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) == len(n.Values) {
				for i := range n.Names {
					a.bind(env, info, n.Names[i], n.Values[i])
				}
			}
		case *ast.CompositeLit:
			for _, elt := range n.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*ast.Ident); ok {
						a.bind(env, info, key, kv.Value)
					}
				}
			}
		case *ast.CallExpr:
			a.call(env, j, n)
		}
		return true
	})
}

// objectOf returns the variable or field an expression denotes.
func objectOf(info *types.Info, e ast.Expr) types.Object {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if o := info.Defs[e]; o != nil {
			return o
		}
		return info.Uses[e]
	case *ast.SelectorExpr:
		if sel := info.Selections[e]; sel != nil {
			if sel.Kind() == types.FieldVal {
				return sel.Obj()
			}
			return nil
		}
		return info.Uses[e.Sel] // package-qualified, e.g. http.DefaultServeMux
	}
	return nil
}

func (a *analyzer) bind(env map[types.Object]*state, info *types.Info, lhs, rhs ast.Expr) {
	obj := objectOf(info, lhs)
	if obj == nil || routerType(obj.Type()) == nil {
		return
	}
	st, ok := a.eval(env, info, rhs)
	if !ok {
		return
	}
	env[obj] = st
	if v, ok := obj.(*types.Var); ok && v.IsField() && a.pass == 1 {
		a.fields[v] = append(a.fields[v], st)
	}
}

func classify(info *types.Info, call *ast.CallExpr) (*types.Func, *framework, rcall, bool) {
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	if fn == nil {
		return nil, nil, rcall{}, false
	}
	fw := frameworkOf(fn)
	if fw == nil {
		return fn, nil, rcall{}, false
	}
	rc, ok := fw.classify(fn, call)
	return fn, fw, rc, ok
}

func receiver(call *ast.CallExpr) ast.Expr {
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		return sel.X
	}
	return nil
}

// eval computes the router state of an expression; ok=false if it is not a tracked router value.
func (a *analyzer) eval(env map[types.Object]*state, info *types.Info, e ast.Expr) (*state, bool) {
	e = ast.Unparen(e)
	if call, ok := e.(*ast.CallExpr); ok {
		_, fw, rc, ok := classify(info, call)
		if !ok {
			return nil, false
		}
		switch rc.kind {
		case opRoot:
			return &state{fw: fw, prefix: fw.root}, true
		case opGroup:
			base := a.recvState(env, info, call)
			grp := &state{fw: fw, prefix: fw.join(base.prefix, a.path(info, call, rc)), chain: base.chain}
			return grp.with(a.targets(info, rc.handlers)), true
		case opUse:
			return a.use(env, info, call, rc), true
		case opRegister:
			return a.recvState(env, info, call), true // registrations return their router (chaining)
		}
		return nil, false
	}
	obj := objectOf(info, e)
	if obj == nil {
		return nil, false
	}
	if st := env[obj]; st != nil {
		return st, true
	}
	v, ok := obj.(*types.Var)
	if !ok {
		return nil, false
	}
	if v.IsField() && a.frozen[v] != nil {
		env[v] = a.frozen[v]
		return env[v], true
	}
	// A framework's own package-level router (http.DefaultServeMux) is a root.
	if fw := routerType(v.Type()); fw != nil && v.Pkg() != nil && v.Pkg().Path() == fw.pkg && !v.IsField() {
		return &state{fw: fw, prefix: fw.root}, true
	}
	return nil, false
}

// recvState is the state of a router method's receiver; untracked receivers get an unknown prefix.
func (a *analyzer) recvState(env map[types.Object]*state, info *types.Info, call *ast.CallExpr) *state {
	recv := receiver(call)
	if st, ok := a.eval(env, info, recv); ok {
		return st
	}
	fw := routerType(info.TypeOf(recv))
	if fw == nil {
		_, fw, _, _ = classify(info, call)
	}
	a.diag(graph.SevWarning, "ROUTE_PREFIX_UNKNOWN", "router "+types.ExprString(recv)+" is not tracked (field assigned more than once or never, or returned by a call): prefix unknown", call.Pos())
	return &state{fw: fw, prefix: "?"}
}

func (a *analyzer) call(env map[types.Object]*state, j job, call *ast.CallExpr) {
	info := j.fd.info
	fn, fw, rc, ok := classify(info, call)
	switch {
	case ok && rc.kind == opUse:
		if obj := objectOf(info, receiver(call)); obj != nil {
			env[obj] = a.use(env, info, call, rc)
		}
	case ok && rc.kind == opGroup && rc.route:
		// Route("/x", func(r Router) {...}): the literal's parameter is the group. ast.Inspect visits
		// the literal after this call, so binding it here is enough. A named func gets the "?" fallback.
		st, _ := a.eval(env, info, call)
		if lit, isLit := argsFrom(call.Args, 1)[0].(*ast.FuncLit); isLit && len(lit.Type.Params.List) > 0 && len(lit.Type.Params.List[0].Names) > 0 {
			if obj := info.Defs[lit.Type.Params.List[0].Names[0]]; obj != nil {
				env[obj] = st
			}
		}
	case ok && rc.kind == opRegister && rc.implicit:
		a.register(&state{fw: fw, prefix: fw.root}, info, call, rc)
	case ok && rc.kind == opRegister:
		a.register(a.recvState(env, info, call), info, call, rc)
	case fw == nil && fn != nil:
		a.propagate(env, j, call, fn)
	}
}

// use is the receiver's state after r.Use(...), with Fiber's optional leading prefix as the scope.
func (a *analyzer) use(env map[types.Object]*state, info *types.Info, call *ast.CallExpr, rc rcall) *state {
	st := a.recvState(env, info, call)
	hs, scope := rc.handlers, ""
	if rc.usePrefix && len(hs) > 0 {
		switch t := info.TypeOf(hs[0]).Underlying().(type) {
		case *types.Basic:
			if t.Info()&types.IsString != 0 {
				scope = st.fw.join(st.prefix, a.path(info, call, rcall{pathArg: 0}))
				hs = hs[1:]
			}
		case *types.Slice:
			hs = hs[1:] // ponytail: []string prefixes are not scoped; the middleware shows on every later route
		}
	}
	ts := a.targets(info, hs)
	for i := range ts {
		ts[i].scope = scope
	}
	return st.with(ts)
}

// propagate passes router arguments into a project function (rule 4).
func (a *analyzer) propagate(env map[types.Object]*state, j job, call *ast.CallExpr, fn *types.Func) {
	fd := a.decls[fn.Origin()]
	if fd == nil {
		return
	}
	ps := a.routerParams(fd)
	if len(ps) == 0 {
		return
	}
	sig := fd.obj.Type().(*types.Signature)
	params := map[types.Object]*state{}
	key := fd.obj.FullName()
	for i := range sig.Params().Len() {
		v := sig.Params().At(i)
		fw := ps[v]
		if fw == nil || i >= len(call.Args) {
			continue
		}
		st, ok := a.eval(env, j.fd.info, call.Args[i])
		if !ok {
			st = &state{fw: fw, prefix: "?"}
			a.diag(graph.SevWarning, "ROUTE_PREFIX_UNKNOWN", "router argument "+types.ExprString(call.Args[i])+" is not tracked: prefix unknown", call.Args[i].Pos())
		}
		params[v] = st
		key += "#" + st.key()
	}
	a.received[fd] = true
	if a.seen[key] {
		return
	}
	a.seen[key] = true
	if j.depth+1 > maxDepth {
		if !a.depthHit[fd] {
			a.depthHit[fd] = true
			a.diag(graph.SevWarning, "ROUTE_PREFIX_DEPTH", "router passed through more than 8 calls into "+fd.obj.Name()+": deeper registrations are not followed", call.Pos())
		}
		return
	}
	a.queue = append(a.queue, job{fd: fd, params: params, depth: j.depth + 1})
}

// path returns the constant path argument, or "?" with ROUTE_PATH_DYNAMIC.
func (a *analyzer) path(info *types.Info, call *ast.CallExpr, rc rcall) string {
	if rc.pathArg < 0 || rc.pathArg >= len(call.Args) {
		return ""
	}
	if s, ok := constString(info, call.Args[rc.pathArg]); ok {
		return s
	}
	a.diag(graph.SevWarning, "ROUTE_PATH_DYNAMIC", "route path is not a constant; kept with a '?' segment", call.Args[rc.pathArg].Pos())
	return "?"
}

func constString(info *types.Info, e ast.Expr) (string, bool) {
	if tv, ok := info.Types[e]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
		return constant.StringVal(tv.Value), true
	}
	return "", false
}

// methods of a registration: fixed, a constant string, a literal of constant strings, else ANY.
func methods(info *types.Info, call *ast.CallExpr, rc rcall) []string {
	if rc.methods != nil || rc.methodArg < 0 || rc.methodArg >= len(call.Args) {
		return rc.methods
	}
	arg := call.Args[rc.methodArg]
	if s, ok := constString(info, arg); ok {
		return []string{strings.ToUpper(s)}
	}
	if lit, ok := ast.Unparen(arg).(*ast.CompositeLit); ok {
		var ms []string
		for _, elt := range lit.Elts {
			s, ok := constString(info, elt)
			if !ok {
				return []string{"ANY"}
			}
			ms = append(ms, strings.ToUpper(s))
		}
		return ms
	}
	return []string{"ANY"}
}

func (a *analyzer) register(st *state, info *types.Info, call *ast.CallExpr, rc rcall) {
	if len(rc.handlers) == 0 {
		return
	}
	rel, ms := a.path(info, call, rc), methods(info, call, rc)
	if st.fw.pattern != nil && rel != "?" {
		ms, rel = st.fw.pattern(rel)
	}
	path := st.fw.join(st.prefix, rel)
	hs := slices.Clone(rc.handlers)
	var wrappers []target
	if st.fw.unwrap {
		wrappers, hs[len(hs)-1] = a.unwrap(info, hs[len(hs)-1])
	}
	ts := a.targets(info, hs)
	var chain []target
	for _, mw := range st.chain {
		if strings.HasPrefix(path, mw.scope) {
			chain = append(chain, mw)
		}
	}
	handler := ts[len(ts)-1]
	chain = append(append(chain, ts[:len(ts)-1]...), wrappers...)
	reg := a.loc(call.Pos())
	status := graph.StatusResolved
	if strings.Contains(path, "?") || handler.method == graph.MethodUnresolved {
		status = graph.StatusPartiallyResolved
	}
	for _, m := range ms {
		id := graph.EndpointID(st.fw.name, m, path, reg)
		if a.nodes[id] != nil {
			continue // ponytail: same route+site reached twice with different chains keeps the first
		}
		a.nodes[id] = &graph.Node{ID: id, Kind: graph.KindEndpoint, Name: m + " " + path, Status: status,
			Framework: st.fw.name, Method: m, Path: path, Loc: &reg}
		a.out.Edges = append(a.out.Edges, graph.Edge{Source: id, Target: handler.id, Kind: graph.EdgeRoutesTo,
			Confidence: handler.conf, Method: handler.method, Reason: "registered as handler", Site: reg})
		for i, mw := range chain {
			a.out.Edges = append(a.out.Edges, graph.Edge{Source: id, Target: mw.id, Kind: graph.EdgeUsesMiddleware,
				Confidence: mw.conf, Method: mw.method, Reason: "middleware in the chain", Site: reg, Order: &i})
		}
	}
}

func (a *analyzer) targets(info *types.Info, es []ast.Expr) []target {
	ts := make([]target, len(es))
	for i, e := range es {
		ts[i] = a.resolve(info, e)
	}
	return ts
}

// resolve maps a handler expression to its node (ARCHITECTURE §4.2 handler rules).
func (a *analyzer) resolve(info *types.Info, e ast.Expr) target {
	e = unconvert(info, e)
	direct := func(id graph.NodeID) target {
		return target{id: id, conf: graph.ConfHigh, method: graph.MethodFrameworkRegistration}
	}
	if m := serveHTTP(info.TypeOf(e)); m != nil {
		if t, ok := a.funcTarget(m, direct); ok {
			return t
		}
		if _, isCall := e.(*ast.CallExpr); isAbstract(m) && !isCall {
			return a.unresolved(e, "no_concrete_type")
		}
	}
	switch x := e.(type) {
	case *ast.FuncLit:
		if id, ok := a.byLit[x]; ok {
			return direct(id)
		}
	case *ast.Ident, *ast.SelectorExpr:
		if fn, ok := objOfFunc(info, x); ok {
			if t, ok := a.funcTarget(fn, direct); ok {
				return t
			}
			if isAbstract(fn) {
				return a.unresolved(e, "no_concrete_type")
			}
		}
	case *ast.CallExpr:
		// A middleware factory call: the handler is the literal it returns, else the factory.
		if fn, ok := typeutil.Callee(info, x).(*types.Func); ok {
			heur := func(id graph.NodeID) target {
				return target{id: id, conf: graph.ConfMedium, method: graph.MethodWrapperHeuristic}
			}
			if fd := a.decls[fn.Origin()]; fd != nil {
				if lit := returnedLiteral(fd.decl); lit != nil && a.byLit[lit] != "" {
					return heur(a.byLit[lit])
				}
			}
			if t, ok := a.funcTarget(fn, heur); ok {
				return t
			}
		}
	}
	return a.unresolved(e, "func_value_unknown")
}

// unconvert strips parens and type conversions: http.HandlerFunc(f) → f.
func unconvert(info *types.Info, e ast.Expr) ast.Expr {
	for {
		e = ast.Unparen(e)
		call, ok := e.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || !info.Types[call.Fun].IsType() {
			return e
		}
		e = call.Args[0]
	}
}

// serveHTTP is the ServeHTTP method of a non-func type (a named handler type or an interface).
func serveHTTP(t types.Type) *types.Func {
	if t == nil {
		return nil
	}
	if _, isFunc := t.Underlying().(*types.Signature); isFunc {
		return nil
	}
	m, _, _ := types.LookupFieldOrMethod(t, true, nil, "ServeHTTP")
	fn, _ := m.(*types.Func)
	return fn
}

// unwrap peels w(inner) calls to project functions off a handler: each w is middleware
// (outermost first, medium/wrapper_heuristic) and inner is the argument that is a handler.
func (a *analyzer) unwrap(info *types.Info, e ast.Expr) ([]target, ast.Expr) {
	var ws []target
	for {
		e = unconvert(info, e)
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return ws, e
		}
		fn, _ := typeutil.Callee(info, call).(*types.Func)
		id, inProject := graph.NodeID(""), false
		if fn != nil {
			id, inProject = a.byObj[fn.Origin()]
		}
		inner := handlerArg(info, call)
		if !inProject || inner == nil {
			return ws, e
		}
		ws = append(ws, target{id: id, conf: graph.ConfMedium, method: graph.MethodWrapperHeuristic})
		e = inner
	}
}

// handlerArg is the first argument that is a func or has ServeHTTP.
func handlerArg(info *types.Info, call *ast.CallExpr) ast.Expr {
	for _, arg := range call.Args {
		t := info.TypeOf(arg)
		if t == nil {
			continue
		}
		if _, isFunc := t.Underlying().(*types.Signature); isFunc || serveHTTP(t) != nil {
			return arg
		}
	}
	return nil
}

func objOfFunc(info *types.Info, e ast.Expr) (*types.Func, bool) {
	id, ok := e.(*ast.Ident)
	if sel, isSel := e.(*ast.SelectorExpr); isSel {
		id, ok = sel.Sel, true
	}
	if !ok {
		return nil, false
	}
	fn, ok := info.Uses[id].(*types.Func)
	return fn, ok
}

func isAbstract(fn *types.Func) bool {
	recv := fn.Type().(*types.Signature).Recv()
	return recv != nil && types.IsInterface(recv.Type())
}

// funcTarget: a project function node, or an external node for a dependency's function.
func (a *analyzer) funcTarget(fn *types.Func, mk func(graph.NodeID) target) (target, bool) {
	if id, ok := a.byObj[fn.Origin()]; ok {
		return mk(id), true
	}
	if fn.Pkg() == nil || isAbstract(fn) {
		return target{}, false
	}
	path := fn.Pkg().Path()
	if path == a.p.Module || strings.HasPrefix(path, a.p.Module+"/") {
		return target{}, false // in-module but excluded or not built
	}
	s := graph.Sym{Pkg: path, Name: fn.Name()}
	if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
		s.Recv = types.TypeString(recv.Type(), func(*types.Package) string { return "" })
	}
	n := callgraph.External(s)
	a.nodes[n.ID] = &n
	return mk(n.ID), true
}

// returnedLiteral is the func literal of a factory whose only return statement returns it.
func returnedLiteral(fd *ast.FuncDecl) *ast.FuncLit {
	var rets []*ast.ReturnStmt
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			rets = append(rets, n)
		}
		return true
	})
	if len(rets) != 1 || len(rets[0].Results) != 1 {
		return nil
	}
	lit, _ := ast.Unparen(rets[0].Results[0]).(*ast.FuncLit)
	return lit
}

func (a *analyzer) unresolved(e ast.Expr, reason string) target {
	loc := a.loc(e.Pos())
	id := graph.UnresolvedID(loc)
	name := types.ExprString(e)
	if len(name) > 80 {
		name = name[:77] + "..."
	}
	a.nodes[id] = &graph.Node{ID: id, Kind: graph.KindUnresolved, Name: name, Status: graph.StatusUnresolved, Reason: reason, Loc: &loc}
	a.diag(graph.SevWarning, "HANDLER_UNRESOLVED", "handler or middleware "+name+" not resolved ("+reason+")", e.Pos())
	return target{id: id, conf: graph.ConfUnknown, method: graph.MethodUnresolved}
}

func (a *analyzer) loc(p token.Pos) graph.Loc { return graph.LocOf(a.p.Fset.Position(p), a.p.Root) }

func (a *analyzer) diag(sev graph.Severity, code, msg string, at token.Pos) {
	loc := a.loc(at)
	key := code + graph.SiteKey(loc) + msg
	if a.diagSeen[key] {
		return
	}
	a.diagSeen[key] = true
	a.out.Diags = append(a.out.Diags, graph.Diagnostic{Severity: sev, Code: code, Message: msg, Loc: &loc})
}
