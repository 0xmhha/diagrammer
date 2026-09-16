package compose_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// fixtures are the committed models stage 3 is held to. order-service is flat
// and covers the vocabulary; nested-platform is shaped so the record and region
// paths are exercised rather than merely present.
var fixtures = []string{"order-service", "nested-platform"}

func load(t *testing.T, name string) (*uml.Model, string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "codegraph", name+".codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("%s does not validate: %v", name, err)
	}
	return model, path
}

func composeFixture(t *testing.T, name string) (*uml.Model, *diagram.Document) {
	t.Helper()
	model, path := load(t, name)
	doc, err := compose.Component(path, model)
	if err != nil {
		t.Fatalf("compose %s: %v", name, err)
	}
	return model, doc
}

func encode(t *testing.T, doc *diagram.Document) []byte {
	t.Helper()
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b
}

func TestComposedDocumentSatisfiesItsSchema(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			_, doc := composeFixture(t, name)
			if err := schema.Validate(schema.Diagram, encode(t, doc)); err != nil {
				t.Errorf("document does not satisfy the schema: %v", err)
			}
		})
	}
}

// TestCompletenessHoldsOnEveryFixture is the rule the family exists to keep:
// every relationship the model proves is drawn or recorded, and nothing the
// model declared is silently discarded.
func TestCompletenessHoldsOnEveryFixture(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			model, doc := composeFixture(t, name)
			for _, p := range invariant.Component(model, doc) {
				t.Errorf("%s", p)
			}
		})
	}
}

func TestCompositionIsByteIdenticalAcrossRuns(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			_, first := composeFixture(t, name)
			want := string(encode(t, first))
			for i := range 4 {
				_, again := composeFixture(t, name)
				if string(encode(t, again)) != want {
					t.Fatalf("run %d differs from the first", i+2)
				}
			}
		})
	}
}

// TestDenseFixtureExercisesTheRecordPath is what keeps the accounting honest.
//
// A document where nothing is ever dropped satisfies the invariant trivially,
// so a fixture that never drops anything cannot tell a working record from one
// that was never written.
func TestDenseFixtureExercisesTheRecordPath(t *testing.T) {
	_, doc := composeFixture(t, "nested-platform")

	if doc.Accounting.Dropped == 0 {
		t.Fatal("the dense fixture drops nothing, so the record path is never taken")
	}

	recorded := 0
	regions := 0
	drilldowns := 0
	for _, l := range doc.Levels {
		regions += len(l.Regions)
		if l.Parent != "" {
			drilldowns++
		}
		for _, b := range l.Boxes {
			recorded += len(b.Dropped)
			for _, d := range b.Dropped {
				if d.Reason != diagram.ReasonSelfReference {
					t.Errorf("unexpected drop reason %q", d.Reason)
				}
			}
		}
	}
	if recorded != doc.Accounting.Dropped {
		t.Errorf("%d dropped by the accounting, %d recorded on boxes", doc.Accounting.Dropped, recorded)
	}
	if regions == 0 {
		t.Error("no region band, so the unfold path is never taken")
	}
	if drilldowns == 0 {
		t.Error("no drill-down level, so multi-level placement is never taken")
	}
}

// TestAssemblyIsDerived proves the connector UML implies but the model does not
// write down is actually built: one component requires an interface, another
// provides it, and a relationship appears between them.
func TestAssemblyIsDerived(t *testing.T) {
	_, doc := composeFixture(t, "nested-platform")
	for _, l := range doc.Levels {
		for _, c := range l.Connections {
			if c.Kind != diagram.KindAssembly {
				continue
			}
			if c.Interface == "" {
				t.Errorf("%s: an assembly connector names no interface", c.ID)
			}
			return
		}
	}
	t.Error("no assembly connector was derived, though the fixture has a required and a provided port for one interface")
}

// TestRefusesAModelWithoutTheFamily keeps compose from inventing a diagram for
// a model that never claimed to support one.
func TestRefusesAModelWithoutTheFamily(t *testing.T) {
	model := &uml.Model{Families: []uml.Family{uml.FamilySequence}}
	if _, err := compose.Component("test", model); err == nil {
		t.Error("want a refusal, got a document")
	}
}

// TestALevelThatDrawsNothingIsStillAnArray is about JSON rather than geometry.
//
// A container whose children have no relationships among them is an ordinary
// page that happens to draw no lines. Built as a nil slice, its connections
// encode as null, and the schema refuses null where it wants an array. Nothing
// caught it until a model with such a level came along, because every fixture
// until then drew something on every page.
func TestALevelThatDrawsNothingIsStillAnArray(t *testing.T) {
	model := &uml.Model{
		SchemaVersion: 1,
		Meta:          uml.Meta{Title: "t"},
		Provenance:    uml.Provenance{Origin: uml.OriginHandwritten},
		Families:      []uml.Family{uml.FamilyComponent},
		Component: &uml.ComponentModel{
			Components: []uml.Component{
				{ID: "box", Name: "Box"},
				{ID: "holder", Name: "Holder"},
				// Two children with nothing between them, so the page inside
				// Holder has boxes and no connections at all.
				{ID: "a", Name: "A", Parent: "holder"},
				{ID: "b", Name: "B", Parent: "holder"},
			},
			Dependencies: []uml.Dependency{
				{ID: "d", From: "box", To: "holder"},
			},
		},
	}

	doc, err := compose.Component("test", model)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}

	empty := 0
	for _, l := range doc.Levels {
		if len(l.Connections) == 0 {
			empty++
			if l.Connections == nil {
				t.Errorf("%s draws nothing and carries a nil slice, which encodes as null", l.ID)
			}
		}
	}
	if empty == 0 {
		t.Fatal("no level drew nothing, so this test proves nothing")
	}

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(encoded), `"connections": null`) {
		t.Error("the document carries a null where the schema wants an array")
	}
	if err := schema.Validate(schema.Diagram, append(encoded, '\n')); err != nil {
		t.Errorf("the document does not satisfy the schema: %v", err)
	}
}
