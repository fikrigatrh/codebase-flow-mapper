package load

import (
	"context"
	"slices"
	"testing"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
)

const fixtures = "../../testdata/fixtures/"

func codes(ds []graph.Diagnostic) map[string][]graph.Diagnostic {
	m := map[string][]graph.Diagnostic{}
	for _, d := range ds {
		m[d.Code] = append(m[d.Code], d)
	}
	return m
}

func TestConfigAppliesSafetyEnvLast(t *testing.T) {
	cfg := config(context.Background(), Options{Env: []string{"GOPROXY=https://evil.example", "CGO_ENABLED=1"}}, "/x")
	tail := cfg.Env[len(cfg.Env)-len(SafetyEnv):]
	if !slices.Equal(tail, SafetyEnv) {
		t.Fatalf("safety env must come last so it wins: %v", cfg.Env)
	}
	if cfg.Tests {
		t.Fatal("tests must not be loaded")
	}
}

func TestExcluded(t *testing.T) {
	cases := []struct {
		rel   string
		extra []string
		want  bool
	}{
		{"internal/repo/repofakes", nil, true},
		{"internal/mocks", nil, true},
		{"internal/mockdb", nil, true},
		{"pkg/testutil", nil, true},
		{"internal/repo", nil, false},
		{"internal/legacy/old", []string{"internal/legacy/*"}, true},
		{"internal/gen", []string{"gen"}, true},
		{"internal/general", []string{"gen"}, false},
	}
	for _, c := range cases {
		if got := excluded(c.rel, c.extra); got != c.want {
			t.Errorf("excluded(%q, %v) = %v, want %v", c.rel, c.extra, got, c.want)
		}
	}
}

func TestLoadDIInterface(t *testing.T) {
	p, ds, err := Load(context.Background(), Options{Dir: fixtures + "di-interface"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Module != "example.com/fixturedi" || p.SSA == nil {
		t.Fatalf("module=%q ssa=%v", p.Module, p.SSA != nil)
	}
	c := codes(ds)
	if len(c["PACKAGE_EXCLUDED"]) != 1 || len(c["NO_COMPOSITION_ROOT"]) != 0 || len(c["GO_LOAD_ERROR"]) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", ds)
	}
	for _, pkg := range p.Pkgs {
		if pkg.PkgPath == "example.com/fixturedi/internal/repo/repofakes" {
			t.Fatal("fakes package must not be loaded into the program")
		}
	}
}

func TestLoadWireMissing(t *testing.T) {
	_, ds, err := Load(context.Background(), Options{Dir: fixtures + "wire-missing"})
	if err != nil {
		t.Fatal(err)
	}
	c := codes(ds)
	w := c["WIRE_GEN_MISSING"]
	if len(w) != 1 || w[0].Details["fix"] != "wire ./cmd/api" || w[0].Loc == nil || w[0].Loc.File != "cmd/api/wire.go" {
		t.Fatalf("WIRE_GEN_MISSING: %+v (all: %+v)", w, ds)
	}
	g := c["GENERATED_PKG_MISSING"]
	if len(g) != 1 || g[0].Details["fix"] != "swag init" {
		t.Fatalf("GENERATED_PKG_MISSING: %+v (all: %+v)", g, ds)
	}
	if len(c["NO_COMPOSITION_ROOT"]) != 1 {
		t.Fatalf("main is ill-typed (InitApp undefined), so no composition root: %+v", ds)
	}
}

func TestLoadMissingDepIsPartialNotFatal(t *testing.T) {
	p, ds, err := Load(context.Background(), Options{Dir: fixtures + "missing-dep"})
	if err != nil {
		t.Fatal(err)
	}
	if len(codes(ds)["GO_LOAD_ERROR"]) == 0 {
		t.Fatalf("want GO_LOAD_ERROR, got %+v", ds)
	}
	if len(p.Pkgs) != 1 || len(p.Pkgs[0].Syntax) != 2 {
		t.Fatal("the package's AST must still be available for routes/flow")
	}
	for _, d := range ds {
		if d.Loc != nil && d.Loc.File[0] == '/' {
			t.Fatalf("absolute path leaked: %+v", d)
		}
	}
}

func TestLoadNotAModule(t *testing.T) {
	if _, _, err := Load(context.Background(), Options{Dir: t.TempDir()}); err == nil {
		t.Fatal("want error outside a module")
	}
}

func TestLoadCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Load(ctx, Options{Dir: fixtures + "di-interface"}); err == nil {
		t.Fatal("want error on cancelled context")
	}
}
