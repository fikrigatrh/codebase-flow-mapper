package pipeline

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/report"
)

var update = flag.Bool("update", false, "rewrite testdata/fixtures/*/expected.json")

const fixtures = "../../testdata/fixtures/"

func encode(t *testing.T, fixture string) []byte {
	t.Helper()
	g, err := Run(context.Background(), Options{Dir: fixtures + fixture, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	g.GeneratedAt, g.Stats.ElapsedMs = "", 0 // the only non-deterministic fields
	var b bytes.Buffer
	if err := graph.Encode(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Goldens are snapshots: each stage is pinned by its own hand-written expectation; this pins the
// merge (dedup, flow attachment, stats, blockers). Review every diff after -update.
func TestGolden(t *testing.T) {
	dirs, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		t.Run(d.Name(), func(t *testing.T) {
			got := encode(t, d.Name())
			path := filepath.Join(fixtures, d.Name(), "expected.json")
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err, "(run go test ./internal/pipeline -update)")
			}
			if !bytes.Equal(got, want) {
				t.Errorf("graph differs from %s; run with -update and review the diff", path)
			}
			if bytes.Contains(got, []byte(os.Getenv("HOME"))) {
				t.Errorf("absolute path leaked into the graph")
			}
		})
	}
}

// The evil fixture tries to run code (go:generate, init, cgo) and to break out of the report's
// script tag; analysis must do neither.
func TestEvil(t *testing.T) {
	cwd, _ := os.Getwd()
	var html bytes.Buffer
	if err := report.Write(&html, encode(t, "evil"), "example.com/evil", ""); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{fixtures + "evil", cwd} {
		for _, marker := range []string{"PWNED", "PWNED_CGO", "INIT_RAN"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				t.Errorf("%s exists in %s: target code ran", marker, dir)
			}
		}
	}
	page := html.String()
	lower := strings.ToLower(page)
	if open, closed := strings.Count(lower, "<script"), strings.Count(lower, "</script"); open != closed {
		t.Errorf("report has %d <script but %d </script: graph text broke out of a tag", open, closed)
	}
	if !strings.Contains(page, "\\u003c/script\\u003e\\u003cimg src=x onerror=alert(1)\\u003e") {
		t.Errorf("the if condition should be present, escaped")
	}
}
