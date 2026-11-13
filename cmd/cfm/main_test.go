package main

import (
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
