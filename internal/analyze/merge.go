package analyze

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/graph"
)

// Merge combines what several analyzers found into one graph.
//
// A repository is not one language, and a graph of it should not be one either.
// Two parser paths must not become two graph shapes, so the merge is here
// rather than in any analyzer: every one of them emits the same five node kinds
// against the same root, and this joins them without any of them knowing the
// others exist.
//
// The result is sorted, so it is byte-identical however many analyzers
// contributed and whatever order they ran in.
func Merge(graphs ...*graph.Graph) (*graph.Graph, error) {
	present := make([]*graph.Graph, 0, len(graphs))
	for _, g := range graphs {
		if g != nil {
			present = append(present, g)
		}
	}
	if len(present) == 0 {
		return nil, fmt.Errorf("nothing to merge")
	}
	if len(present) == 1 {
		return present[0], nil
	}

	nodes := map[string]graph.Node{}
	// contributors counts the languages that put something under a node, so a
	// directory holding two languages can stop claiming to be either.
	contributors := map[string]map[graph.Language]bool{}
	edges := map[string]*graph.Edge{}
	// Built empty rather than nil: a nil slice encodes as null and the schema
	// wants an array. A merge of graphs that all parsed cleanly is the ordinary
	// case, and it is exactly the one that produces the nil.
	merged := graph.Diagnostics{ParseFailures: []graph.ParseFailure{}}
	unresolved := map[string]int{}

	for _, g := range present {
		if g.Root != present[0].Root {
			return nil, fmt.Errorf("analyzers disagree about the root: %q and %q", present[0].Root, g.Root)
		}
		for _, n := range g.Nodes {
			if contributors[n.ID] == nil {
				contributors[n.ID] = map[graph.Language]bool{}
			}
			if n.Language != "" {
				contributors[n.ID][n.Language] = true
			}
			existing, seen := nodes[n.ID]
			if !seen {
				nodes[n.ID] = n
				continue
			}
			// The same id from two analyzers is a directory both read. Keep
			// whichever carries more: a package node with a doc comment beats
			// one without, and neither analyzer is more right about the name.
			if existing.Doc == "" && n.Doc != "" {
				existing.Doc = n.Doc
			}
			nodes[n.ID] = existing
		}
		for _, e := range g.Edges {
			key := string(e.Kind) + "\x00" + e.From + "\x00" + e.To
			if seen, ok := edges[key]; ok {
				seen.Weight += e.Weight
				continue
			}
			copied := e
			edges[key] = &copied
		}
		merged.FilesParsed += g.Diagnostics.FilesParsed
		merged.ParseFailures = append(merged.ParseFailures, g.Diagnostics.ParseFailures...)
		for _, u := range g.Diagnostics.UnresolvedReferences {
			unresolved[u.Reason] += u.Count
		}
	}

	// A directory two languages wrote into belongs to neither of them. Saying
	// it is Python because Python was read first would be a claim nobody
	// checked, and the language is there to be trusted.
	for id, langs := range contributors {
		if len(langs) > 1 {
			n := nodes[id]
			n.Language = ""
			nodes[id] = n
		}
	}

	// Slices are made empty rather than left nil throughout. A nil slice
	// encodes as null and the schema wants an array, and the case that produces
	// the nil is always the ordinary one: a tree with no edges, a merge with no
	// failures. This is the third time that has cost a debugging session, so
	// TestEmptyOutputsAreStillArrays now watches the whole class.
	out := &graph.Graph{
		SchemaVersion: present[0].SchemaVersion,
		GeneratedBy:   present[0].GeneratedBy,
		Root:          present[0].Root,
		Nodes:         make([]graph.Node, 0, len(nodes)),
		Edges:         make([]graph.Edge, 0, len(edges)),
		Diagnostics:   merged,
	}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, n)
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })

	for _, e := range edges {
		if _, ok := nodes[e.From]; !ok {
			continue
		}
		if _, ok := nodes[e.To]; !ok {
			continue
		}
		out.Edges = append(out.Edges, *e)
	}
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].From != out.Edges[j].From {
			return out.Edges[i].From < out.Edges[j].From
		}
		if out.Edges[i].To != out.Edges[j].To {
			return out.Edges[i].To < out.Edges[j].To
		}
		return out.Edges[i].Kind < out.Edges[j].Kind
	})

	sort.Slice(out.Diagnostics.ParseFailures, func(i, j int) bool {
		return out.Diagnostics.ParseFailures[i].Path < out.Diagnostics.ParseFailures[j].Path
	})
	for reason, count := range unresolved {
		out.Diagnostics.UnresolvedReferences = append(out.Diagnostics.UnresolvedReferences,
			graph.UnresolvedReference{Reason: reason, Count: count})
	}
	sort.Slice(out.Diagnostics.UnresolvedReferences, func(i, j int) bool {
		return out.Diagnostics.UnresolvedReferences[i].Reason < out.Diagnostics.UnresolvedReferences[j].Reason
	})
	out.GeneratedBy = "diagrammer"
	return out, nil
}
