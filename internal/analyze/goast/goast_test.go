package goast_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/0xmhha/diagrammer/internal/analyze/goast"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/schema"
)

func fixture(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "src", name)
}

func analyze(t *testing.T, name string) *graph.Graph {
	t.Helper()
	g, err := goast.Analyze(context.Background(), fixture(name), goast.Options{})
	if err != nil {
		t.Fatalf("analyze %s: %v", name, err)
	}
	return g
}

func encode(t *testing.T, g *graph.Graph) []byte {
	t.Helper()
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b
}

// TestGraphSatisfiesItsSchema is the check the analyzer exists to pass. A graph
// that does not validate is a defect in the analyzer, never in the source.
func TestGraphSatisfiesItsSchema(t *testing.T) {
	for _, name := range []string{"go-basic", "go-broken"} {
		t.Run(name, func(t *testing.T) {
			if err := schema.Validate(schema.Graph, encode(t, analyze(t, name))); err != nil {
				t.Errorf("graph does not satisfy the schema: %v", err)
			}
		})
	}
}

// TestGraphIsByteIdenticalAcrossRuns holds the determinism the contract
// requires. Map iteration is deliberately unordered in Go, so this is the test
// that catches an unsorted field before it reaches a diff someone has to
// explain.
func TestGraphIsByteIdenticalAcrossRuns(t *testing.T) {
	for _, name := range []string{"go-basic", "go-broken"} {
		t.Run(name, func(t *testing.T) {
			first := encode(t, analyze(t, name))
			for i := range 4 {
				again := encode(t, analyze(t, name))
				if string(first) != string(again) {
					t.Fatalf("run %d differs from the first", i+2)
				}
			}
		})
	}
}

func TestGraphShape(t *testing.T) {
	g := analyze(t, "go-basic")

	byID := make(map[string]graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}

	t.Run("kinds", func(t *testing.T) {
		counts := map[graph.NodeKind]int{}
		for _, n := range g.Nodes {
			counts[n.Kind]++
		}
		want := map[graph.NodeKind]int{
			graph.KindGroup:   1, // internal/, which holds no Go files of its own
			graph.KindPackage: 4, // the root, store, service and internal/util
			graph.KindFile:    4,
			graph.KindType:    3,
			graph.KindFunc:    8,
		}
		for kind, n := range want {
			if counts[kind] != n {
				t.Errorf("%s: want %d, got %d", kind, n, counts[kind])
			}
		}
	})

	t.Run("declarations", func(t *testing.T) {
		cases := []struct {
			id       string
			kind     graph.NodeKind
			exported bool
			wantDoc  bool
		}{
			{"ty:store.Store", graph.KindType, true, true},
			{"fn:store.(Store).Save", graph.KindFunc, true, true},
			{"fn:service.(Service).lookup", graph.KindFunc, false, true},
			{"fn:internal/util.Normalize", graph.KindFunc, true, true},
			{"pkg:internal", graph.KindGroup, false, false},
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
			if (n.Doc != "") != c.wantDoc {
				t.Errorf("%s: doc present is %v, want %v", c.id, n.Doc != "", c.wantDoc)
			}
		}
	})

	t.Run("edges", func(t *testing.T) {
		seen := map[string]bool{}
		for _, e := range g.Edges {
			seen[string(e.Kind)+" "+e.From+" -> "+e.To] = true
		}
		want := []string{
			// a call that crosses a package boundary
			"call fn:service.(Service).Place -> fn:store.(Store).Save",
			// a call to an unexported method in the same package
			"call fn:service.(Service).Exists -> fn:service.(Service).lookup",
			// a call reaching down into internal/
			"call fn:store.(Store).Save -> fn:internal/util.Normalize",
			// the imports those calls travel over
			"import pkg:service -> pkg:store",
			"import pkg:store -> pkg:internal/util",
		}
		for _, w := range want {
			if !seen[w] {
				t.Errorf("edge is missing: %s", w)
			}
		}
	})
}

// TestEveryNodeReachesTheRoot is the structural invariant the schema cannot
// state: exactly one node has no parent, and every other one walks up to it.
// A broken parent pointer would otherwise leave a subtree that validates but
// can never be drawn.
func TestEveryNodeReachesTheRoot(t *testing.T) {
	g := analyze(t, "go-basic")

	byID := make(map[string]graph.Node, len(g.Nodes))
	var roots []string
	for _, n := range g.Nodes {
		byID[n.ID] = n
		if n.Parent == "" {
			roots = append(roots, n.ID)
		}
	}
	if len(roots) != 1 || roots[0] != g.Root {
		t.Fatalf("want exactly one parentless node named %q, got %v", g.Root, roots)
	}

	for _, n := range g.Nodes {
		steps := 0
		for cur := n; cur.Parent != ""; steps++ {
			if steps > len(g.Nodes) {
				t.Fatalf("%s: parent chain does not terminate", n.ID)
			}
			parent, ok := byID[cur.Parent]
			if !ok {
				t.Errorf("%s: parent %q is not in the graph", n.ID, cur.Parent)
				break
			}
			cur = parent
		}
	}
}

// TestEveryEdgeEndpointExists keeps an edge from naming a node that was never
// emitted. A dangling endpoint draws as a line to nowhere.
func TestEveryEdgeEndpointExists(t *testing.T) {
	g := analyze(t, "go-basic")
	byID := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		byID[n.ID] = true
	}
	for _, e := range g.Edges {
		if !byID[e.From] {
			t.Errorf("edge from %q, which is not a node", e.From)
		}
		if !byID[e.To] {
			t.Errorf("edge to %q, which is not a node", e.To)
		}
	}
}

// TestParseFailureIsReportedNotDropped is the fixture that has to exist for the
// diagnostics block to mean anything.
//
// go/ast refuses the file, so its declarations are genuinely absent from the
// graph. What must not happen is that they are absent without a trace: a graph
// of a project with a broken file and a graph of a project that never had it
// would otherwise be identical.
func TestParseFailureIsReportedNotDropped(t *testing.T) {
	g := analyze(t, "go-broken")
	d := g.Diagnostics

	if len(d.ParseFailures) != 1 {
		t.Fatalf("want exactly one parse failure, got %d: %+v", len(d.ParseFailures), d.ParseFailures)
	}
	failure := d.ParseFailures[0]
	if failure.Path != "bad.go" {
		t.Errorf("failure names %q, want bad.go", failure.Path)
	}
	if failure.Language != graph.Go {
		t.Errorf("failure language is %q, want go", failure.Language)
	}
	if failure.Line == 0 {
		t.Error("failure carries no line, so a reader has to go and search for it")
	}
	if failure.Message == "" {
		t.Error("failure carries no message")
	}

	// Two files were read: one parsed and one did not. The count must include
	// both, or a broken file would not even register as having been opened.
	if d.FilesParsed != 2 {
		t.Errorf("filesParsed is %d, want 2", d.FilesParsed)
	}

	byID := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		byID[n.ID] = true
	}
	// The root package's rel path is ".", so its declarations are keyed
	// "fn:..Name". The doubled dot is the id scheme showing through rather than
	// a defect, and the test names it so a reader is not left wondering.
	if !byID["fn:..Good"] {
		t.Error("the file that parsed is missing from the graph")
	}
	if byID["fn:..Bad"] {
		t.Error("the file that did not parse contributed a node, so the fixture no longer proves anything")
	}
}

// TestOptions checks the knobs do something, since an option that is accepted
// and ignored is worse than one that does not exist.
func TestOptions(t *testing.T) {
	base := analyze(t, "go-basic")

	t.Run("exclude drops a subtree", func(t *testing.T) {
		g, err := goast.Analyze(context.Background(), fixture("go-basic"), goast.Options{Exclude: []string{"internal"}})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		for _, n := range g.Nodes {
			if n.ID == "pkg:internal/util" {
				t.Error("internal/util survived being excluded")
			}
		}
		if len(g.Nodes) >= len(base.Nodes) {
			t.Errorf("excluding a package did not shrink the graph: %d then %d", len(base.Nodes), len(g.Nodes))
		}
	})

	t.Run("max depth stops the walk", func(t *testing.T) {
		g, err := goast.Analyze(context.Background(), fixture("go-basic"), goast.Options{MaxDepth: 1})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		for _, n := range g.Nodes {
			if n.ID == "pkg:internal/util" {
				t.Error("internal/util is two deep and should not have been reached")
			}
		}
	})
}

// TestCancellation proves the walk can be stopped. A traversal that ignores its
// context cannot be interrupted once it is handed a large tree.
func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := goast.Analyze(ctx, fixture("go-basic"), goast.Options{}); err == nil {
		t.Error("want the cancelled walk to fail, got success")
	}
}

// TestSymlinkedFilesAreNotRead closes a way of getting a file from outside the
// analyzed tree into a graph of it.
//
// A symlink named *.go is not a directory, so a filter that only skipped
// directories let it through, and go/ast happily followed it. The declarations
// arrived recorded under the in-tree name, so nothing in the graph said the
// content came from somewhere else. A repository that ships such a link decides
// which of the reader's own files end up in their diagram, and the graph is
// handed to a model in the next stage.
func TestSymlinkedFilesAreNotRead(t *testing.T) {
	g := analyze(t, "go-symlink")

	ids := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		ids[n.ID] = true
	}
	if !ids["fn:..Own"] {
		t.Fatal("the fixture's own file was not read, so this test proves nothing")
	}

	// linked.go points at go-basic/doc.go, which declares Version and hidden.
	// Nothing from it may appear, and neither may the file node itself.
	for _, forbidden := range []string{"file:linked.go", "fn:..Version", "ty:.Version"} {
		if ids[forbidden] {
			t.Errorf("%s came from outside the analyzed tree", forbidden)
		}
	}
	for _, n := range g.Nodes {
		if n.Source != nil && n.Source.Path == "linked.go" {
			t.Errorf("%s is recorded at linked.go, which is a symlink out of the tree", n.ID)
		}
	}

	// The link is not a parse failure either. It is skipped as something that
	// is not a file to read, which is a different thing from a file that would
	// not parse, and saying so wrongly would send a reader looking for a syntax
	// error that is not there.
	for _, f := range g.Diagnostics.ParseFailures {
		if f.Path == "linked.go" {
			t.Errorf("the symlink is reported as a parse failure: %s", f.Message)
		}
	}
}
