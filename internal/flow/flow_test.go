package flow

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/callgraph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
)

const fixtures = "../../testdata/fixtures/"

func build(t *testing.T, fixture string) (*callgraph.Result, map[graph.NodeID][]graph.Step, []graph.Diagnostic) {
	t.Helper()
	p, _, err := load.Load(context.Background(), load.Options{Dir: fixtures + fixture})
	if err != nil {
		t.Fatal(err)
	}
	cg := callgraph.Build(p)
	steps, diags := Build(p, cg)
	return cg, steps, diags
}

// render prints a step tree in the flow.expected.txt format.
func render(steps []graph.Step, indent string, out *[]string) {
	for _, s := range steps {
		line := fmt.Sprintf("%s%s L%d", indent, s.K, s.Line)
		if text := s.Cond + s.Tag + s.Head; text != "" {
			line += " " + text
		}
		*out = append(*out, line)
		if s.Then != nil || s.Else != nil {
			*out = append(*out, indent+"  then:")
			render(s.Then, indent+"    ", out)
			if s.Else != nil {
				*out = append(*out, indent+"  else:")
				render(s.Else, indent+"    ", out)
			}
		}
		render(s.Body, indent+"  ", out)
		for _, c := range s.Cases {
			*out = append(*out, indent+"  case "+c.Label+":")
			render(c.Body, indent+"    ", out)
		}
	}
}

// TestMatchesHandWrittenExpectation compares against flow.expected.txt, written by hand before
// this package existed.
func TestMatchesHandWrittenExpectation(t *testing.T) {
	raw, err := os.ReadFile(fixtures + "flow/flow.expected.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	var cur string
	for _, l := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "#"):
		case strings.HasPrefix(l, "== "):
			cur = strings.TrimPrefix(l, "== ")
			want[cur] = []string{}
		default:
			want[cur] = append(want[cur], l)
		}
	}
	_, flows, diags := build(t, "flow")
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %+v", diags)
	}
	got := map[string][]string{}
	for id, steps := range flows {
		var lines []string
		render(steps, "", &lines)
		got[string(id)] = lines
	}
	for id, w := range want {
		if !slices.Equal(got[id], w) {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", id, strings.Join(got[id], "\n"), strings.Join(w, "\n"))
		}
	}
	for id := range got {
		if want[id] == nil {
			t.Errorf("unexpected flow for %s", id)
		}
	}
}

// TestSitesMatchCallEdges is GRAPH_SCHEMA §7.3 on every fixture: the call/go/defer steps of a
// function and the siteIds of its calls edges are the same set.
func TestSitesMatchCallEdges(t *testing.T) {
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			cg, flows, diags := build(t, e.Name())
			if len(diags) > 0 {
				t.Fatalf("diagnostics: %+v", diags)
			}
			edges := map[graph.NodeID]map[string]bool{}
			for _, ed := range cg.Edges {
				if edges[ed.Source] == nil {
					edges[ed.Source] = map[string]bool{}
				}
				edges[ed.Source][ed.SiteID] = true
			}
			for id := range cg.Funcs {
				steps := map[string]bool{}
				graph.WalkSites(flows[id], func(s string) { steps[s] = true })
				for s := range steps {
					if !edges[id][s] {
						t.Errorf("%s: flow site %s has no calls edge", id, s)
					}
				}
				for s := range edges[id] {
					if !steps[s] {
						t.Errorf("%s: calls edge site %s has no flow step", id, s)
					}
				}
			}
		})
	}
}

func TestTrunc(t *testing.T) {
	long := strings.Repeat("é", 121)
	if got := trunc(long); got != strings.Repeat("é", 120)+"…" {
		t.Errorf("trunc counts runes, got %d runes", len([]rune(got)))
	}
	if got := trunc("x < y"); got != "x < y" {
		t.Errorf("short text changed: %q", got)
	}
}
