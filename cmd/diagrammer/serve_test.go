package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0xmhha/diagrammer/internal/command"
	"github.com/0xmhha/diagrammer/internal/schema"
)

// The contract these tests hold is that the CLI and the MCP server are two
// faces on one set of capabilities, and that neither gains a capability the
// other lacks. That claim is worth nothing unless something checks it, so the
// checks run over the real protocol rather than over the registry alone.

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := newServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("start the server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func listTools(t *testing.T) map[string]*mcp.Tool {
	t.Helper()
	result, err := connect(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	out := make(map[string]*mcp.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		out[tool.Name] = tool
	}
	return out
}

func call(t *testing.T, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := connect(t).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return result
}

func textOf(result *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range result.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// TestBothSurfacesCoverTheSameCapabilities is the contract itself. A tool added
// to one surface and not the other, or a subcommand with no tool behind it,
// fails here rather than being discovered by a plugin author.
func TestBothSurfacesCoverTheSameCapabilities(t *testing.T) {
	tools := listTools(t)

	var served []string
	for name := range tools {
		served = append(served, name)
	}
	sort.Strings(served)

	var declared []string
	for _, op := range command.Ops() {
		declared = append(declared, string(op))
	}
	sort.Strings(declared)

	if !reflect.DeepEqual(served, declared) {
		t.Errorf("the MCP server offers %v, the registry declares %v", served, declared)
	}

	// The CLI has version and help of its own, which are about the command
	// line rather than about a capability, so they are excluded deliberately
	// rather than by accident.
	surface := map[string]bool{}
	for _, c := range commands() {
		surface[c.name] = true
	}
	for _, op := range declared {
		if !surface[op] {
			t.Errorf("capability %q has a tool but no subcommand", op)
		}
	}
}

// TestToolArgumentsMatchTheRequests checks the tool schema a plugin reads is
// complete: every argument the request takes is offered, and none is offered
// that it does not take.
//
// The SDK builds the schema from the request type, so this cannot catch a
// rename. What it does catch is a field excluded from the schema, a required
// argument the request treats as optional, and this test's own table falling
// behind the registry. Whether the CLI reaches every field is a different
// question, and TestEveryRequestFieldHasAFlag asks it.
func TestToolArgumentsMatchTheRequests(t *testing.T) {
	tools := listTools(t)

	cases := map[command.Op]any{
		command.OpGraph:    command.GraphRequest{},
		command.OpValidate: command.ValidateRequest{},
		command.OpCompose:  command.ComposeRequest{},
		command.OpRender:   command.RenderRequest{},
	}
	if len(cases) != len(command.Ops()) {
		t.Fatalf("this table covers %d operations, the registry declares %d", len(cases), len(command.Ops()))
	}

	for op, request := range cases {
		t.Run(string(op), func(t *testing.T) {
			tool, ok := tools[string(op)]
			if !ok {
				t.Fatalf("no tool named %q", op)
			}
			raw, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("encode the tool schema: %v", err)
			}
			var declared struct {
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required"`
			}
			if err := json.Unmarshal(raw, &declared); err != nil {
				t.Fatalf("read the tool schema: %v", err)
			}

			typ := reflect.TypeOf(request)
			wanted := map[string]bool{}
			required := map[string]bool{}
			for i := range typ.NumField() {
				f := typ.Field(i)
				tag, ok := f.Tag.Lookup("json")
				if !ok {
					t.Errorf("%s.%s has no json tag, so it has no argument name", typ, f.Name)
					continue
				}
				parts := strings.Split(tag, ",")
				wanted[parts[0]] = true
				if len(parts) == 1 {
					required[parts[0]] = true
				}
			}

			for name := range wanted {
				if _, ok := declared.Properties[name]; !ok {
					t.Errorf("the request takes %q, the tool does not offer it", name)
				}
			}
			for name := range declared.Properties {
				if !wanted[name] {
					t.Errorf("the tool offers %q, the request does not take it", name)
				}
			}
			for _, name := range declared.Required {
				if !required[name] {
					t.Errorf("the tool requires %q, which the request treats as optional", name)
				}
			}
		})
	}
}

// TestEveryToolIsDescribed keeps a tool from arriving with nothing to say for
// itself. A model choosing between tools has only the description to go on.
func TestEveryToolIsDescribed(t *testing.T) {
	for name, tool := range listTools(t) {
		if len(tool.Description) < 40 {
			t.Errorf("%s: description is %q, which says too little for a caller to choose it", name, tool.Description)
		}
	}
}

func TestValidateOverMCP(t *testing.T) {
	t.Run("accepts the fixture", func(t *testing.T) {
		result := call(t, "validate", map[string]any{"model": fixture})
		if result.IsError {
			t.Fatalf("want accepted, got %s", textOf(result))
		}
		if !strings.Contains(textOf(result), "valid, 4 families") {
			t.Errorf("unexpected result: %s", textOf(result))
		}
	})

	t.Run("a refused model comes back as content, with the whole report", func(t *testing.T) {
		broken := filepath.Join(t.TempDir(), "broken.codegraph.json")
		body := `{"schemaVersion":1,"meta":{"title":"t"},"provenance":{"origin":"handwritten"},` +
			`"families":["component","sequence"],"component":{"components":[{"id":"a","name":"A"}]}}`
		if err := os.WriteFile(broken, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}

		result := call(t, "validate", map[string]any{"model": broken})
		if !result.IsError {
			t.Fatal("want the model refused")
		}
		// A refusal is the caller's to act on, so the detail has to survive the
		// trip. One line saying "invalid" would waste the useful part.
		if !strings.Contains(textOf(result), "sequence") {
			t.Errorf("the report lost the detail: %s", textOf(result))
		}
	})
}

func TestGraphOverMCP(t *testing.T) {
	result := call(t, "graph", map[string]any{"source": basicSrc})
	if result.IsError {
		t.Fatalf("want a graph, got %s", textOf(result))
	}
	text := textOf(result)
	if !strings.Contains(text, "nodes") {
		t.Errorf("no summary in the result: %s", text)
	}
	// The graph itself comes back when no file was named, and it is the same
	// document the command would have written.
	start := strings.Index(text, "{")
	if start < 0 {
		t.Fatalf("no graph in the result: %s", text)
	}
	if err := schema.Validate(schema.Graph, []byte(text[start:])); err != nil {
		t.Errorf("the returned graph does not satisfy the schema: %v", err)
	}
}

func TestComposeOverMCP(t *testing.T) {
	dir := t.TempDir()
	result := call(t, "compose", map[string]any{"model": fixture, "out": dir})
	if result.IsError {
		t.Fatalf("want documents, got %s", textOf(result))
	}
	for _, family := range []string{"component", "sequence", "state", "usecase"} {
		raw, err := os.ReadFile(filepath.Join(dir, family+".diagram.json"))
		if err != nil {
			t.Errorf("%s was not written: %v", family, err)
			continue
		}
		if err := schema.Validate(schema.Diagram, raw); err != nil {
			t.Errorf("%s does not satisfy the schema: %v", family, err)
		}
	}
}

// TestUnbuiltToolSaysSo keeps the two surfaces honest about the same gap: a
// capability named but not built reports that, on both faces.
func TestUnbuiltToolSaysSo(t *testing.T) {
	result := call(t, "render", map[string]any{"document": "doc.json"})
	if !result.IsError {
		t.Fatal("want render to report that it is not built")
	}
	if !strings.Contains(textOf(result), "not implemented") {
		t.Errorf("unexpected message: %s", textOf(result))
	}
}
