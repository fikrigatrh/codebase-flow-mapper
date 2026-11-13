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

// Build returns the outline of every project function node in cg.
func Build(p *load.Program, cg *callgraph.Result) (map[graph.NodeID][]graph.Step, []graph.Diagnostic) {
	infos := map[*token.File]*types.Info{}
	for _, pkg := range p.Pkgs {
		for _, f := range pkg.Syntax {
			infos[p.Fset.File(f.Pos())] = pkg.TypesInfo
		}
	}
	out := map[graph.NodeID][]graph.Step{}
	var diags []graph.Diagnostic
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
		w := &walker{fset: p.Fset, info: info, root: p.Root}
		func() {
			defer func() {
				if r := recover(); r != nil {
					loc := w.loc(syn.Pos())
					diags = append(diags, graph.Diagnostic{Severity: graph.SevError, Code: "ANALYZER_PANIC",
						Message: fmt.Sprintf("flow outline of %s failed: %v", id, r), Loc: &loc, NodeID: id})
				}
			}()
			if steps := w.stmts(body.List); len(steps) > 0 {
				out[id] = steps
			}
		}()
	}
	return out, diags
}

type walker struct {
	fset *token.FileSet
	info *types.Info
	root string
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
		return append(pre, graph.Step{K: graph.StepIf, Cond: w.text(s.Cond), Line: w.line(s.If), Then: then, Else: els})
	case *ast.SwitchStmt:
		tag := ""
		if s.Tag != nil {
			tag = w.text(s.Tag)
		}
		return w.switchStmt(s.Switch, tag, s.Body, w.calls(s.Init, s.Tag))
	case *ast.TypeSwitchStmt:
		return w.switchStmt(s.Switch, w.text(s.Assign), s.Body, w.calls(s.Init, s.Assign))
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

func (w *walker) switchStmt(at token.Pos, tag string, body *ast.BlockStmt, pre []graph.Step) []graph.Step {
	var cases []graph.Case
	for _, cc := range body.List {
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
		cases = append(cases, graph.Case{Label: label, Body: steps})
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
