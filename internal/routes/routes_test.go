package routes

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/callgraph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
)

const fixtures = "../../testdata/fixtures/"

// render prints a Result in the routes.expected.txt format.
func render(r *Result, module string) (endpoints, diags []string) {
	short := func(id graph.NodeID) string {
		s := strings.TrimPrefix(string(id), "fn:")
		return strings.Replace(s, module, "~", 1)
	}
	label := func(e graph.Edge) string {
		if e.Method == graph.MethodWrapperHeuristic {
			return short(e.Target) + "(h)"
		}
		return short(e.Target)
	}
	for _, n := range r.Nodes {
		if n.Kind != graph.KindEndpoint {
			continue
		}
		var handler string
		var mws []graph.Edge
		for _, e := range r.Edges {
			switch {
			case e.Source != n.ID:
			case e.Kind == graph.EdgeRoutesTo:
				handler = label(e)
			case e.Kind == graph.EdgeUsesMiddleware:
				mws = append(mws, e)
			}
		}
		slices.SortFunc(mws, func(a, b graph.Edge) int { return *a.Order - *b.Order })
		var parts []string
		for _, e := range mws {
			parts = append(parts, label(e))
		}
		endpoints = append(endpoints, strings.TrimSpace(n.Method+" "+n.Path+" -> "+handler+" | "+strings.Join(parts, ", ")))
	}
	for _, d := range r.Diags {
		diags = append(diags, "diag: "+d.Code)
	}
	slices.Sort(endpoints)
	slices.Sort(diags)
	return endpoints, diags
}

func readExpected(t *testing.T, fixture string) (endpoints, diags []string) {
	raw, err := os.ReadFile(fixtures + fixture + "/routes.expected.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		switch line = strings.TrimSpace(line); {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "diag: "):
			diags = append(diags, line)
		default:
			endpoints = append(endpoints, line)
		}
	}
	slices.Sort(endpoints)
	slices.Sort(diags)
	return endpoints, diags
}

func analyze(t *testing.T, fixture string) (*Result, *load.Program) {
	t.Helper()
	p, _, err := load.Load(context.Background(), load.Options{Dir: fixtures + fixture})
	if err != nil {
		t.Fatal(err)
	}
	return Build(p, callgraph.Build(p)), p
}

// TestMatchesHandWrittenExpectation compares against routes.expected.txt, written by hand before
// this package ran on the fixture.
func TestMatchesHandWrittenExpectation(t *testing.T) {
	for _, fixture := range []string{"gin-basic", "gin-split-routes", "nethttp", "dynamic", "di-interface", "fiber", "missing-dep"} {
		t.Run(fixture, func(t *testing.T) {
			r, p := analyze(t, fixture)
			gotE, gotD := render(r, p.Module)
			wantE, wantD := readExpected(t, fixture)
			if !slices.Equal(gotE, wantE) {
				t.Errorf("endpoints:\n got:\n  %s\nwant:\n  %s", strings.Join(gotE, "\n  "), strings.Join(wantE, "\n  "))
			}
			if !slices.Equal(gotD, wantD) {
				t.Errorf("diagnostics: got %q want %q\n%+v", gotD, wantD, r.Diags)
			}
		})
	}
}

func TestGinJoin(t *testing.T) {
	cases := [][3]string{{"/", "/ping", "/ping"}, {"/api", "v1", "/api/v1"}, {"/api", "/x/", "/api/x/"}, {"/api", "", "/api"}, {"?", "/x", "?/x"}}
	for _, c := range cases {
		if got := ginJoin(c[0], c[1]); got != c[2] {
			t.Errorf("ginJoin(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestNetHTTPPattern(t *testing.T) {
	cases := map[string]string{"GET /x": "GET /x", "/x": "ANY /x", "POST \t /a/{id}": "POST /a/{id}",
		"GET example.com/x": "GET example.com/x", "example.com/x": "ANY example.com/x", "": "ANY "}
	for in, want := range cases {
		ms, p := netHTTPPattern(in)
		if got := ms[0] + " " + p; got != want {
			t.Errorf("netHTTPPattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFiberJoin(t *testing.T) {
	cases := [][3]string{{"", "/x", "/x"}, {"/api", "v1", "/api/v1"}, {"/api/", "/x", "/api/x"}, {"/api", "", "/api"}, {"", "", "/"}, {"?", "/x", "?/x"}}
	for _, c := range cases {
		if got := fiberJoin(c[0], c[1]); got != c[2] {
			t.Errorf("fiberJoin(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}
