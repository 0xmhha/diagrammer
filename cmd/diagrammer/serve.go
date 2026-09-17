package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0xmhha/diagrammer/internal/command"
)

// The MCP server is the second face on the same set of capabilities, and it is
// built from the same registry the CLI is. Neither surface can gain a
// capability the other lacks, because neither declares one: they both read
// internal/command.
//
// The tool names and argument names become public contract the moment 0.1.0
// ships, since plugins bind to them. They are the operation names and the JSON
// tags of the request structs, so renaming one is a visible change to a
// contract rather than a tidy-up inside a flag declaration.

const serverName = "diagrammer"

// toolSummaries describes each tool to whatever is calling it. A model choosing
// between tools has only this to go on, so it says what the tool is for rather
// than what it is called.
func toolSummaries() map[command.Op]string {
	return map[command.Op]string{
		command.OpGraph: "Parse a source tree with an AST parser and return a code graph: packages, files, types and functions, " +
			"with the imports and calls that can be proven and the doc comments their authors wrote. " +
			"Structure only; nothing is interpreted. Go is read today.",
		command.OpValidate: "Check a UML codegraph against the stage-2 contract. This is the only gate at that boundary, " +
			"and it reports every defect at once rather than the first.",
		command.OpCompose: "Turn a validated UML codegraph into diagram sources, one per family the model declares. " +
			"Laid out but not drawn: boxes carry the cell they sit in, not a pixel position.",
		command.OpRender: "Turn a diagram source into a self-contained HTML page.",
		command.OpInstruct: "Return the instruction stage 2 is performed from: what a code graph holds, the schema a UML model " +
			"must satisfy, what the gate checks beyond that schema, and how to choose what to say. " +
			"Ask for this before reading a graph; it is built from the same schemas the gate uses, so it cannot disagree with them.",
	}
}

func runServe(args []string, _, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = usageFor(stderr, flags, "serve")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("serve takes no arguments, got %d", flags.NArg())
	}

	// stdio is the transport a local plugin starts the binary on. Nothing may
	// be written to standard output but protocol traffic, which is why every
	// summary the operations produce is returned in the tool result rather
	// than printed.
	return newServer().Run(context.Background(), &mcp.StdioTransport{})
}

// newServer registers one tool per capability.
//
// Each line names an operation from the registry and the request type that
// carries its arguments. The SDK reads the tool's argument schema off that
// type, so the argument names a plugin sees are the request's own JSON tags and
// cannot drift from the ones the CLI fills in.
func newServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: version}, nil)
	summaries := toolSummaries()

	addOp[command.GraphRequest](server, command.OpGraph, summaries)
	addOp[command.ValidateRequest](server, command.OpValidate, summaries)
	addOp[command.ComposeRequest](server, command.OpCompose, summaries)
	addOp[command.RenderRequest](server, command.OpRender, summaries)
	addOp[command.InstructRequest](server, command.OpInstruct, summaries)
	return server
}

// addOp registers one capability as a tool.
//
// The constraint says T is a struct whose pointer is a Request, which is what
// lets the SDK infer the argument schema from the struct while the operation
// itself hangs off the pointer.
func addOp[T any, PT interface {
	*T
	command.Request
}](server *mcp.Server, op command.Op, summaries map[command.Op]string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        string(op),
		Description: summaries[op],
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in T) (*mcp.CallToolResult, any, error) {
		result, err := PT(&in).Run(ctx)
		if err != nil {
			// A refused document is the caller's problem to fix, not a fault in
			// the server, so it comes back as tool content rather than as a
			// protocol error. The whole report survives: a validation failure
			// names every defect, and collapsing it to one line would waste the
			// part that is useful.
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}
		return toolResult(result), nil, nil
	})
}

// toolResult turns an operation's output into content a caller can read.
//
// The summary comes first because it is what a person or a model reads to know
// what happened. Documents that were written to a file are named rather than
// repeated, since a code graph runs to hundreds of megabytes and nobody wants
// it twice.
func toolResult(result *command.Result) *mcp.CallToolResult {
	var content []mcp.Content
	if len(result.Summary) > 0 {
		content = append(content, &mcp.TextContent{Text: strings.Join(result.Summary, "\n")})
	}
	for _, f := range result.Files {
		if f.Path != "" {
			continue
		}
		content = append(content, &mcp.TextContent{Text: string(f.Content)})
	}
	return &mcp.CallToolResult{Content: content}
}
