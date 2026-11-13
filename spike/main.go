// Spike CFM-001: does VTA resolve handler → service → repo through DI interfaces?
// Throwaway code. Not the product.
package main

import (
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var excludeGlobs = []string{"*fakes", "*mocks", "mock*", "testutil*"}

func main() {
	dir := flag.String("dir", ".", "module root to analyze")
	match := flag.String("func", "", "substring of function name to use as roots; empty = auto-detect HTTP handlers")
	depth := flag.Int("depth", 6, "max tree depth")
	quiet := flag.Bool("summary", false, "print only the per-root summary, not trees")
	scope := flag.String("scope", "", "import-path prefix treated as project code (default: module path)")
	lean := flag.Bool("lean", false, "build SSA bodies only for module packages (deps are opaque)")
	flag.Parse()

	start := time.Now()
	prog, modPath, err := load(*dir, *lean, *scope)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	if *scope != "" {
		modPath = *scope
	}
	loaded := time.Since(start)
	funcs := ssautil.AllFunctions(prog)
	if *lean {
		for fn := range funcs {
			if !inModule(fn, modPath) && (fn.Pkg == nil || !isRouterPkg(fn.Pkg.Pkg.Path())) {
				delete(funcs, fn)
			}
		}
	}
	cg := vta.CallGraph(funcs, cha.CallGraph(prog))
	cg.DeleteSyntheticNodes()
	analyzed := time.Since(start)

	t := tracer{cg: cg, mod: modPath, fset: prog.Fset, root: *dir, depth: *depth, quiet: *quiet}
	roots := findRoots(cg, modPath, *match)
	for _, fn := range roots {
		t.report(fn)
	}

	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	fmt.Printf("\nroots=%d load=%s total=%s maxRSS=%dMB (darwin: bytes)\n",
		len(roots), loaded.Round(time.Millisecond), analyzed.Round(time.Millisecond), ru.Maxrss/1024/1024)
}

func load(dir string, lean bool, scope string) (*ssa.Program, string, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:   dir,
		Tests: false,
		// ADR-105 safety env: no downloads, no toolchain fetch, no cgo, no go.mod rewrite.
		Env: append(os.Environ(), "GOFLAGS=-mod=readonly", "GOPROXY=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0"),
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, "", err
	}
	var keep []*packages.Package
	modPath := ""
	for _, p := range pkgs {
		for _, e := range p.Errors {
			fmt.Fprintln(os.Stderr, "diag GO_LOAD_ERROR:", e)
		}
		if p.Module != nil && modPath == "" {
			modPath = p.Module.Path
		}
		if excluded(p.PkgPath) {
			fmt.Fprintln(os.Stderr, "diag PACKAGE_EXCLUDED:", p.PkgPath)
			continue
		}
		keep = append(keep, p)
	}
	prog, spkgs := ssautil.AllPackages(keep, ssa.InstantiateGenerics)
	if !lean {
		prog.Build()
		return prog, modPath, nil
	}
	// ponytail: deps outside scope get no bodies, so flows through library code are lost.
	_ = spkgs
	if scope == "" {
		scope = modPath
	}
	for _, sp := range prog.AllPackages() {
		if p := sp.Pkg.Path(); inScope(p, scope) || isRouterPkg(p) {
			sp.Build()
		}
	}
	return prog, modPath, nil
}

func inScope(p, scope string) bool { return p == scope || strings.HasPrefix(p, scope+"/") }

// Router packages call the handlers; their bodies carry the receiver flow into handlers (lean mode).
func isRouterPkg(p string) bool {
	for _, r := range []string{"github.com/gin-gonic/gin", "github.com/gofiber/fiber/v2", "net/http"} {
		if inScope(p, r) {
			return true
		}
	}
	return false
}

func excluded(pkgPath string) bool {
	last := path.Base(pkgPath)
	for _, g := range excludeGlobs {
		if ok, _ := path.Match(g, last); ok {
			return true
		}
	}
	return false
}

func inModule(fn *ssa.Function, mod string) bool {
	for fn.Parent() != nil {
		fn = fn.Parent()
	}
	p := fn.Pkg
	if p == nil && fn.Origin() != nil {
		p = fn.Origin().Pkg
	}
	return p != nil && (p.Pkg.Path() == mod || strings.HasPrefix(p.Pkg.Path(), mod+"/"))
}

// findRoots: functions whose params look like an HTTP handler (gin, fiber, net/http), or a name match.
func findRoots(cg *callgraph.Graph, mod, match string) []*ssa.Function {
	var out []*ssa.Function
	for fn := range cg.Nodes {
		if fn == nil || fn.Pkg == nil || !inModule(fn, mod) || excluded(fn.Pkg.Pkg.Path()) {
			continue
		}
		if match != "" {
			if strings.Contains(fn.String(), match) {
				out = append(out, fn)
			}
		} else if isHandler(fn.Signature) {
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func isHandler(sig *types.Signature) bool {
	ps := sig.Params()
	for i := 0; i < ps.Len(); i++ {
		switch types.TypeString(ps.At(i).Type(), nil) {
		case "*github.com/gin-gonic/gin.Context", "*github.com/gofiber/fiber/v2.Ctx", "net/http.ResponseWriter":
			return true
		}
	}
	return false
}

type tracer struct {
	cg    *callgraph.Graph
	mod   string
	fset  *token.FileSet
	root  string
	depth int
	quiet bool
	// per-root stats
	dynamic, maxDepth, nodes int
	pkgs                     map[string]bool
}

func (t *tracer) report(fn *ssa.Function) {
	t.dynamic, t.maxDepth, t.nodes, t.pkgs = 0, 0, 0, map[string]bool{}
	if !t.quiet {
		fmt.Printf("\n== %s  (%s)\n", fn, t.pos(fn.Pos()))
	}
	t.walk(fn, 1, map[*ssa.Function]bool{fn: true})
	var ps []string
	for p := range t.pkgs {
		ps = append(ps, strings.TrimPrefix(p, t.mod+"/"))
	}
	sort.Strings(ps)
	fmt.Printf("SUMMARY %s | depth=%d nodes=%d dynamicEdges=%d pkgs=%v\n", fn, t.maxDepth, t.nodes, t.dynamic, ps)
}

func (t *tracer) walk(fn *ssa.Function, d int, onPath map[*ssa.Function]bool) {
	if d > t.depth {
		return
	}
	node := t.cg.Nodes[fn]
	if node == nil {
		return
	}
	// group by site so multi-target dynamic sites are visible
	type site struct {
		pos     token.Pos
		dynamic bool
		targets []*ssa.Function
	}
	bySite := map[ssa.CallInstruction]*site{}
	var order []ssa.CallInstruction
	for _, e := range node.Out {
		if !inModule(e.Callee.Func, t.mod) {
			continue
		}
		s := bySite[e.Site]
		if s == nil {
			s = &site{pos: e.Site.Pos(), dynamic: e.Site.Common().StaticCallee() == nil}
			bySite[e.Site] = s
			order = append(order, e.Site)
		}
		s.targets = append(s.targets, e.Callee.Func)
	}
	sort.Slice(order, func(i, j int) bool { return bySite[order[i]].pos < bySite[order[j]].pos })
	for _, k := range order {
		s := bySite[k]
		kind := "static"
		if s.dynamic {
			kind = fmt.Sprintf("DYNAMIC x%d", len(s.targets))
			t.dynamic++
		}
		for _, callee := range s.targets {
			t.nodes++
			if callee.Pkg != nil {
				t.pkgs[callee.Pkg.Pkg.Path()] = true
			}
			if d > t.maxDepth {
				t.maxDepth = d
			}
			if !t.quiet {
				fmt.Printf("%s└ %s  [%s] @%s\n", strings.Repeat("  ", d), short(callee, t.mod), kind, t.pos(s.pos))
			}
			if onPath[callee] {
				continue
			}
			onPath[callee] = true
			t.walk(callee, d+1, onPath)
			delete(onPath, callee)
		}
	}
}

func short(fn *ssa.Function, mod string) string {
	return strings.ReplaceAll(fn.String(), mod+"/", "")
}

func (t *tracer) pos(p token.Pos) string {
	if !p.IsValid() {
		return "?"
	}
	pp := t.fset.Position(p)
	abs, _ := filepath.Abs(t.root)
	rel, err := filepath.Rel(abs, pp.Filename)
	if err != nil {
		rel = pp.Filename
	}
	return fmt.Sprintf("%s:%d", rel, pp.Line)
}
