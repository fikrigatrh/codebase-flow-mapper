// Command cfm writes a static HTML map of a Go module's HTTP flows. See README.md.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/pipeline"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/report"
)

const version = "0.1.0"

const (
	exitOK       = 0
	exitDegraded = 1 // analysis had errors, report written
	exitBug      = 2 // CFM bug / graph validation failure, nothing written
	exitUsage    = 3
)

const usage = `usage:
  cfm analyze <dir> -o report.html [--json graph.json] [--exclude glob]... [--editor vscode|idea|zed]
  cfm version`

var editors = []string{"vscode", "idea", "zed"}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

type analyzeArgs struct {
	dir, out, jsonOut, editor string
	exclude                   []string
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && args[0] == "version":
		fmt.Fprintln(stdout, "cfm", version)
		return exitOK
	case len(args) > 0 && args[0] == "analyze":
		a, err := parseAnalyze(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "cfm: %v\n%s\n", err, usage)
			return exitUsage
		}
		return analyze(ctx, a, stderr)
	}
	fmt.Fprintln(stderr, usage)
	return exitUsage
}

// parseAnalyze accepts flags before or after <dir>.
func parseAnalyze(args []string) (analyzeArgs, error) {
	var a analyzeArgs
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&a.out, "o", "", "HTML report path")
	fs.StringVar(&a.jsonOut, "json", "", "graph JSON path")
	fs.StringVar(&a.editor, "editor", "", "vscode|idea|zed")
	fs.Func("exclude", "package glob (repeatable)", func(s string) error { a.exclude = append(a.exclude, s); return nil })

	var dirs []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return a, err
		}
		if fs.NArg() == 0 {
			break
		}
		dirs, rest = append(dirs, fs.Arg(0)), fs.Args()[1:]
	}
	switch {
	case len(dirs) != 1:
		return a, errors.New("analyze takes exactly one <dir>")
	case a.out == "":
		return a, errors.New("-o report.html is required")
	case a.editor != "" && !slices.Contains(editors, a.editor):
		return a, fmt.Errorf("--editor must be one of %v", editors)
	}
	a.dir = dirs[0]
	return a, nil
}

func analyze(ctx context.Context, a analyzeArgs, stderr io.Writer) int {
	g, err := pipeline.Run(ctx, pipeline.Options{Dir: a.dir, Exclude: a.exclude, Version: version, EditorRoot: a.editor != ""})
	if err != nil {
		fmt.Fprintln(stderr, "cfm:", err)
		return exitUsage
	}
	var js bytes.Buffer
	if err := graph.Encode(&js, g); err != nil {
		fmt.Fprintln(stderr, "cfm:", err)
		return exitBug
	}
	var html bytes.Buffer
	if err := report.Write(&html, js.Bytes(), g.Module, a.editor); err != nil {
		fmt.Fprintln(stderr, "cfm:", err)
		return exitBug
	}
	if err := os.WriteFile(a.out, html.Bytes(), 0o644); err != nil {
		fmt.Fprintln(stderr, "cfm:", err)
		return exitUsage
	}
	if a.jsonOut != "" {
		if err := os.WriteFile(a.jsonOut, js.Bytes(), 0o644); err != nil {
			fmt.Fprintln(stderr, "cfm:", err)
			return exitUsage
		}
	}
	s := g.Stats
	fmt.Fprintf(stderr, "cfm: %d endpoints, %d functions, %d edges, %d unresolved, %d diagnostics in %d ms\n",
		s.Endpoints, s.Functions, s.Edges, s.Unresolved, len(g.Diagnostics), s.ElapsedMs)
	for _, i := range s.Blockers {
		d := g.Diagnostics[i]
		fmt.Fprintf(stderr, "cfm: blocker %s: run %q\n", d.Code, d.Details["fix"])
	}
	if pipeline.HasErrors(g) {
		return exitDegraded
	}
	return exitOK
}
