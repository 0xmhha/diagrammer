package analyze

import (
	"context"
	"sort"

	"github.com/0xmhha/diagrammer/internal/graph"
)

// Options tunes a walk. The zero value reads everything under the root.
//
// They are shared rather than per-analyzer, because a caller choosing between
// languages should not have to learn a different vocabulary for each of them.
type Options struct {
	// IncludeTests reads a language's test files too. They are skipped by
	// default: test code describes how something is exercised rather than what
	// it is.
	IncludeTests bool
	// MaxDepth limits how far below the root to descend. Zero is unlimited.
	MaxDepth int
	// Exclude lists root-relative directory prefixes to skip.
	Exclude []string
}

// Analyzer reads one language's source into a stage-1 code graph.
//
// The contract is narrow on purpose. An analyzer decides what its language
// means; it does not decide what a graph is. Every one of them emits the same
// five node kinds and two edge kinds, so nothing downstream has to know which
// language it is looking at.
type Analyzer interface {
	// Language names what this analyzer reads.
	Language() graph.Language

	// Extensions lists the file suffixes it claims, each with its leading dot.
	// It is what lets a caller route a mixed tree without asking every analyzer
	// to walk all of it.
	Extensions() []string

	// Analyze walks root and returns what it could prove, including the group
	// and package hierarchy above the declarations.
	//
	// It must honour cancellation, because a walk over a large tree is the one
	// place this program can be left running with nothing to show for it.
	//
	// A file that will not parse is recorded in the graph's diagnostics, never
	// dropped in silence. A graph missing a file looks exactly like a graph of
	// a project that never had one, and no later stage can tell the difference.
	Analyze(ctx context.Context, root string, opts Options) (*graph.Graph, error)
}

// Registry holds the analyzers a build knows about.
//
// It is a value rather than a package-level variable, so a caller can assemble
// the set it wants and a test can assemble a different one. A global register
// would make the set depend on which packages happened to be linked.
type Registry struct {
	byLanguage map[graph.Language]Analyzer
}

// NewRegistry indexes analyzers by language.
func NewRegistry(analyzers ...Analyzer) *Registry {
	r := &Registry{byLanguage: make(map[graph.Language]Analyzer, len(analyzers))}
	for _, a := range analyzers {
		r.byLanguage[a.Language()] = a
	}
	return r
}

// For returns the analyzer that reads a language.
func (r *Registry) For(language graph.Language) (Analyzer, bool) {
	a, ok := r.byLanguage[language]
	return a, ok
}

// Languages lists what this build can read, in a stable order.
//
// Callers report it rather than assuming: a build that reads one language
// should say so plainly, not leave someone to discover it by pointing the
// program at a repository it will quietly return almost nothing for.
func (r *Registry) Languages() []graph.Language {
	out := make([]graph.Language, 0, len(r.byLanguage))
	for language := range r.byLanguage {
		out = append(out, language)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
