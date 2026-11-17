package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const fixtures = "../../testdata/fixtures/"

func TestExitCodes(t *testing.T) {
	out := t.TempDir()
	html, js := filepath.Join(out, "r.html"), filepath.Join(out, "g.json")
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"ok, flags after dir", []string{"analyze", fixtures + "gin-basic", "-o", html, "--json", js}, exitOK},
		{"ok, flags before dir", []string{"analyze", "-o", html, "--exclude", "x*", "--editor", "zed", fixtures + "flow"}, exitOK},
		{"type error degrades", []string{"analyze", fixtures + "wire-missing", "-o", html}, exitDegraded},
		{"no -o", []string{"analyze", fixtures + "flow"}, exitUsage},
		{"two dirs", []string{"analyze", "a", "b", "-o", html}, exitUsage},
		{"bad editor", []string{"analyze", fixtures + "flow", "-o", html, "--editor", "vim"}, exitUsage},
		{"not a module", []string{"analyze", out, "-o", html}, exitUsage},
		{"unknown command", []string{"run"}, exitUsage},
	}
	for _, c := range cases {
		if got := run(context.Background(), c.args, io.Discard, io.Discard); got != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, got, c.want)
		}
	}
	if _, err := os.Stat(js); err != nil {
		t.Errorf("--json not written: %v", err)
	}
}

// A graph written by `analyze --json` renders to the same report, and foreign input that breaks
// the schema is refused as a usage error (ADR-108).
func TestRender(t *testing.T) {
	dir := t.TempDir()
	p := func(name string) string { return filepath.Join(dir, name) }
	if got := run(context.Background(), []string{"analyze", fixtures + "body", "-o", p("a.html"), "--json", p("g.json")}, io.Discard, io.Discard); got != exitOK {
		t.Fatalf("analyze: exit %d", got)
	}
	bad := map[string]string{
		"old.json":     `{"schemaVersion":"1.0.0"}`,
		"unknown.json": `{"schemaVersion":"2.1.0","language":"ts"}`,
		"invalid.json": `{"schemaVersion":"2.1.0","nodes":[{"id":"x","kind":"bogus"}]}`,
		"broken.json":  `{`,
	}
	for name, body := range bad {
		if err := os.WriteFile(p(name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"render", p("g.json"), "-o", p("r.html")}, exitOK},
		{[]string{"render", "-o", p("x.html"), p("missing.json")}, exitUsage},
		{[]string{"render", p("g.json")}, exitUsage},
		{[]string{"render", p("old.json"), "-o", p("x.html")}, exitUsage},
		{[]string{"render", p("unknown.json"), "-o", p("x.html")}, exitUsage},
		{[]string{"render", p("invalid.json"), "-o", p("x.html")}, exitUsage},
		{[]string{"render", p("broken.json"), "-o", p("x.html")}, exitUsage},
	}
	for _, c := range cases {
		if got := run(context.Background(), c.args, io.Discard, io.Discard); got != c.want {
			t.Errorf("%v: exit %d, want %d", c.args[1:], got, c.want)
		}
	}
	a, _ := os.ReadFile(p("a.html"))
	r, _ := os.ReadFile(p("r.html"))
	if len(a) == 0 || !bytes.Equal(a, r) {
		t.Error("render of analyze --json differs from the analyze report")
	}
	if _, err := os.Stat(p("x.html")); err == nil {
		t.Error("a refused graph still wrote a report")
	}
}
