package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0xmhha/diagrammer/internal/command"
	"github.com/0xmhha/diagrammer/internal/instruct"
	"github.com/0xmhha/diagrammer/internal/schema"
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

// stage2Prompt is the name a client asks for the instruction by. It is
// contract: a plugin binds to it.
const stage2Prompt = "stage-2"

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
		command.OpMermaid: "Write a diagram source as Mermaid text, one fenced block per level, for a README or a tool that " +
			"redraws. Unlike the page it is not bound by a grid, so it carries every relationship the document proved.",
		command.OpSVG: "Write a diagram source's drawings as standalone SVG files, one per level, framed to a size a slide, " +
			"a document or a print has. The same drawing the page holds, with nothing to move between levels.",
		command.OpInstruct: "Return the instruction stage 2 is performed from: what a code graph holds, the schema a UML model " +
			"must satisfy, what the gate checks beyond that schema, and how to choose what to say. " +
			"Ask for this before reading a graph; it is built from the same schemas the gate uses, so it cannot disagree with them.",
	}
}

func runServe(args []string, _, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var given string
	flags.StringVar(&given, "root", "", "confine tool arguments to this directory (default: the working directory)")
	flags.Usage = usageFor(stderr, flags, "serve [-root dir]")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("serve takes no arguments, got %d", flags.NArg())
	}
	root, err := newRoot(given)
	if err != nil {
		return err
	}
	// Said once, on the way up, because a plugin reading stderr is the only
	// place an operator finds out what the server will and will not reach.
	say(stderr, "serving; tool arguments are confined to %s\n", root.resolved)

	// stdio is the transport a local plugin starts the binary on. Nothing may
	// be written to standard output but protocol traffic, which is why every
	// summary the operations produce is returned in the tool result rather
	// than printed.
	return newServer(root).Run(context.Background(), &mcp.StdioTransport{})
}

// newServer registers one tool per capability.
//
// Each line names an operation from the registry and the request type that
// carries its arguments. The SDK reads the tool's argument schema off that
// type, so the argument names a plugin sees are the request's own JSON tags and
// cannot drift from the ones the CLI fills in.
func newServer(root aRoot) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	summaries := toolSummaries()

	addOp[command.GraphRequest](server, command.OpGraph, summaries, root)
	addOp[command.ValidateRequest](server, command.OpValidate, summaries, root)
	addOp[command.ComposeRequest](server, command.OpCompose, summaries, root)
	addOp[command.RenderRequest](server, command.OpRender, summaries, root)
	addOp[command.InstructRequest](server, command.OpInstruct, summaries, root)
	addOp[command.MermaidRequest](server, command.OpMermaid, summaries, root)
	addOp[command.SVGRequest](server, command.OpSVG, summaries, root)

	addStage2Prompt(server)
	addSchemaResources(server)
	return server
}

// serverInstructions is what a client puts in front of the model before it has
// called anything.
//
// A tool's description says what that tool does. Nothing in a list of tools
// says what the four of them are for, which order they go in, or that one whole
// stage is the caller's own work. On the command line that is what `diagrammer`
// with no arguments prints; over MCP this field is the only place it fits, and
// leaving it empty is how a server ends up being used one tool at a time by
// something that never learned what it was holding.
const serverInstructions = `diagrammer turns a source tree into UML diagrams in four stages. You perform
the second one.

  graph     a source tree in, a code graph out
  (you)     the code graph in, a UML model out
  compose   the UML model in, one diagram source per family out
  render    a diagram source in, a self-contained HTML page out
  mermaid   the same diagram source in, Mermaid text out, for a README or a
            tool that redraws
  svg       the same diagram source in, one SVG file per level out, framed
            for a slide, a document or a print

Ask for the ` + "`stage-2`" + ` prompt before anything else. It carries the schema your
model has to satisfy and what the gate checks beyond that schema, and it is
built from the same files the gate uses, so it cannot disagree with them. The
` + "`instruct`" + ` tool returns the same text if prompts are not available to you.

Then: call ` + "`graph`" + ` on the tree, read what comes back, and write the UML model
yourself. This server has no tool that writes it, and never calls a model.
` + "`validate`" + ` will accept or refuse what you wrote and name every defect at once.
` + "`compose`" + ` and ` + "`render`" + ` take it from there. ` + "`mermaid`" + ` is the other way out of a
diagram source: text rather than a page, carrying every relationship the
document proved, one fenced block per level. ` + "`svg`" + ` is the third: the page's
drawing as a file, framed to a size the destination has.

The schemas are also readable as resources if you want one on its own.

Every path argument is a path on the machine this server runs on, and they are
confined to one directory: a path outside it is refused with the root named, and
only the person who started the server can widen it. Pass paths the person asked
for, and do not go looking for others.`

// addStage2Prompt offers the stage-2 instruction as a prompt.
//
// It is already a tool, and it is both deliberately. A tool is a thing a model
// decides to call; a prompt is text a client can put in front of the model
// before it decides anything, which is what this actually is. Offering it only
// as a tool leaves the work of knowing to ask entirely to the caller.
func addStage2Prompt(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        stage2Prompt,
		Title:       "Stage 2: read a code graph, return a UML model",
		Description: "What a code graph holds, the schema a UML model must satisfy, what the gate checks beyond that schema, and how to choose what to say.",
	}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		text, err := instruct.Stage2()
		if err != nil {
			return nil, err
		}
		return &mcp.GetPromptResult{
			Description: "The instruction stage 2 is performed from.",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: text},
			}},
		}, nil
	})
}

// addSchemaResources offers each embedded schema under the identifier it
// declares as its own $id.
//
// The prompt carries two of them already. A resource is for the caller that
// wants one on its own: to check a document it is holding, or to read the
// diagram schema, which the prompt deliberately leaves out because writing one
// is not stage 2's job.
func addSchemaResources(server *mcp.Server) {
	for _, name := range schema.All() {
		server.AddResource(&mcp.Resource{
			URI:         schema.URI(name),
			Name:        string(name),
			Description: schemaDescriptions()[name],
			MIMEType:    "application/schema+json",
		}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			raw, err := schema.Raw(name)
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/schema+json",
				Text:     string(raw),
			}}}, nil
		})
	}
}

// schemaDescriptions says which stage each schema belongs to, because a caller
// choosing between three needs to know that before it reads any of them.
func schemaDescriptions() map[schema.Name]string {
	return map[schema.Name]string{
		schema.Graph:     "Stage 1's output: the code graph an analyzer emits, and what graph returns.",
		schema.Codegraph: "Stage 2's output: the UML model you return, and what validate checks.",
		schema.Diagram:   "Stage 3's output: the diagram source compose emits and render draws. Not yours to write.",
	}
}

// addOp registers one capability as a tool.
//
// The constraint says T is a struct whose pointer is a Request, which is what
// lets the SDK infer the argument schema from the struct while the operation
// itself hangs off the pointer.
func addOp[T any, PT interface {
	*T
	command.Request
}](server *mcp.Server, op command.Op, summaries map[command.Op]string, root aRoot) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        string(op),
		Description: summaries[op],
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in T) (*mcp.CallToolResult, any, error) {
		// Checked before the operation runs, so a refused path is refused
		// rather than half-acted-on. The check is here and not in the
		// operation because the command line is not confined and the two
		// surfaces share the request.
		if err := root.confine(PT(&in)); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}
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
