package instruct_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/instruct"
	"github.com/0xmhha/diagrammer/internal/schema"
)

// The instruction is built, not stored, and the reason is that a stored copy
// drifts. These check the building actually happened.

// TestTheSchemasTravelVerbatim is the whole point of generating this rather
// than writing it. A description of a schema is a second statement of the
// contract that can disagree with the first, and the gate checks the document
// against the schema, not against the prose.
func TestTheSchemasTravelVerbatim(t *testing.T) {
	told, err := instruct.Stage2()
	if err != nil {
		t.Fatalf("build the instruction: %v", err)
	}

	for _, name := range []schema.Name{schema.Graph, schema.Codegraph} {
		raw, err := schema.Raw(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(told, strings.TrimRight(string(raw), "\n")) {
			t.Errorf("the %s schema is not carried verbatim, so a model is told something "+
				"other than what the gate will check", name)
		}
		if !strings.Contains(told, schema.Filename(name)) {
			t.Errorf("the %s schema is carried without being named, so a reader cannot tell "+
				"which of the two it is looking at", name)
		}
	}
}

// TestTheDiagramSchemaIsNotCarried is the other half. Stage 2 does not write a
// diagram source and handing it that schema would invite a model to try, which
// stage 3 would then refuse.
func TestTheDiagramSchemaIsNotCarried(t *testing.T) {
	told, err := instruct.Stage2()
	if err != nil {
		t.Fatalf("build the instruction: %v", err)
	}
	if strings.Contains(told, schema.Filename(schema.Diagram)) {
		t.Error("the instruction carries the diagram schema, which is stage 3's output and not stage 2's")
	}
}

// TestTheCarriedSchemasStillParse guards the fencing. A schema spliced into
// markdown badly is a schema a model reads wrongly, and the splice is string
// work that nothing else checks.
func TestTheCarriedSchemasStillParse(t *testing.T) {
	told, err := instruct.Stage2()
	if err != nil {
		t.Fatalf("build the instruction: %v", err)
	}
	blocks := fencedJSON(told)
	if len(blocks) != 2 {
		t.Fatalf("the instruction carries %d JSON blocks; it should carry the two schemas", len(blocks))
	}
	for i, block := range blocks {
		var doc map[string]any
		if err := json.Unmarshal([]byte(block), &doc); err != nil {
			t.Errorf("JSON block %d does not parse: %v", i, err)
			continue
		}
		if _, ok := doc["$schema"]; !ok {
			t.Errorf("JSON block %d parses but declares no $schema, so it is not a schema", i)
		}
	}
}

// TestItSaysWhatToReturn covers the instruction's own job. A model that reads
// all of this and still does not know it must answer with one JSON document has
// been told the contract and not the task.
func TestItSaysWhatToReturn(t *testing.T) {
	told, err := instruct.Stage2()
	if err != nil {
		t.Fatalf("build the instruction: %v", err)
	}
	for _, phrase := range []string{
		"Return one JSON document and nothing else",
		"drawn + dropped == proven",
		"provenance",
	} {
		if !strings.Contains(told, phrase) {
			t.Errorf("the instruction never says %q", phrase)
		}
	}
}

// fencedJSON pulls the contents of every ```json block out of markdown.
func fencedJSON(text string) []string {
	var out []string
	rest := text
	for {
		start := strings.Index(rest, "```json\n")
		if start < 0 {
			return out
		}
		rest = rest[start+len("```json\n"):]
		end := strings.Index(rest, "\n```")
		if end < 0 {
			return out
		}
		out = append(out, rest[:end])
		rest = rest[end:]
	}
}
