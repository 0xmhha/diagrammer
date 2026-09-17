package command

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/graph"
)

// What a refused file costs depends on which parser refused it, and the summary
// is the only place a person finds out.
//
// go/ast refuses a file outright: nothing of it arrives. tree-sitter recovers,
// and most of the file usually comes through — the two files in the reference
// tree this project reads contributed every declaration they have, and what the
// grammar could not read was one expression inside one function body. One
// sentence covering both was wrong half the time, and wrong in the direction
// that matters: telling somebody their code is absent when it is there.

func summaryOf(g *graph.Graph) string {
	out := &Result{}
	summariseGraph(out, "src", g)
	return strings.Join(out.Summary, "\n")
}

func declaredIn(path string, n int) []graph.Node {
	out := make([]graph.Node, 0, n)
	for i := range n {
		out = append(out, graph.Node{
			ID:     "decl:" + path + ".f" + string(rune('a'+i)),
			Kind:   graph.KindFunc,
			Name:   "f",
			Source: &graph.SourceRef{Path: path},
		})
	}
	return out
}

func TestTheSummarySeparatesAFileThatIsGoneFromAFileWithAHole(t *testing.T) {
	gone := summaryOf(&graph.Graph{
		Diagnostics: graph.Diagnostics{
			FilesParsed: 2,
			ParseFailures: []graph.ParseFailure{
				{Path: "bad.go", Line: 6, Message: "missing ',' in parameter list"},
			},
		},
	})
	if !strings.Contains(gone, "nothing from this file is in the graph") {
		t.Errorf("a file that contributed nothing is not reported as gone:\n%s", gone)
	}

	hole := summaryOf(&graph.Graph{
		Nodes: declaredIn("web/keys.js", 3),
		Diagnostics: graph.Diagnostics{
			FilesParsed: 1,
			ParseFailures: []graph.ParseFailure{
				{Path: "web/keys.js", Line: 13, Message: "column 18: the grammar does not accept what is here"},
			},
		},
	})
	if strings.Contains(hole, "nothing from this file") {
		t.Errorf("a file that contributed three declarations is reported as gone:\n%s", hole)
	}
	if !strings.Contains(hole, "3 declarations from this file are in the graph") {
		t.Errorf("the summary does not say how much of the file survived:\n%s", hole)
	}
	// The count is what separates the two, so it has to come from the graph and
	// not from the diagnostics, which do not know it.
	if !strings.Contains(hole, "what the message names and not the file") {
		t.Errorf("the summary does not say what is actually missing:\n%s", hole)
	}
}

// TestOnlyDeclarationsAreCounted guards the count itself. A file node and a
// package node carry the same path as the declarations in them, and counting
// those would report a file as partly present when nothing of its contents is.
func TestOnlyDeclarationsAreCounted(t *testing.T) {
	g := &graph.Graph{
		Nodes: []graph.Node{
			{ID: "file:bad.go", Kind: graph.KindFile, Source: &graph.SourceRef{Path: "bad.go"}},
			{ID: "pkg:.", Kind: graph.KindPackage},
		},
		Diagnostics: graph.Diagnostics{
			FilesParsed:   1,
			ParseFailures: []graph.ParseFailure{{Path: "bad.go", Message: "unreadable"}},
		},
	}
	if got := summaryOf(g); !strings.Contains(got, "nothing from this file is in the graph") {
		t.Errorf("a file whose only node is the file itself is reported as partly present:\n%s", got)
	}
}

// TestOneFileReadsAsOne is the kind of wrongness that makes a reader trust the
// rest of a report less.
func TestOneFileReadsAsOne(t *testing.T) {
	got := summaryOf(&graph.Graph{
		Diagnostics: graph.Diagnostics{
			FilesParsed:   1,
			ParseFailures: []graph.ParseFailure{{Path: "bad.go", Message: "unreadable"}},
		},
	})
	if !strings.Contains(got, "1 file has something") {
		t.Errorf("one file is not reported as one file:\n%s", got)
	}
}
