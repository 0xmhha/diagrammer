package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/0xmhha/diagrammer/internal/analyze/goast"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/schema"
)

// runGraph is stage 1: source in, code graph out.
//
// Nothing is interpreted here. The graph records structure and the references
// an AST can prove, and stage 2 is where meaning is attributed to it. Keeping
// that line sharp is what lets the machine be held to facts and the model be
// held to reading them.
func runGraph(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("graph", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var (
		out      = flags.String("o", "", "write the graph here instead of stdout")
		tests    = flags.Bool("tests", false, "read _test.go files too")
		maxDepth = flags.Int("max-depth", 0, "stop descending below this depth (0 is unlimited)")
		exclude  = flags.String("exclude", "", "comma-separated root-relative directories to skip")
	)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: diagrammer graph <src> [-o graph.json] [flags]")
		flags.PrintDefaults()
	}
	src, err := parseOperand(flags, args, "source directory")
	if err != nil {
		return err
	}

	g, err := goast.Analyze(context.Background(), src, goast.Options{
		IncludeTests: *tests,
		MaxDepth:     *maxDepth,
		Exclude:      strings.Split(*exclude, ","),
	})
	if err != nil {
		return fmt.Errorf("analyze %s: %w", src, err)
	}

	encoded, err := encodeGraph(g)
	if err != nil {
		return err
	}

	// The graph is checked against its own schema before it is written. An
	// analyzer that drifts from the contract should fail here, where the bug
	// is, rather than at the plugin that reads the file.
	if err := schema.Validate(schema.Graph, encoded); err != nil {
		return fmt.Errorf("the graph does not satisfy the graph schema, which is a defect in the analyzer rather than in the source: %w", err)
	}

	// The summary is reported whichever way the graph leaves, and always on
	// stderr. Writing it only when -o is given would mean a graph piped to a
	// file carried no word of the files that went missing from it, which is
	// exactly the case where nobody is watching.
	reportGraph(stderr, src, g)

	if *out == "" {
		_, err := stdout.Write(encoded)
		return err
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *out, err)
	}
	return nil
}

// encodeGraph renders a graph deterministically.
//
// Byte-identical output across runs is part of the contract, so nothing here
// may depend on map ordering. The graph types hold no maps for that reason,
// and every slice is sorted before it arrives.
func encodeGraph(g *graph.Graph) ([]byte, error) {
	encoded, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode graph: %w", err)
	}
	return append(encoded, '\n'), nil
}

// reportGraph summarises the walk on stderr, so a person running the command
// sees what was read without having to open the graph.
//
// Parse failures are named rather than counted. A count tells someone that
// something is missing; the paths tell them what to go and look at.
func reportGraph(w io.Writer, src string, g *graph.Graph) {
	d := g.Diagnostics
	fmt.Fprintf(w, "%s: %d nodes, %d edges, %d files read\n", src, len(g.Nodes), len(g.Edges), d.FilesParsed)
	if len(d.ParseFailures) == 0 {
		return
	}
	fmt.Fprintf(w, "%d file(s) did not parse and are missing from the graph:\n", len(d.ParseFailures))
	for _, f := range d.ParseFailures {
		if f.Line > 0 {
			fmt.Fprintf(w, "  %s:%d: %s\n", f.Path, f.Line, f.Message)
			continue
		}
		fmt.Fprintf(w, "  %s: %s\n", f.Path, f.Message)
	}
}
