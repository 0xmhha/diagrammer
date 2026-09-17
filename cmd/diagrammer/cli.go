package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/0xmhha/diagrammer/internal/command"
)

// The CLI is one of two faces on the same set of capabilities. It owns flag
// parsing and where output goes, and nothing else: what an operation does lives
// in internal/command, so the MCP server reaches exactly the same code.
//
// Output goes where its kind belongs. The summary is commentary and goes to
// standard error; the documents are the product and go to standard output when
// no file was named, which is what keeps the command usable in a pipe.

func runGraph(args []string, stdout, stderr io.Writer) error {
	req, err := buildGraphRequest(args, stderr)
	if err != nil {
		return err
	}
	return deliverResult(req, stdout, stderr)
}

// buildGraphRequest turns a command line into a request.
//
// Construction is split from execution so a test can drive the same code the
// command does and check that every field of the request is reachable from the
// command line. A field with no flag would be a capability the MCP server has
// and the CLI does not.
func buildGraphRequest(args []string, stderr io.Writer) (*command.GraphRequest, error) {
	req := &command.GraphRequest{}
	flags := flag.NewFlagSet("graph", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&req.Out, "o", "", "write the graph here instead of standard output")
	flags.BoolVar(&req.IncludeTests, "tests", false, "read _test.go files too")
	flags.IntVar(&req.MaxDepth, "max-depth", 0, "stop descending below this depth (0 is unlimited)")
	exclude := flags.String("exclude", "", "comma-separated source-relative directories to skip")
	flags.Usage = usageFor(stderr, flags, "graph <src> [-o graph.json]")

	src, err := parseOperand(flags, args, "source directory")
	if err != nil {
		return nil, err
	}
	req.Source = src
	req.Exclude = splitList(*exclude)
	return req, nil
}

func runValidate(args []string, stdout, stderr io.Writer) error {
	req, err := buildValidateRequest(args, stderr)
	if err != nil {
		return err
	}
	return deliverResult(req, stdout, stderr)
}

func buildValidateRequest(args []string, stderr io.Writer) (*command.ValidateRequest, error) {
	req := &command.ValidateRequest{}
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = usageFor(stderr, flags, "validate <codegraph.json>")

	model, err := parseOperand(flags, args, "codegraph.json path")
	if err != nil {
		return nil, err
	}
	req.Model = model
	return req, nil
}

func runCompose(args []string, stdout, stderr io.Writer) error {
	req, err := buildComposeRequest(args, stderr)
	if err != nil {
		return err
	}
	return deliverResult(req, stdout, stderr)
}

func buildComposeRequest(args []string, stderr io.Writer) (*command.ComposeRequest, error) {
	req := &command.ComposeRequest{}
	flags := flag.NewFlagSet("compose", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&req.Out, "o", "", "write the documents into this directory instead of standard output")
	flags.StringVar(&req.Family, "family", "", "compose only this family instead of every declared one")
	flags.Usage = usageFor(stderr, flags, "compose <codegraph.json> -o <dir>")

	model, err := parseOperand(flags, args, "codegraph.json path")
	if err != nil {
		return nil, err
	}
	req.Model = model
	return req, nil
}

func runRender(args []string, stdout, stderr io.Writer) error {
	req, err := buildRenderRequest(args, stderr)
	if err != nil {
		return err
	}
	return deliverResult(req, stdout, stderr)
}

func buildRenderRequest(args []string, stderr io.Writer) (*command.RenderRequest, error) {
	req := &command.RenderRequest{}
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&req.Out, "o", "", "write the page here instead of standard output")
	flags.Usage = usageFor(stderr, flags, "render <doc.json> -o out.html")

	doc, err := parseOperand(flags, args, "diagram source path")
	if err != nil {
		return nil, err
	}
	req.Document = doc
	return req, nil
}

// deliverResult runs a request and puts its output where each part belongs.
// runInstruct prints what stage 2 is performed from.
//
// It takes no operand, unlike every other command here, because it reads
// nothing: the instruction is built from the schemas the binary carries. That
// is the point of it being a command rather than a document in the repository.
func runInstruct(args []string, stdout, stderr io.Writer) error {
	req, err := buildInstructRequest(args, stderr)
	if err != nil {
		return err
	}
	return deliverResult(req, stdout, stderr)
}

func buildInstructRequest(args []string, stderr io.Writer) (*command.InstructRequest, error) {
	req := &command.InstructRequest{}
	flags := flag.NewFlagSet("instruct", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&req.Out, "o", "", "write the instruction here instead of standard output")
	flags.Usage = usageFor(stderr, flags, "instruct [-o prompt.md]")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if flags.NArg() != 0 {
		return nil, fmt.Errorf("instruct takes no arguments, got %d", flags.NArg())
	}
	return req, nil
}

func deliverResult(req command.Request, stdout, stderr io.Writer) error {
	result, err := req.Run(context.Background())
	if err != nil {
		return err
	}
	for _, line := range result.Summary {
		say(stderr, "%s\n", line)
	}
	for _, f := range result.Files {
		if f.Path != "" {
			continue // already written where it was asked for
		}
		if _, err := stdout.Write(f.Content); err != nil {
			return err
		}
	}
	return nil
}

func usageFor(w io.Writer, flags *flag.FlagSet, line string) func() {
	return func() {
		say(w, "usage: diagrammer %s\n", line)
		flags.PrintDefaults()
	}
}

// splitList turns a comma-separated flag into entries, dropping empty ones so
// that "a,,b" behaves as "a,b" and an unset flag yields nothing.
func splitList(raw string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && raw[i] != ',' {
			continue
		}
		if part := raw[start:i]; part != "" {
			out = append(out, part)
		}
		start = i + 1
	}
	return out
}
