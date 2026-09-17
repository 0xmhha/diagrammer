package main

import (
	"context"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/instruct"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// How a caller learns what this server is for.
//
// A tool's description says what that tool does. Nothing in a list of tools
// says what the four of them are for, which order they go in, or that one whole
// stage is the caller's own work. On the command line that is what `diagrammer`
// with no arguments prints. Over MCP there are three places it can go, and the
// server was using one of them.
//
// These run over the real protocol rather than over the registry, because what
// matters is what a client receives.

// TestTheServerSaysWhatItIsFor is the one that was empty. Instructions reach a
// client before it has called anything, which is the only moment at which
// "you perform stage 2 yourself" can arrive in time to be true.
func TestTheServerSaysWhatItIsFor(t *testing.T) {
	session := connect(t)
	result := session.InitializeResult()
	if result == nil {
		t.Fatal("the session carries no initialize result")
	}
	if strings.TrimSpace(result.Instructions) == "" {
		t.Fatal("the server sends no instructions, so a caller meets four tools and no account of them")
	}

	// The things a caller cannot work out from the tool list alone.
	for _, what := range []string{
		"four stages",
		"You perform",
		stage2Prompt,
		"never calls a model",
	} {
		if !strings.Contains(result.Instructions, what) {
			t.Errorf("the instructions never mention %q", what)
		}
	}

	// Every tool is named, so nothing is offered without being placed.
	for _, op := range declaredOps(t) {
		if !strings.Contains(result.Instructions, "`"+op+"`") {
			t.Errorf("the instructions never place the %q tool", op)
		}
	}
}

// TestTheStage2InstructionIsAPrompt covers the primitive built for this. A tool
// is a thing a model decides to call; a prompt is text a client can put in
// front of it before it decides anything, which is what this is.
func TestTheStage2InstructionIsAPrompt(t *testing.T) {
	session := connect(t)
	listed, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	var found *mcp.Prompt
	for _, p := range listed.Prompts {
		if p.Name == stage2Prompt {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("no prompt named %q is offered", stage2Prompt)
	}
	if found.Description == "" {
		t.Error("the prompt is offered without a description, so a caller cannot tell what it is")
	}

	got, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: stage2Prompt})
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("the prompt came back as %d messages; it is one instruction", len(got.Messages))
	}
	text, ok := got.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("the prompt's content is %T rather than text", got.Messages[0].Content)
	}

	// The same text the tool and the command hand out. Two spellings of one
	// instruction is the drift this whole capability exists to avoid.
	want, err := instruct.Stage2()
	if err != nil {
		t.Fatalf("build the instruction: %v", err)
	}
	if text.Text != want {
		t.Error("the prompt and the instruction differ, so a caller is told one thing by the prompt " +
			"and another by the tool")
	}
}

// TestEverySchemaIsReadableOnItsOwn covers the third primitive. The prompt
// carries two schemas; a caller wanting one on its own, or wanting the diagram
// schema the prompt deliberately leaves out, needs somewhere to ask.
func TestEverySchemaIsReadableOnItsOwn(t *testing.T) {
	session := connect(t)
	listed, err := session.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	offered := map[string]*mcp.Resource{}
	for _, r := range listed.Resources {
		offered[r.URI] = r
	}

	for _, name := range schema.All() {
		uri := schema.URI(name)
		r, ok := offered[uri]
		if !ok {
			t.Errorf("the %s schema is not offered as a resource", name)
			continue
		}
		if r.Description == "" {
			t.Errorf("the %s schema is offered without saying which stage it belongs to", name)
		}

		got, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if len(got.Contents) != 1 {
			t.Errorf("%s came back as %d parts; it is one file", name, len(got.Contents))
			continue
		}
		want, err := schema.Raw(name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		if got.Contents[0].Text != string(want) {
			t.Errorf("the %s served over MCP is not the one embedded in the binary", name)
		}
	}
	if len(offered) != len(schema.All()) {
		t.Errorf("%d resources are offered and %d schemas exist", len(offered), len(schema.All()))
	}
}

// declaredOps lists the tools the server actually offers, so the instructions
// are held to what is there rather than to a list written beside them.
func declaredOps(t *testing.T) []string {
	t.Helper()
	var out []string
	for name := range listTools(t) {
		out = append(out, name)
	}
	return out
}
