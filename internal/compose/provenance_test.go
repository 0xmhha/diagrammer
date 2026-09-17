package compose_test

import (
	"testing"

	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/vcs"
)

// The commit a drawing describes is written down once, by the stage that read
// the source, and copied by every stage after it. compose is the middle of that
// chain and the only place it can be dropped without anything failing, because
// a document with no revision is a valid document.
//
// So it is tested here per family rather than once. Four composers build the
// provenance and a fifth family would build a fifth, which is why they share
// one function; these hold that sharing to its purpose.

func TestEveryFamilyCarriesTheRevisionForward(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.name)+"/"+f.fixture, func(t *testing.T) {
			model, doc := composeFamily(t, f)

			var want *vcs.Revision
			if model.Provenance.SourceGraph != nil {
				want = model.Provenance.SourceGraph.Revision
			}
			if want == nil {
				if doc.Provenance.Revision != nil {
					t.Fatalf("the model states no revision and the document claims %q",
						doc.Provenance.Revision.Commit)
				}
				t.Skip("this fixture carries no revision, which the case below covers")
			}

			got := doc.Provenance.Revision
			if got == nil {
				t.Fatalf("the model states commit %s and the document carries none", want.Commit)
			}
			if got.Commit != want.Commit || got.Ref != want.Ref {
				t.Errorf("the document carries %+v, the model stated %+v", *got, *want)
			}
		})
	}
}

// TestAModelWithNoRevisionProducesADocumentClaimingNone is the other half, and
// the more important one. An invented commit is worse than an absent one:
// nothing downstream can tell a wrong commit from a right one, so a document
// that fills the gap in would be believed.
func TestAModelWithNoRevisionProducesADocumentClaimingNone(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.name)+"/"+f.fixture, func(t *testing.T) {
			model, path := load(t, f.fixture)
			model.Provenance.SourceGraph = nil

			doc, err := f.build(path, model)
			if err != nil {
				t.Fatalf("compose %s: %v", f.name, err)
			}
			if doc.Provenance.Revision != nil {
				t.Errorf("no revision was stated and the document claims %q",
					doc.Provenance.Revision.Commit)
			}
		})
	}
}

// TestAGraphNamedWithoutARevisionIsNotOne covers the shape between the two: a
// model that names the graph it read but records no commit, which is what a
// model of a tree that was never in a checkout looks like.
func TestAGraphNamedWithoutARevisionIsNotOne(t *testing.T) {
	f := families()[0]
	model, path := load(t, f.fixture)
	model.Provenance.SourceGraph = &uml.SourceGraph{Path: "somewhere.graph.json"}

	doc, err := f.build(path, model)
	if err != nil {
		t.Fatalf("compose %s: %v", f.name, err)
	}
	if doc.Provenance.Revision != nil {
		t.Errorf("a graph was named and a commit was invented: %q", doc.Provenance.Revision.Commit)
	}
}
