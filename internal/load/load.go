// Package load loads a Go module under the no-execution safety env (ADR-105) and builds SSA.
package load

import (
	"context"
	"fmt"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
)

// SafetyEnv pins go list: no downloads, no toolchain fetch, no cgo, no go.mod rewrite (ADR-105).
var SafetyEnv = []string{"GOFLAGS=-mod=readonly", "GOPROXY=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0"}

// DefaultExcludes match the last element of a package path (PRD FR-02): fakes and mocks must
// never become call targets.
var DefaultExcludes = []string{"*fakes", "*mocks", "mock*", "testutil*"}

const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
	packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps | packages.NeedModule

type Options struct {
	Dir     string
	Exclude []string // extra globs, matched against the module-relative package path and its last element
	Env     []string // base environment (nil = os.Environ()); SafetyEnv is appended and wins
}

type Program struct {
	Root      string // absolute module directory
	Module    string
	GoVersion string // the target module's go directive
	Fset      *token.FileSet
	Pkgs      []*packages.Package // project packages after exclusion, including ill-typed ones (AST is still usable)
	SSA       *ssa.Program
}

func config(ctx context.Context, opts Options, root string) *packages.Config {
	base := opts.Env
	if base == nil {
		base = os.Environ() // go list needs HOME/GOCACHE/PATH; a non-nil Env replaces the whole environment
	}
	return &packages.Config{
		Context: ctx,
		Mode:    loadMode,
		Dir:     root,
		Tests:   false,
		Env:     append(slices.Clone(base), SafetyEnv...),
	}
}

// Load never fails on broken target code: problems become diagnostics. The error is only for
// unusable input (bad dir, not a module, go toolchain missing) or cancellation.
func Load(ctx context.Context, opts Options) (*Program, []graph.Diagnostic, error) {
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve dir: %w", err)
	}
	pkgs, err := packages.Load(config(ctx, opts, root), "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("go list: %w", err)
	}
	p := &Program{Root: root}
	for _, pkg := range pkgs {
		if pkg.Module != nil && pkg.Module.Main {
			p.Module, p.GoVersion = pkg.Module.Path, pkg.Module.GoVersion
			break
		}
	}
	if p.Module == "" {
		return nil, nil, fmt.Errorf("%s is not inside a Go module (no main module found)", root)
	}

	d := diags{root: root, module: p.Module, seen: map[string]bool{}}
	for _, pkg := range pkgs {
		if excluded(p.relPkg(pkg.PkgPath), opts.Exclude) {
			d.add(graph.SevInfo, "PACKAGE_EXCLUDED", "package excluded from analysis: "+pkg.PkgPath, nil, nil)
			continue
		}
		d.pkgErrors(pkg)
		d.wireGen(pkg)
		p.Pkgs = append(p.Pkgs, pkg)
		if p.Fset == nil {
			p.Fset = pkg.Fset
		}
	}
	if !slices.ContainsFunc(p.Pkgs, func(pkg *packages.Package) bool { return pkg.Name == "main" && !pkg.IllTyped }) {
		d.add(graph.SevInfo, "NO_COMPOSITION_ROOT",
			"no well-typed main package loaded: interface calls can only resolve if concrete types are constructed in the analyzed code", nil, nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// Bodies only for project packages: CFM reads call sites from project functions only, and
	// dependency bodies cost ~2x time and ~40% RSS (CFM-302). ponytail: values that flow
	// through a dependency's body (stored in a library struct, handed back by a callback) lose
	// their VTA types and their calls show as unresolved; build those packages too if it bites.
	var spkgs []*ssa.Package
	p.SSA, spkgs = ssautil.AllPackages(p.Pkgs, ssa.InstantiateGenerics)
	for _, sp := range spkgs {
		if sp != nil { // ill-typed packages get no SSA
			sp.Build()
		}
	}
	return p, d.out, nil
}

func (p *Program) relPkg(pkgPath string) string {
	if pkgPath == p.Module {
		return "."
	}
	return strings.TrimPrefix(pkgPath, p.Module+"/")
}

func excluded(rel string, extra []string) bool {
	last := path.Base(rel)
	for _, g := range DefaultExcludes {
		if ok, _ := path.Match(g, last); ok {
			return true
		}
	}
	for _, g := range extra {
		if ok, _ := path.Match(g, rel); ok {
			return true
		}
		if ok, _ := path.Match(g, last); ok {
			return true
		}
	}
	return false
}

type diags struct {
	root, module string
	seen         map[string]bool
	out          []graph.Diagnostic
}

func (d *diags) add(sev graph.Severity, code, msg string, loc *graph.Loc, details map[string]string) {
	key := code + "\x00" + msg
	if d.seen[key] {
		return
	}
	d.seen[key] = true
	d.out = append(d.out, graph.Diagnostic{Severity: sev, Code: code, Message: msg, Loc: loc, Details: details})
}

// go/packages reports any import it could not load as `could not import X (invalid package name: "")`:
// an in-module X is a generated package that isn't checked in; anything else is a dependency
// missing from the module cache (CFM never downloads — ADR-105).
var missingPkgRe = regexp.MustCompile(`could not import (\S+) \(invalid package name: ""\)`)

func (d *diags) pkgErrors(pkg *packages.Package) {
	for _, e := range pkg.Errors {
		loc := d.parsePos(e.Pos)
		msg := strings.ReplaceAll(e.Msg, d.root+string(filepath.Separator), "") // logs/report carry no absolute paths
		if m := missingPkgRe.FindStringSubmatch(e.Msg); m != nil {
			imp := m[1]
			if imp == d.module || strings.HasPrefix(imp, d.module+"/") {
				if loc != nil && path.Base(loc.File) == "wire_gen.go" {
					continue // a stale wire_gen.go importing a deleted package: WIRE_GEN_STALE covers it
				}
				fix := "go generate ./" + strings.TrimPrefix(imp, d.module+"/")
				if path.Base(imp) == "docs" {
					fix = "swag init"
				}
				d.add(graph.SevWarning, "GENERATED_PKG_MISSING",
					"in-module package "+imp+" has no Go files (generated and not checked in?); importers are only partially typed",
					nil, map[string]string{"fix": fix, "package": imp})
			} else {
				d.add(graph.SevWarning, "GO_LOAD_ERROR",
					"dependency "+imp+" is not in the module cache; importers are only partially typed",
					loc, map[string]string{"fix": "go mod download", "package": imp})
			}
			continue
		}
		code := "GO_LOAD_ERROR"
		if e.Kind == packages.TypeError {
			code = "GO_TYPE_ERROR"
			msg += " (package left out of the call graph)"
		}
		d.add(graph.SevWarning, code, msg, loc, nil)
	}
}

// wireGen: a wireinject-tagged file is present but the package has no wire_gen.go (ADR-107), or
// wire_gen.go exists but no longer type-checks because providers changed after it was generated.
func (d *diags) wireGen(pkg *packages.Package) {
	if i := slices.IndexFunc(pkg.GoFiles, func(f string) bool { return filepath.Base(f) == "wire_gen.go" }); i >= 0 {
		for _, e := range pkg.Errors {
			loc := d.parsePos(e.Pos)
			if loc == nil || path.Base(loc.File) != "wire_gen.go" {
				continue
			}
			d.add(graph.SevWarning, "WIRE_GEN_STALE",
				"wire_gen.go does not type-check (generated before providers changed?): its package is left out of the call graph, so interface calls wired by Wire cannot be resolved",
				loc, map[string]string{"fix": d.wireFix(pkg.GoFiles[i])})
			return
		}
		return
	}
	for _, f := range pkg.IgnoredFiles {
		if !isWireInject(f) {
			continue
		}
		fix := d.wireFix(f)
		loc := graph.LocOf(token.Position{Filename: f, Line: 1, Column: 1}, d.root)
		d.add(graph.SevWarning, "WIRE_GEN_MISSING",
			"wire injector found but wire_gen.go is missing: interface calls wired by Wire cannot be resolved",
			&loc, map[string]string{"fix": fix})
		return
	}
}

// wireFix is the wire command for the package directory holding file.
func (d *diags) wireFix(file string) string {
	rel, _ := filepath.Rel(d.root, filepath.Dir(file))
	if rel == "." {
		return "wire ."
	}
	return "wire ./" + filepath.ToSlash(rel)
}

// isWireInject reports whether a file's //go:build constraint is satisfied by the wireinject tag.
func isWireInject(file string) bool {
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return false
	}
	for _, cg := range f.Comments {
		if cg.Pos() > f.Package {
			break
		}
		for _, c := range cg.List {
			if expr, err := constraint.Parse(c.Text); err == nil {
				return expr.Eval(func(tag string) bool { return tag == "wireinject" })
			}
		}
	}
	return false
}

// parsePos turns go list's "file:line:col" (or "file:line") into a Loc; "" or "-" means none.
func (d *diags) parsePos(pos string) *graph.Loc {
	if pos == "" || pos == "-" {
		return nil
	}
	parts := strings.Split(pos, ":")
	tp := token.Position{Filename: parts[0]}
	if len(parts) >= 2 {
		tp.Line, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		tp.Column, _ = strconv.Atoi(parts[2])
	}
	loc := graph.LocOf(tp, d.root)
	if filepath.IsAbs(loc.File) {
		return nil // outside the module (e.g. module cache): don't leak the path
	}
	return &loc
}
