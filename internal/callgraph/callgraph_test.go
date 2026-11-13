package callgraph

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/load"
)

const fixtures = "../../testdata/fixtures/"

func build(t *testing.T, fixture string) *Result {
	t.Helper()
	p, _, err := load.Load(context.Background(), load.Options{Dir: fixtures + fixture})
	if err != nil {
		t.Fatal(err)
	}
	return Build(p)
}

// TestMatchesHandWrittenExpectation compares against calls.expected.json files written by hand
// before this package ran on the fixture.
func TestMatchesHandWrittenExpectation(t *testing.T) {
	for _, fixture := range []string{"di-interface", "dynamic"} {
		t.Run(fixture, func(t *testing.T) { checkExpected(t, fixture) })
	}
}

func checkExpected(t *testing.T, fixture string) {
	raw, err := os.ReadFile(fixtures + fixture + "/calls.expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Functions  []string
		Externals  map[string]string
		Unresolved map[string]string
		Calls      [][4]string
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	r := build(t, fixture)

	var fns []string
	externals, unresolved := map[string]string{}, map[string]string{}
	for _, n := range r.Nodes {
		switch n.Kind {
		case graph.KindFunction:
			fns = append(fns, string(n.ID))
		case graph.KindExternal:
			externals[string(n.ID)] = n.Category
		case graph.KindUnresolved:
			unresolved[string(n.ID)] = n.Reason
		}
	}
	slices.Sort(fns)
	if !slices.Equal(fns, want.Functions) {
		t.Errorf("functions:\n got %q\nwant %q", fns, want.Functions)
	}
	if !maps.Equal(externals, want.Externals) {
		t.Errorf("externals:\n got %v\nwant %v", externals, want.Externals)
	}
	if !maps.Equal(unresolved, want.Unresolved) && (len(unresolved) > 0 || len(want.Unresolved) > 0) {
		t.Errorf("unresolved:\n got %v\nwant %v", unresolved, want.Unresolved)
	}

	var calls [][4]string
	for _, e := range r.Edges {
		calls = append(calls, [4]string{e.SiteID, string(e.Target), string(e.Confidence), string(e.Method)})
		if !strings.HasPrefix(e.SiteID, e.Site.File+":") {
			t.Errorf("siteId %q does not match site %+v", e.SiteID, e.Site)
		}
		if (e.Confidence == graph.ConfLow) != (e.Alternatives >= 2) {
			t.Errorf("alternatives must be set exactly on low edges: %+v", e)
		}
	}
	slices.SortFunc(calls, func(a, b [4]string) int { return strings.Compare(a[0]+a[1], b[0]+b[1]) })
	if !slices.Equal(calls, want.Calls) {
		t.Errorf("calls:\n got %q\nwant %q", calls, want.Calls)
	}
	if len(r.Diags) != len(want.Unresolved) {
		t.Errorf("want one CALL_UNRESOLVED per unresolved node, got %+v", r.Diags)
	}
	for _, n := range r.Nodes {
		if strings.Contains(string(n.ID), "repofakes") || strings.Contains(string(n.ID), "memRepo") {
			t.Errorf("fake or unwired implementation became a node: %s", n.ID)
		}
	}
}

// Synthetic SSA functions must never leak into node IDs.
func TestNoSyntheticNodes(t *testing.T) {
	for _, n := range build(t, "dynamic").Nodes {
		if strings.Contains(string(n.ID), "$bound") || strings.Contains(string(n.ID), "$thunk") || strings.Contains(string(n.ID), "[") {
			t.Errorf("synthetic or instantiated function leaked: %s", n.ID)
		}
	}
}

// The function holding an unresolved site is partially resolved; the rest stay resolved.
func TestPartialStatus(t *testing.T) {
	for _, n := range build(t, "dynamic").Nodes {
		partial := strings.HasSuffix(string(n.ID), "notify.Log") || strings.HasSuffix(string(n.ID), "notify.Apply")
		if n.Kind == graph.KindFunction && (n.Status == graph.StatusPartiallyResolved) != partial {
			t.Errorf("%s: status %s", n.ID, n.Status)
		}
	}
}

func TestCategory(t *testing.T) {
	cases := map[string]string{
		"database/sql.(*DB).ExecContext":             "db",
		"gorm.io/gorm.(*DB).Create":                  "db",
		"github.com/jackc/pgx/v5.(*Conn).Exec":       "db",
		"github.com/redis/go-redis/v9.(*Client).Get": "cache",
		"net/http.(*Client).Do":                      "http_client",
		"net/http.Get":                               "http_client",
		"net/http.(*Request).Context":                "other",
		"log.Printf":                                 "log",
		"log/slog.Info":                              "log",
		"logfoo.Bar":                                 "other",
		"github.com/redisfoo/x.F":                    "other",
	}
	for q, want := range cases {
		if got := category(q); got != want {
			t.Errorf("category(%q) = %q, want %q", q, got, want)
		}
	}
}
