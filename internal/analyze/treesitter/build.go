//go:build cgo

package treesitter

import (
	"path"
	"sort"
	"strings"

	"github.com/0xmhha/diagrammer/internal/graph"
)

// resolveCalls turns invocations into edges, as far as a grammar without type
// resolution can.
//
// A name is matched against the declarations that carry it. One match becomes
// an edge; several means the name is ambiguous and nothing is claimed, because
// an edge to the wrong function is worse than no edge. None means the callee is
// outside what was read.
//
// This is weaker than what go/ast manages for Go, and deliberately so: stage 2
// attributes the meaning, and stage 1 only has to be honest about what it
// actually saw.
// resolveImports turns the imports the walk collected into edges, now that it
// knows which directories it actually read.
//
// A target that does not name one of them is outside the tree: the standard
// library, a package from a registry, a file in a sibling project. That is the
// boundary of what was read rather than a defect in the file, so it is counted
// and not drawn. An edge to a package nobody read would be a line to nowhere,
// and a reader could not tell it from a real one.
func (w *walker) resolveImports() {
	known := make(map[string]bool, len(w.declsIn))
	for dir := range w.declsIn {
		known[dir] = true
	}
	for _, imp := range w.pendingImports {
		to := w.resolveImport(imp.fromDir, imp.target)
		if to == "" {
			w.unresolved["import-outside-the-tree"]++
			continue
		}
		// The last segment of a module path may name a package or a file, and
		// the text does not say which: `from ..lib import x` names a directory,
		// `from ..lib.shared import x` names a file inside one. Both are tried
		// against what the walk actually read rather than guessed at from the
		// shape of the string.
		if !known[to] {
			if parent := path.Dir(to); known[parent] {
				to = parent
			} else {
				w.unresolved["import-outside-the-tree"]++
				continue
			}
		}
		if to == imp.fromDir {
			// A package importing itself is not an edge between packages. The
			// file-level relationship is real and the graph draws packages.
			continue
		}
		w.addEdge(packageID(imp.fromDir), packageID(to), graph.EdgeImport)
	}
}

func (w *walker) resolveCalls() {
	for _, call := range w.pendingCalls {
		candidates := w.byName[call.name]
		switch len(candidates) {
		case 0:
			w.unresolved["not-declared-here"]++
		case 1:
			if candidates[0] == call.from {
				w.unresolved["self-call"]++
				continue
			}
			w.addEdge(call.from, candidates[0], graph.EdgeCall)
		default:
			w.unresolved["ambiguous-name"]++
		}
	}
}

// build assembles the graph: the directory tree first so every declaration has
// a parent, then the files, then the declarations, then the edges whose
// endpoints both exist.
//
// Everything is sorted before it is returned, which is what makes the output
// byte-identical across runs.
func (w *walker) build() *graph.Graph {
	nodes := map[string]graph.Node{
		rootNodeID: {ID: rootNodeID, Kind: graph.KindGroup, Name: path.Base(w.root)},
	}

	dirs := make([]string, 0, len(w.declsIn))
	for dir := range w.declsIn {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	files := map[string]bool{}
	for _, dir := range dirs {
		w.ensureAncestors(nodes, dir)
		nodes[packageID(dir)] = graph.Node{
			ID:       packageID(dir),
			Kind:     graph.KindPackage,
			Name:     packageName(dir, w.root),
			Parent:   parentID(dir),
			Language: w.lang.name,
			Source:   &graph.SourceRef{Path: dir},
		}

		decls := append([]declaration(nil), w.declsIn[dir]...)
		sort.Slice(decls, func(i, j int) bool { return decls[i].id < decls[j].id })
		for _, decl := range decls {
			if !files[decl.file] {
				files[decl.file] = true
				nodes[fileID(decl.file)] = graph.Node{
					ID:       fileID(decl.file),
					Kind:     graph.KindFile,
					Name:     path.Base(decl.file),
					Parent:   packageID(path.Dir(decl.file)),
					Language: w.lang.name,
					Source:   &graph.SourceRef{Path: decl.file, File: decl.file},
				}
			}
			nodes[decl.id] = graph.Node{
				ID:       decl.id,
				Kind:     decl.kind,
				Name:     decl.name,
				Parent:   fileID(decl.file),
				Language: w.lang.name,
				Doc:      decl.doc,
				Exported: decl.exported,
				Source: &graph.SourceRef{
					Path: decl.file, Line: decl.line, Lines: decl.lines, File: decl.file,
				},
			}
		}
	}

	ordered := make([]graph.Node, 0, len(nodes))
	for _, n := range nodes {
		ordered = append(ordered, n)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	edges := make([]graph.Edge, 0, len(w.edges))
	for _, e := range w.edges {
		if _, ok := nodes[e.From]; !ok {
			continue
		}
		if _, ok := nodes[e.To]; !ok {
			// An import naming something outside what was read is not a defect
			// in the file; it is the boundary of what this walk covered.
			w.unresolved["outside-the-tree"]++
			continue
		}
		edges = append(edges, *e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})

	return &graph.Graph{
		SchemaVersion: 1,
		GeneratedBy:   generatedBy,
		Root:          rootNodeID,
		Nodes:         ordered,
		Edges:         edges,
		Diagnostics:   w.diagnostics(),
	}
}

func (w *walker) diagnostics() graph.Diagnostics {
	// Empty rather than nil, for the same reason everywhere else: null is not
	// an array, and a tree that parsed cleanly is the common case.
	failures := make([]graph.ParseFailure, 0, len(w.parseFailures))
	failures = append(failures, w.parseFailures...)
	sort.Slice(failures, func(i, j int) bool { return failures[i].Path < failures[j].Path })

	unresolved := make([]graph.UnresolvedReference, 0, len(w.unresolved))
	for reason, count := range w.unresolved {
		unresolved = append(unresolved, graph.UnresolvedReference{Reason: reason, Count: count})
	}
	sort.Slice(unresolved, func(i, j int) bool { return unresolved[i].Reason < unresolved[j].Reason })

	return graph.Diagnostics{
		FilesParsed:          w.filesRead,
		ParseFailures:        failures,
		UnresolvedReferences: unresolved,
	}
}

// ensureAncestors creates grouping nodes for directories that hold no source of
// their own, so every package can be reached from the root by walking parents.
func (w *walker) ensureAncestors(nodes map[string]graph.Node, dir string) {
	if dir == "." || dir == "" {
		return
	}
	segments := strings.Split(dir, "/")
	for i := 1; i < len(segments); i++ {
		prefix := strings.Join(segments[:i], "/")
		id := packageID(prefix)
		if _, ok := nodes[id]; ok {
			continue
		}
		if _, isPackage := w.declsIn[prefix]; isPackage {
			continue
		}
		nodes[id] = graph.Node{
			ID:     id,
			Kind:   graph.KindGroup,
			Name:   segments[i-1],
			Parent: parentID(prefix),
			Source: &graph.SourceRef{Path: prefix},
		}
	}
}

func parentID(dir string) string {
	if dir == "." || dir == "" {
		return ""
	}
	if i := strings.LastIndex(dir, "/"); i > 0 {
		return packageID(dir[:i])
	}
	return rootNodeID
}

func packageName(dir, root string) string {
	if dir == "." || dir == "" {
		return path.Base(root)
	}
	return path.Base(dir)
}
