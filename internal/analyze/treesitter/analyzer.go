//go:build cgo

package treesitter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/graph"
)

const (
	rootNodeID  = "root"
	generatedBy = "diagrammer treesitter"
	// maxWarnings caps the problems that are not parse failures. One unreadable
	// tree can produce an unbounded number of them, and a parse failure is
	// never capped because that is the one thing that must not be lost.
	maxWarnings = 50
)

// skippedDirs are never descended into.
var skippedDirs = map[string]bool{
	"vendor": true, "testdata": true, "node_modules": true, ".git": true,
	"__pycache__": true, "dist": true, "build": true,
}

// Analyzers returns one analyzer per language this build can read.
func Analyzers() []analyze.Analyzer {
	out := make([]analyze.Analyzer, 0, len(languages()))
	for _, l := range languages() {
		out = append(out, Analyzer{lang: l})
	}
	return out
}

// Analyzer reads one language.
type Analyzer struct{ lang language }

func (a Analyzer) Language() graph.Language { return a.lang.name }

func (a Analyzer) Extensions() []string {
	out := append([]string(nil), a.lang.extensions...)
	sort.Strings(out)
	return out
}

// Analyze walks root and returns what the grammar could see.
func (a Analyzer) Analyze(ctx context.Context, root string, opts analyze.Options) (*graph.Graph, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("read root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read root %s: not a directory", root)
	}

	w := &walker{
		lang:     a.lang,
		root:     abs,
		opts:     opts,
		nodes:    map[string]graph.Node{},
		edges:    map[string]*graph.Edge{},
		declsIn:  map[string][]declaration{},
		byName:   map[string][]string{},
		excluded: normalizeExcludes(opts.Exclude),
	}
	if err := w.collect(ctx); err != nil {
		return nil, err
	}
	w.resolveImports()
	w.resolveCalls()
	return w.build(), nil
}

func normalizeExcludes(raw []string) []string {
	var out []string
	for _, part := range raw {
		if cleaned := strings.Trim(strings.TrimSpace(filepath.ToSlash(part)), "/"); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

// pendingCall is one invocation, before anything tries to say which
// declaration it meant.
type pendingCall struct {
	from string
	name string
}

// pendingImport is one import, before the walk knows whether its target is in
// this tree.
type pendingImport struct {
	fromDir string
	target  string
}

// declaration is one thing the grammar named.
type declaration struct {
	id       string
	kind     graph.NodeKind
	name     string
	file     string
	line     int
	lines    int
	doc      string
	exported bool
}

type walker struct {
	lang     language
	root     string
	opts     analyze.Options
	excluded []string

	nodes   map[string]graph.Node
	edges   map[string]*graph.Edge
	declsIn map[string][]declaration // key: package directory
	// byName indexes declarations by their plain name, which is as much as a
	// grammar without type resolution can offer a call.
	byName map[string][]string

	pendingCalls   []pendingCall
	pendingImports []pendingImport
	filesRead      int
	parseFailures  []graph.ParseFailure
	unresolved     map[string]int
	warnings       []string
}
