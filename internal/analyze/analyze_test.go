package analyze_test

import (
	"testing"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/analyze/goast"
	"github.com/0xmhha/diagrammer/internal/graph"
)

// The interface exists so that adding a language is an addition rather than a
// redesign. That claim is worth nothing unless something holds the one
// implementation to it, so this does.
var _ analyze.Analyzer = goast.Analyzer{}

func TestRegistry(t *testing.T) {
	r := analyze.NewRegistry(goast.Analyzer{})

	if _, ok := r.For(graph.Go); !ok {
		t.Error("the registry does not hold the Go analyzer")
	}
	// Every language the schema names that this build cannot read has to be
	// absent rather than silently returning an analyzer for something else.
	for _, language := range []graph.Language{graph.Python, graph.JavaScript, graph.TypeScript} {
		if _, ok := r.For(language); ok {
			t.Errorf("the registry claims to read %s, which this build cannot", language)
		}
	}

	got := r.Languages()
	if len(got) != 1 || got[0] != graph.Go {
		t.Errorf("this build reads %v, want just go", got)
	}
}

// TestEveryAnalyzerClaimsItsFiles keeps an analyzer from joining the registry
// without saying which files are its own.
func TestEveryAnalyzerClaimsItsFiles(t *testing.T) {
	for _, a := range []analyze.Analyzer{goast.Analyzer{}} {
		if a.Language() == "" {
			t.Errorf("%T names no language", a)
		}
		exts := a.Extensions()
		if len(exts) == 0 {
			t.Errorf("%T claims no file extensions", a)
		}
		for _, e := range exts {
			if len(e) < 2 || e[0] != '.' {
				t.Errorf("%T claims %q, which is not a suffix with a leading dot", a, e)
			}
		}
	}
}
