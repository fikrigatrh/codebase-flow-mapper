// Command cfm writes a static HTML map of a Go module's HTTP flows. See README.md.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/pipeline"
	"github.com/fikrigatrh/codebase-flow-mapper/internal/report"
)

const version = "0.2.0"

const (
	exitOK       = 0
	exitDegraded = 1 // analysis had errors, report written
	exitBug      = 2 // CFM bug / graph validation failure, nothing written
	exitUsage    = 3
)

const usage = `usage:
  cfm analyze <dir> -o report.html [--json graph.json] [--exclude glob]... [--editor vscode|idea|zed]
  cfm render <graph.json> -o report.html
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
	case len(args) > 0 && args[0] == "render":
		in, out, err := parseRender(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "cfm: %v\n%s\n", err, usage)
			return exitUsage
		}
		return render(in, out, stderr)
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

	dirs, err := parseAnywhere(fs, args)
	switch {
	case err != nil:
		return a, err
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

// parseAnywhere parses flags placed before, between or after the positional arguments.
func parseAnywhere(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos, rest = append(pos, fs.Arg(0)), fs.Args()[1:]
	}
}

func parseRender(args []string) (in, out string, err error) {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&out, "o", "", "HTML report path")
	files, err := parseAnywhere(fs, args)
	switch {
	case err != nil:
		return "", "", err
	case len(files) != 1:
		return "", "", errors.New("render takes exactly one <graph.json>")
	case out == "":
		return "", "", errors.New("-o report.html is required")
	}
	return files[0], out, nil
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
	if code, err := writeReport(js.Bytes(), g.Module, a.editor, a.out); err != nil {
		fmt.Fprintln(stderr, "cfm:", err)
		return code
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

// writeReport returns the exit code for its error: a report that can't be built is a CFM bug,
// a file that can't be written is the user's path.
func writeReport(js []byte, module, editor, out string) (int, error) {
	var html bytes.Buffer
	if err := report.Write(&html, js, module, editor); err != nil {
		return exitBug, err
	}
	if err := os.WriteFile(out, html.Bytes(), 0o644); err != nil {
		return exitUsage, err
	}
	return exitOK, nil
}

// render wraps a graph written by any analyzer (ADR-108) in the viewer. The graph is foreign
// input, so a schema violation is a usage error, not a CFM bug.
func render(in, out string, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, "cfm:", err); return exitUsage }
	data, err := os.ReadFile(in)
	if err != nil {
		return fail(err)
	}
	var g graph.Graph
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields() // a field the viewer doesn't know is schema drift, not something to drop silently
	if err := dec.Decode(&g); err != nil {
		return fail(fmt.Errorf("%s: %w", in, err))
	}
	if major(g.SchemaVersion) != major(graph.SchemaVersion) {
		return fail(fmt.Errorf("%s: schemaVersion %q, this cfm reads %s.x", in, g.SchemaVersion, major(graph.SchemaVersion)))
	}
	graph.Normalize(&g)
	if err := graph.Validate(&g); err != nil {
		return fail(fmt.Errorf("%s does not follow GRAPH_SCHEMA.md: %w", in, err))
	}
	var js bytes.Buffer
	if err := graph.Encode(&js, &g); err != nil {
		return fail(err)
	}
	// ponytail: no --editor; links need the analyzer's editorRoot, add the flag with the first analyzer that sets it.
	code, err := writeReport(js.Bytes(), g.Module, "", out)
	if err != nil {
		fmt.Fprintln(stderr, "cfm:", err)
	}
	return code
}

func major(v string) string { m, _, _ := strings.Cut(v, "."); return m }
