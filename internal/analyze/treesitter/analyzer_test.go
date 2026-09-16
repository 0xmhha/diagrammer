//go:build cgo

package treesitter_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/analyze/treesitter"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/schema"
)

func fixture() string { return filepath.Join("..", "..", "..", "testdata", "src", "polyglot") }

func analyzeAll(t *testing.T) *graph.Graph {
	t.Helper()
	var graphs []*graph.Graph
	for _, a := range treesitter.Analyzers() {
		g, err := a.Analyze(context.Background(), fixture(), analyze.Options{})
		if err != nil {
			t.Fatalf("%s: %v", a.Language(), err)
		}
		graphs = append(graphs, g)
	}
	merged, err := analyze.Merge(graphs...)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	return merged
}

func TestGraphSatisfiesItsSchema(t *testing.T) {
	encoded, err := json.MarshalIndent(analyzeAll(t), "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := schema.Validate(schema.Graph, append(encoded, '\n')); err != nil {
		t.Errorf("graph does not satisfy the schema: %v", err)
	}
}

func TestGraphIsByteIdenticalAcrossRuns(t *testing.T) {
	first, err := json.Marshal(analyzeAll(t))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for i := range 3 {
		again, err := json.Marshal(analyzeAll(t))
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("run %d differs from the first", i+2)
		}
	}
}

// TestEachLanguageIsRead checks every grammar actually found something, because
// an analyzer that silently reads nothing looks exactly like a tree with none
// of that language in it.
func TestEachLanguageIsRead(t *testing.T) {
	g := analyzeAll(t)
	found := map[graph.Language]int{}
	for _, n := range g.Nodes {
		if n.Language != "" {
			found[n.Language]++
		}
	}
	for _, want := range []graph.Language{graph.Python, graph.Solidity, graph.JavaScript, graph.TypeScript} {
		if found[want] == 0 {
			t.Errorf("%s contributed nothing to the graph", want)
		}
	}
}

func TestDeclarationsAndDocumentation(t *testing.T) {
	g := analyzeAll(t)
	byID := map[string]graph.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}

	cases := []struct {
		id       string
		kind     graph.NodeKind
		exported bool
		doc      string
	}{
		// Python keeps its documentation in a docstring inside the body.
		{"decl:api/store.py.Store", graph.KindType, true, "Holds orders in memory."},
		{"decl:api/store.py.save", graph.KindFunc, true, "Record an order"},
		{"decl:api/store.py._evict", graph.KindFunc, false, "Not part of the public surface."},
		// Solidity keeps it in a comment above, and marks visibility with a
		// leading underscore by convention.
		{"decl:contracts/Vault.sol.Vault", graph.KindType, true, "Holds deposits for one owner."},
		{"decl:contracts/Vault.sol.deposit", graph.KindFunc, true, "Take a deposit"},
		{"decl:contracts/Vault.sol._record", graph.KindFunc, false, "Internal bookkeeping."},
		// TypeScript marks visibility with a keyword above the declaration, so
		// the comment sits above the wrapper rather than above the declaration.
		{"decl:web/client.ts.Order", graph.KindType, true, "Talks to the order API."},
		{"decl:web/client.ts.place", graph.KindFunc, true, "Sends an order"},
		{"decl:web/client.ts.normalise", graph.KindFunc, false, "Not exported"},
	}
	for _, c := range cases {
		n, ok := byID[c.id]
		if !ok {
			t.Errorf("%s is missing from the graph", c.id)
			continue
		}
		if n.Kind != c.kind {
			t.Errorf("%s: kind is %q, want %q", c.id, n.Kind, c.kind)
		}
		if n.Exported != c.exported {
			t.Errorf("%s: exported is %v, want %v", c.id, n.Exported, c.exported)
		}
		if !strings.Contains(n.Doc, c.doc) {
			t.Errorf("%s: doc is %q, want it to contain %q", c.id, n.Doc, c.doc)
		}
	}
}

// TestSoftFailureIsReported is the one that answers the objection to using
// tree-sitter at all.
//
// It does not refuse a file it cannot parse: it emits an ERROR node and carries
// on, which is how code goes missing from a graph that looks complete. Asking
// with HasError turns that silent gap into a reported one, and this is what
// keeps the asking honest.
func TestSoftFailureIsReported(t *testing.T) {
	g := analyzeAll(t)

	var broken *graph.ParseFailure
	for i, f := range g.Diagnostics.ParseFailures {
		if strings.HasSuffix(f.Path, "broken.js") {
			broken = &g.Diagnostics.ParseFailures[i]
		}
	}
	if broken == nil {
		t.Fatal("the file that does not parse is not in the diagnostics; it went missing in silence")
	}
	if broken.Line == 0 {
		t.Error("the failure carries no line, so a reader has to go and search for it")
	}
	if broken.Language != graph.JavaScript {
		t.Errorf("the failure names %q rather than javascript", broken.Language)
	}

	// The recovery is the point of the soft failure: what could be read still
	// is. The report is what stops the rest from vanishing quietly.
	found := false
	for _, n := range g.Nodes {
		if n.ID == "decl:web/broken.js.fine" {
			found = true
		}
	}
	if !found {
		t.Error("the part of the broken file that does parse was dropped too")
	}
}

// TestSymlinkedFilesAreNotRead holds the grammars to the same rule go/ast is
// held to: a symlink named like source is not source in this tree.
func TestSymlinkedFilesAreNotRead(t *testing.T) {
	for _, a := range treesitter.Analyzers() {
		g, err := a.Analyze(context.Background(), filepath.Join("..", "..", "..", "testdata", "src", "go-symlink"), analyze.Options{})
		if err != nil {
			t.Fatalf("%s: %v", a.Language(), err)
		}
		if len(g.Nodes) > 1 {
			t.Errorf("%s read something from a tree that holds only Go and a symlink", a.Language())
		}
	}
}
