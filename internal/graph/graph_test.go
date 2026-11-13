package graph

import (
	"bytes"
	"go/token"
	"strings"
	"testing"
)

func intp(i int) *int { return &i }

// validGraph covers every node and edge kind with consistent flow sites.
func validGraph() *Graph {
	handler := FuncID(Sym{Pkg: "m/handler", Recv: "*H", Name: "Create"})
	svc := FuncID(Sym{Pkg: "m/svc", Recv: "*s", Name: "Create"})
	mw := FuncID(Sym{Pkg: "m/mw", Name: "Auth"})
	ext := ExternalID(Sym{Pkg: "database/sql", Recv: "*DB", Name: "ExecContext"})
	reg := Loc{File: "router.go", Line: 10, Col: 2}
	ep := EndpointID("gin", "POST", "/tx", reg)
	site1 := Loc{File: "handler/h.go", Line: 5, Col: 20}
	site2 := Loc{File: "svc/s.go", Line: 9, Col: 30}
	unres := UnresolvedID(Loc{File: "svc/s.go", Line: 12, Col: 7})
	site3 := Loc{File: "svc/s.go", Line: 12, Col: 7}
	return &Graph{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			{ID: svc, Kind: KindFunction, Name: "(*s).Create", Status: StatusResolved, Flow: []Step{
				{K: StepIf, Line: 8, Cond: "err != nil", Then: []Step{{K: StepReturn, Line: 8}},
					Else: []Step{{K: StepCall, Site: SiteKey(site2), Line: 9}}},
				{K: StepDefer, Site: SiteKey(site3), Line: 12},
			}},
			{ID: ep, Kind: KindEndpoint, Name: "POST /tx", Status: StatusResolved, Loc: &reg},
			{ID: handler, Kind: KindFunction, Name: "(*H).Create", Status: StatusResolved,
				Flow: []Step{{K: StepCall, Site: SiteKey(site1), Line: 5}}},
			{ID: mw, Kind: KindFunction, Name: "Auth", Status: StatusResolved},
			{ID: ext, Kind: KindExternal, Name: "(*DB).ExecContext", Status: StatusResolved, Category: "db"},
			{ID: unres, Kind: KindUnresolved, Name: "f()", Status: StatusUnresolved, Reason: "func_value_unknown"},
		},
		Edges: []Edge{
			{Source: svc, Target: ext, Kind: EdgeCalls, Confidence: ConfHigh, Method: MethodStatic, Site: site2, SiteID: SiteKey(site2)},
			{Source: ep, Target: handler, Kind: EdgeRoutesTo, Confidence: ConfHigh, Method: MethodFrameworkRegistration, Site: reg},
			{Source: handler, Target: svc, Kind: EdgeCalls, Confidence: ConfMedium, Method: MethodVTA, Site: site1, SiteID: SiteKey(site1)},
			{Source: ep, Target: mw, Kind: EdgeUsesMiddleware, Confidence: ConfHigh, Method: MethodFrameworkRegistration, Site: reg, Order: intp(0)},
			{Source: svc, Target: unres, Kind: EdgeCalls, Confidence: ConfUnknown, Method: MethodUnresolved, Site: site3, SiteID: SiteKey(site3)},
		},
		Diagnostics: []Diagnostic{{Severity: SevWarning, Code: "WIRE_GEN_MISSING", Message: "m", Details: map[string]string{"fix": "wire ./..."}}},
		Stats:       Stats{Blockers: []int{0}},
	}
}

func TestIDs(t *testing.T) {
	cases := map[NodeID]NodeID{
		FuncID(Sym{Pkg: "a/b", Name: "F"}):                            "fn:a/b.F",
		FuncID(Sym{Pkg: "a/b", Recv: "*T", Name: "M"}):                "fn:a/b.(*T).M",
		LiteralID(FuncID(Sym{Pkg: "a/b", Name: "F"}), 2):              "fn:a/b.F$2",
		ExternalID(Sym{Pkg: "net/http", Recv: "*Client", Name: "Do"}): "external:net/http.(*Client).Do",
		EndpointID("gin", "GET", "/v1/x", Loc{File: "r.go", Line: 4}): "endpoint:gin:GET:/v1/x:r.go:4",
		UnresolvedID(Loc{File: "s.go", Line: 44, Col: 12}):            "unresolved:s.go:44:12",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if k := SiteKey(Loc{File: "h.go", Line: 52, Col: 31}); k != "h.go:52:31" {
		t.Errorf("SiteKey = %q", k)
	}
}

func TestLocOf(t *testing.T) {
	in := LocOf(token.Position{Filename: "/root/mod/internal/a.go", Line: 3, Column: 4}, "/root/mod")
	if in != (Loc{File: "internal/a.go", Line: 3, Col: 4}) {
		t.Errorf("inside root: %+v", in)
	}
	out := LocOf(token.Position{Filename: "/elsewhere/b.go", Line: 1}, "/root/mod")
	if out.File != "/elsewhere/b.go" {
		t.Errorf("outside root must stay absolute so Validate catches it: %+v", out)
	}
}

func TestValidateAcceptsValidGraph(t *testing.T) {
	g := validGraph()
	Normalize(g)
	if err := Validate(g); err != nil {
		t.Fatal(err)
	}
}

// Each mutation breaks exactly one GRAPH_SCHEMA §7 rule; the error must name it.
func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*Graph)
	}{
		{"dangling edge", "dangling", func(g *Graph) { g.Edges[0].Target = "fn:nope" }},
		{"wrong edge shape", "not allowed", func(g *Graph) {
			for i := range g.Edges {
				if g.Edges[i].Kind == EdgeRoutesTo {
					g.Edges[i].Kind = EdgeCalls
					g.Edges[i].SiteID = "router.go:10:2"
				}
			}
		}},
		{"invalid enum", "invalid confidence", func(g *Graph) { g.Edges[0].Confidence = "87%" }},
		{"calls without siteId", "missing siteId", func(g *Graph) {
			for i := range g.Edges {
				if g.Edges[i].Kind == EdgeCalls {
					g.Edges[i].SiteID = ""
				}
			}
		}},
		{"middleware without order", "missing order", func(g *Graph) {
			for i := range g.Edges {
				g.Edges[i].Order = nil
			}
		}},
		{"flow site without edge", "has no calls edge", func(g *Graph) {
			for i := range g.Nodes {
				if g.Nodes[i].Kind == KindFunction && len(g.Nodes[i].Flow) == 1 {
					g.Nodes[i].Flow = append(g.Nodes[i].Flow, Step{K: StepCall, Site: "handler/h.go:6:1", Line: 6})
				}
			}
		}},
		{"edge without flow step", "has no flow step", func(g *Graph) {
			for i := range g.Nodes {
				if len(g.Nodes[i].Flow) == 1 {
					g.Nodes[i].Flow = nil
				}
			}
		}},
		{"duplicate node", "duplicate node", func(g *Graph) { g.Nodes = append(g.Nodes, g.Nodes[0]) }},
		{"unsorted nodes", "nodes not sorted", func(g *Graph) { g.Nodes[0], g.Nodes[1] = g.Nodes[1], g.Nodes[0] }},
		{"duplicate edge", "duplicate", func(g *Graph) { g.Edges = append(g.Edges[:1], g.Edges...) }},
		{"absolute path", "non-relative", func(g *Graph) { g.Nodes[0].Loc = &Loc{File: "/Users/x/a.go", Line: 1} }},
		{"escaping path", "non-relative", func(g *Graph) { g.Diagnostics[0].Loc = &Loc{File: "../other/a.go", Line: 1} }},
		{"blocker out of range", "out of range", func(g *Graph) { g.Stats.Blockers = []int{5} }},
		{"bad test op", "invalid test op", func(g *Graph) { ifStep(g).Test = &Test{Op: "matches"} }},
		{"compare without value", "missing its ref or value", func(g *Graph) {
			ifStep(g).Test = &Test{Op: OpEq, Ref: &Ref{Type: "m/dto.Req", Path: []string{"currency"}}}
		}},
		{"not with two args", "has 2 args", func(g *Graph) {
			ifStep(g).Test = &Test{Op: OpNot, Args: []Test{{Op: OpUnknown}, {Op: OpUnknown}}}
		}},
		{"bad field kind", "invalid kind \"date\"", func(g *Graph) {
			for i := range g.Nodes {
				if g.Nodes[i].Kind == KindFunction {
					g.Nodes[i].Body = &Body{Type: "m/dto.Req", Fields: []Field{{Name: "at", Kind: "date"}}}
				}
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := validGraph()
			Normalize(g)
			c.mutate(g)
			err := Validate(g)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

// ifStep is the if step in validGraph's service function.
func ifStep(g *Graph) *Step {
	for i := range g.Nodes {
		for j := range g.Nodes[i].Flow {
			if g.Nodes[i].Flow[j].K == StepIf {
				return &g.Nodes[i].Flow[j]
			}
		}
	}
	panic("validGraph has no if step")
}

func TestAbsolutePathAllowedWithEditorRoot(t *testing.T) {
	g := validGraph()
	root := "/Users/x/mod"
	g.EditorRoot = &root
	g.Nodes[0].Loc = &Loc{File: "/Users/x/mod/a.go", Line: 1}
	Normalize(g)
	if err := Validate(g); err != nil {
		t.Fatal(err)
	}
}

func TestEncodeIsDeterministicAndScriptSafe(t *testing.T) {
	a, b := validGraph(), validGraph()
	// reverse input order of b: output must not depend on it
	for i, j := 0, len(b.Nodes)-1; i < j; i, j = i+1, j-1 {
		b.Nodes[i], b.Nodes[j] = b.Nodes[j], b.Nodes[i]
	}
	for i, j := 0, len(b.Edges)-1; i < j; i, j = i+1, j-1 {
		b.Edges[i], b.Edges[j] = b.Edges[j], b.Edges[i]
	}
	a.Nodes[0].Flow[0].Cond = `s == "</script><img onerror=x>"`
	b.Nodes[len(b.Nodes)-1].Flow[0].Cond = a.Nodes[0].Flow[0].Cond

	var ba, bb bytes.Buffer
	if err := Encode(&ba, a); err != nil {
		t.Fatal(err)
	}
	if err := Encode(&bb, b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ba.Bytes(), bb.Bytes()) {
		t.Fatal("encoding depends on input order")
	}
	if bytes.Contains(ba.Bytes(), []byte("</script>")) {
		t.Fatal("raw </script> in output")
	}
}

func TestEncodeEmptyGraphHasArraysNotNull(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, &Graph{SchemaVersion: SchemaVersion}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"nodes": []`, `"edges": []`, `"diagnostics": []`, `"blockers": []`, `"editorRoot": null`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestEncodeRefusesInvalidGraph(t *testing.T) {
	g := validGraph()
	g.Edges[0].Target = "fn:nope"
	if err := Encode(&bytes.Buffer{}, g); err == nil {
		t.Fatal("Encode wrote an invalid graph")
	}
}
