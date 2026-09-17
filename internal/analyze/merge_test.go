package analyze_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/schema"
)

func empty(root string) *graph.Graph {
	return &graph.Graph{
		SchemaVersion: 1,
		Root:          root,
		Nodes:         []graph.Node{{ID: root, Kind: graph.KindGroup, Name: "tree"}},
		Edges:         []graph.Edge{},
		Diagnostics:   graph.Diagnostics{ParseFailures: []graph.ParseFailure{}},
	}
}

// TestEmptyOutputsAreStillArrays watches a whole class of defect rather than
// one instance of it.
//
// A slice built with var or with append onto nil is nil when nothing was
// appended, and a nil slice encodes as JSON null. The schema wants an array,
// so the document is refused — and the case that produces the nil is always
// the ordinary one: a tree with no edges, a merge with no failures, a level
// that draws nothing. It cost three separate debugging sessions before this
// test existed.
func TestEmptyOutputsAreStillArrays(t *testing.T) {
	merged, err := analyze.Merge(empty("root"), empty("root"))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	encoded, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for _, field := range []string{"edges", "nodes", "parseFailures"} {
		if strings.Contains(string(encoded), `"`+field+`": null`) {
			t.Errorf("%s encodes as null, which the schema refuses", field)
		}
	}
	if err := schema.Validate(schema.Graph, append(encoded, '\n')); err != nil {
		t.Errorf("a graph of an empty tree does not satisfy the schema: %v", err)
	}
}

// TestMergeKeepsOneRoot holds the claim that two parser paths do not become two
// graph shapes.
func TestMergeKeepsOneRoot(t *testing.T) {
	if _, err := analyze.Merge(empty("root"), empty("elsewhere")); err == nil {
		t.Error("two graphs with different roots were merged without complaint")
	}
	if _, err := analyze.Merge(); err == nil {
		t.Error("merging nothing succeeded")
	}
	one := empty("root")
	if got, err := analyze.Merge(one); err != nil || got != one {
		t.Error("merging a single graph should hand it straight back")
	}
}

// TestMergeDisownsAMixedDirectory checks the one judgement the merge makes.
//
// A directory two languages wrote into belongs to neither. Saying it is Python
// because Python happened to be read first would be a claim nobody checked, and
// the language field is there to be trusted.
func TestMergeDisownsAMixedDirectory(t *testing.T) {
	shared := func(lang graph.Language) *graph.Graph {
		g := empty("root")
		g.Nodes = append(g.Nodes, graph.Node{
			ID: "pkg:mixed", Kind: graph.KindPackage, Name: "mixed",
			Parent: "root", Language: lang,
		})
		return g
	}

	merged, err := analyze.Merge(shared(graph.Python), shared(graph.TypeScript))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	for _, n := range merged.Nodes {
		if n.ID == "pkg:mixed" && n.Language != "" {
			t.Errorf("a directory two languages wrote into claims to be %q", n.Language)
		}
	}

	// One language writing into it keeps its claim, which is the point of
	// dropping the other.
	alone, err := analyze.Merge(shared(graph.Python), empty("root"))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	for _, n := range alone.Nodes {
		if n.ID == "pkg:mixed" && n.Language != graph.Python {
			t.Errorf("a directory only Python wrote into says %q", n.Language)
		}
	}
}
