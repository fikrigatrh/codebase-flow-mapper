// Package flow builds the per-function outline (ARCHITECTURE §4.3, GRAPH_SCHEMA §5) from the AST.
// Call sites use the same positions as internal/callgraph (graph.SiteKey): CallExpr.Lparen, the
// go/defer keyword, and the `for` keyword of a range-over-func loop. Builtins and conversions are
// not call sites. Func literals are their own functions and are never descended into.
package flow

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/callgraph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
)

const maxText = 120

// Result is the per-function outline plus the request body each function binds (CFM-401).
type Result struct {
	Steps  map[graph.NodeID][]graph.Step
	Bodies map[graph.NodeID]*graph.Body
	Diags  []graph.Diagnostic
}

// Build returns the outline of every project function node in cg. Bodies are found first: a
// condition is a body test only if it reads a type some function binds.
func Build(p *load.Program, cg *callgraph.Result) Result {
	infos := map[*token.File]*types.Info{}
	for _, pkg := range p.Pkgs {
		for _, f := range pkg.Syntax {
			infos[p.Fset.File(f.Pos())] = pkg.TypesInfo
		}
	}
	type fn struct {
		id   graph.NodeID
		syn  ast.Node
		body *ast.BlockStmt
		w    *walker
	}
	var fns []fn
	dtos := map[string]bool{}
	res := Result{Steps: map[graph.NodeID][]graph.Step{}, Bodies: map[graph.NodeID]*graph.Body{}}
	for _, id := range slices.Sorted(maps.Keys(cg.Funcs)) {
		syn := cg.Funcs[id].Syntax()
		var body *ast.BlockStmt
		switch s := syn.(type) {
		case *ast.FuncDecl:
			body = s.Body
		case *ast.FuncLit:
			body = s.Body
		}
		if body == nil {
			continue
		}
		info := infos[p.Fset.File(syn.Pos())]
		if info == nil {
			continue
		}
		w := &walker{fset: p.Fset, info: info, root: p.Root, dtos: dtos}
		fns = append(fns, fn{id, syn, body, w})
		res.guard(w, id, syn, func() {
			if b := w.body(body.List); b != nil {
				res.Bodies[id] = b
				dtos[b.Type] = true
				for t := range nestedTypes(b.Fields) {
					dtos[t] = true
				}
			}
		})
	}
	for _, f := range fns {
		res.guard(f.w, f.id, f.syn, func() {
			if steps := f.w.stmts(f.body.List); len(steps) > 0 {
				res.Steps[f.id] = steps
			}
		})
	}
	return res
}

// guard turns a panic while reading one function into a diagnostic for that function.
func (r *Result) guard(w *walker, id graph.NodeID, syn ast.Node, do func()) {
	defer func() {
		if p := recover(); p != nil {
			loc := w.loc(syn.Pos())
			r.Diags = append(r.Diags, graph.Diagnostic{Severity: graph.SevError, Code: "ANALYZER_PANIC",
				Message: fmt.Sprintf("flow outline of %s failed: %v", id, p), Loc: &loc, NodeID: id})
		}
	}()
	do()
}

// nestedTypes yields the named struct types inside a body, so a usecase that takes req.Data
// still matches.
func nestedTypes(fs []graph.Field) map[string]bool {
	out := map[string]bool{}
	var walk func(f graph.Field)
	walk = func(f graph.Field) {
		if f.Kind == graph.FieldObject && f.Type != "" {
			out[f.Type] = true
		}
		for _, c := range f.Fields {
			walk(c)
		}
		if f.Elem != nil {
			walk(*f.Elem)
		}
	}
	for _, f := range fs {
		walk(f)
	}
	return out
}

type walker struct {
	fset *token.FileSet
	info *types.Info
	root string
	dtos map[string]bool // qualified names of bound body types (filled before the outline pass)
}

func (w *walker) loc(p token.Pos) graph.Loc { return graph.LocOf(w.fset.Position(p), w.root) }
func (w *walker) line(p token.Pos) int      { return w.fset.Position(p).Line }

func (w *walker) site(k string, p token.Pos) graph.Step {
	loc := w.loc(p)
	return graph.Step{K: k, Site: graph.SiteKey(loc), Line: loc.Line}
}

func (w *walker) stmts(list []ast.Stmt) []graph.Step {
	var out []graph.Step
	for _, s := range list {
		out = append(out, w.stmt(s)...)
	}
	return out
}

func (w *walker) stmt(s ast.Stmt) []graph.Step {
	switch s := s.(type) {
	case *ast.BlockStmt:
		return w.stmts(s.List)
	case *ast.LabeledStmt:
		return w.stmt(s.Stmt)
	case *ast.ExprStmt:
		if c, ok := ast.Unparen(s.X).(*ast.CallExpr); ok && w.builtin(c) == "panic" {
			return append(w.calls(nodes(c.Args)...), graph.Step{K: graph.StepPanic, Line: w.line(s.Pos())})
		}
		return w.calls(s.X)
	case *ast.ReturnStmt:
		return append(w.calls(nodes(s.Results)...), graph.Step{K: graph.StepReturn, Line: w.line(s.Return)})
	case *ast.GoStmt:
		return w.goDefer(graph.StepGo, s.Go, s.Call)
	case *ast.DeferStmt:
		return w.goDefer(graph.StepDefer, s.Defer, s.Call)
	case *ast.IfStmt:
		pre := w.calls(s.Init, s.Cond)
		then := w.stmts(s.Body.List)
		var els []graph.Step
		if s.Else != nil {
			els = w.stmt(s.Else)
		}
		if len(then) == 0 && len(els) == 0 {
			return pre
		}
		return append(pre, graph.Step{K: graph.StepIf, Cond: w.text(s.Cond), Line: w.line(s.If), Then: then, Else: els, Test: w.test(s.Cond)})
	case *ast.SwitchStmt:
		return w.switchStmt(s, w.calls(s.Init, s.Tag))
	case *ast.TypeSwitchStmt:
		return w.switchStmt(s, w.calls(s.Init, s.Assign))
	case *ast.ForStmt:
		body := append(w.calls(s.Cond), w.stmts(s.Body.List)...)
		body = append(body, w.calls(s.Post)...)
		pre := w.calls(s.Init)
		if len(body) == 0 {
			return pre
		}
		head := w.src(s.Cond)
		if s.Init != nil || s.Post != nil {
			head = w.src(s.Init) + "; " + w.src(s.Cond) + "; " + w.src(s.Post)
		}
		return append(pre, graph.Step{K: graph.StepLoop, Head: trunc(head), Line: w.line(s.For), Body: body})
	case *ast.RangeStmt:
		pre := w.calls(s.X)
		if _, isFunc := w.info.TypeOf(s.X).Underlying().(*types.Signature); isFunc {
			pre = append(pre, w.site(graph.StepCall, s.For)) // the iterator call (callgraph rangeFuncPos)
		}
		body := w.stmts(s.Body.List)
		if len(body) == 0 {
			return pre
		}
		head := "range " + w.src(s.X)
		if s.Key != nil {
			lhs := w.src(s.Key)
			if s.Value != nil {
				lhs += ", " + w.src(s.Value)
			}
			head = lhs + " " + s.Tok.String() + " " + head
		}
		return append(pre, graph.Step{K: graph.StepLoop, Head: trunc(head), Line: w.line(s.For), Body: body})
	case *ast.SelectStmt:
		var pre []graph.Step
		var cases []graph.Case
		for _, cc := range s.Body.List {
			c := cc.(*ast.CommClause)
			label := "default"
			if c.Comm != nil {
				pre = append(pre, w.calls(c.Comm)...)
				label = w.text(c.Comm)
			}
			if body := w.stmts(c.Body); len(body) > 0 {
				cases = append(cases, graph.Case{Label: label, Body: body})
			}
		}
		if len(cases) == 0 {
			return pre
		}
		return append(pre, graph.Step{K: graph.StepSelect, Line: w.line(s.Select), Cases: cases})
	}
	return w.calls(s) // assign, decl, inc/dec, send, branch, empty
}

// switchStmt handles *ast.SwitchStmt and *ast.TypeSwitchStmt; only the former gets body tests.
func (w *walker) switchStmt(sw ast.Stmt, pre []graph.Step) []graph.Step {
	var (
		at    token.Pos
		tag   string
		body  *ast.BlockStmt
		tests []*graph.Test
	)
	switch s := sw.(type) {
	case *ast.SwitchStmt:
		at, body, tests = s.Switch, s.Body, w.caseTests(s.Tag, s.Body)
		if s.Tag != nil {
			tag = w.text(s.Tag)
		}
	case *ast.TypeSwitchStmt:
		at, body, tag = s.Switch, s.Body, w.text(s.Assign)
	}
	var cases []graph.Case
	for i, cc := range body.List {
		c := cc.(*ast.CaseClause)
		pre = append(pre, w.calls(nodes(c.List)...)...)
		steps := w.stmts(c.Body)
		if len(steps) == 0 {
			continue
		}
		label := "default"
		if c.List != nil {
			parts := make([]string, len(c.List))
			for i, e := range c.List {
				parts[i] = w.src(e)
			}
			label = trunc(strings.Join(parts, ", "))
		}
		gc := graph.Case{Label: label, Body: steps}
		if tests != nil {
			gc.Test = tests[i]
		}
		cases = append(cases, gc)
	}
	if len(cases) == 0 {
		return pre
	}
	return append(pre, graph.Step{K: graph.StepSwitch, Tag: tag, Line: w.line(at), Cases: cases})
}

// goDefer: the function value and arguments are evaluated at the statement, then go/defer runs the
// call. A builtin (defer close(ch)) is not a call site, so only its argument calls remain.
func (w *walker) goDefer(k string, kw token.Pos, c *ast.CallExpr) []graph.Step {
	if !w.isCall(c) {
		return w.calls(c)
	}
	pre := append(w.calls(c.Fun), w.calls(nodes(c.Args)...)...)
	return append(pre, w.site(k, kw))
}

// calls returns a call step per call site in ns, in evaluation order (post-order: arguments first).
func (w *walker) calls(ns ...ast.Node) []graph.Step {
	var out []graph.Step
	for _, n := range ns {
		if n == nil {
			continue
		}
		var stack []ast.Node
		ast.Inspect(n, func(x ast.Node) bool {
			if x == nil {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if c, ok := top.(*ast.CallExpr); ok && w.isCall(c) {
					out = append(out, w.site(graph.StepCall, c.Lparen))
				}
				return true
			}
			if _, lit := x.(*ast.FuncLit); lit {
				return false // its own function node
			}
			stack = append(stack, x)
			return true
		})
	}
	return out
}

func (w *walker) isCall(c *ast.CallExpr) bool {
	tv := w.info.Types[c.Fun]
	return !tv.IsType() && !tv.IsBuiltin()
}

// builtin is the name of the builtin c calls, or "".
func (w *walker) builtin(c *ast.CallExpr) string {
	if id, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
		if b, ok := w.info.Uses[id].(*types.Builtin); ok {
			return b.Name()
		}
	}
	return ""
}

var newlines = regexp.MustCompile(`\s*\n\s*`)

// src is n printed as Go on one line; "" for nil.
func (w *walker) src(n ast.Node) string {
	if n == nil {
		return ""
	}
	var b bytes.Buffer
	if err := printer.Fprint(&b, w.fset, n); err != nil {
		return "?"
	}
	return newlines.ReplaceAllString(b.String(), " ")
}

func (w *walker) text(n ast.Node) string { return trunc(w.src(n)) }

func trunc(s string) string {
	if r := []rune(s); len(r) > maxText {
		return string(r[:maxText]) + "…"
	}
	return s
}

func nodes[T ast.Node](xs []T) []ast.Node {
	out := make([]ast.Node, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}
