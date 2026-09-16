package compose_test

import (
	"testing"

	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// family ties a composer to the rule its output is held to, so every test below
// covers all four without any of them being written out four times.
type family struct {
	name    uml.Family
	build   func(string, *uml.Model) (*diagram.Document, error)
	check   func(*uml.Model, *diagram.Document) []invariant.Problem
	fixture string
}

func families() []family {
	return []family{
		{uml.FamilyComponent, compose.Component, invariant.Component, "nested-platform"},
		{uml.FamilyComponent, compose.Component, invariant.Component, "order-service"},
		{uml.FamilySequence, compose.Sequence, invariant.Sequence, "order-service"},
		{uml.FamilyState, compose.State, invariant.State, "order-service"},
		{uml.FamilyUsecase, compose.Usecase, invariant.Usecase, "order-service"},
	}
}

func composeFamily(t *testing.T, f family) (*uml.Model, *diagram.Document) {
	t.Helper()
	model, path := load(t, f.fixture)
	doc, err := f.build(path, model)
	if err != nil {
		t.Fatalf("compose %s from %s: %v", f.name, f.fixture, err)
	}
	return model, doc
}

func TestEveryFamilySatisfiesItsSchema(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.name)+" from "+f.fixture, func(t *testing.T) {
			_, doc := composeFamily(t, f)
			if err := schema.Validate(schema.Diagram, encode(t, doc)); err != nil {
				t.Errorf("document does not satisfy the schema: %v", err)
			}
		})
	}
}

func TestEveryFamilyIsComplete(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.name)+" from "+f.fixture, func(t *testing.T) {
			model, doc := composeFamily(t, f)
			for _, p := range f.check(model, doc) {
				t.Errorf("%s", p)
			}
		})
	}
}

func TestEveryFamilyIsByteIdenticalAcrossRuns(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.name)+" from "+f.fixture, func(t *testing.T) {
			_, first := composeFamily(t, f)
			want := string(encode(t, first))
			for i := range 4 {
				_, again := composeFamily(t, f)
				if string(encode(t, again)) != want {
					t.Fatalf("run %d differs from the first", i+2)
				}
			}
		})
	}
}

// TestEveryFamilyRefusesAModelWithoutIt keeps a composer from inventing a
// diagram for a model that never claimed to support one.
func TestEveryFamilyRefusesAModelWithoutIt(t *testing.T) {
	empty := &uml.Model{}
	for _, f := range families() {
		t.Run(string(f.name), func(t *testing.T) {
			if _, err := f.build("test", empty); err == nil {
				t.Error("want a refusal, got a document")
			}
		})
	}
}

// TestSequenceKeepsTheLadderInOrder is the sequence family's own concern: the
// same messages in a different order describe a different interaction, so the
// row each one lands on is not a presentation detail.
func TestSequenceKeepsTheLadderInOrder(t *testing.T) {
	model, doc := composeFamily(t, family{uml.FamilySequence, compose.Sequence, invariant.Sequence, "order-service"})

	rows := map[string]int{}
	for _, l := range doc.Levels {
		for _, c := range l.Connections {
			if c.Order == nil {
				t.Fatalf("%s carries no row", c.ID)
			}
			rows[c.ID] = *c.Order
		}
	}
	for i, msg := range model.Sequence.Messages {
		got, ok := rows[msg.ID]
		if !ok {
			t.Errorf("message %q reached no level", msg.ID)
			continue
		}
		if got != i {
			t.Errorf("message %q is at row %d, but the model puts it at %d", msg.ID, got, i)
		}
	}

	// The first message sits at row zero, which a plainly encoded integer would
	// erase on the way out. This is the case that catches that.
	if rows[model.Sequence.Messages[0].ID] != 0 {
		t.Error("the first message did not survive with row zero")
	}
}

// TestSequenceFramesItsFragment checks the frame reaches as wide as the
// messages it holds. A frame narrower than its contents cuts through a line it
// is supposed to contain.
func TestSequenceFramesItsFragment(t *testing.T) {
	_, doc := composeFamily(t, family{uml.FamilySequence, compose.Sequence, invariant.Sequence, "order-service"})
	found := 0
	for _, l := range doc.Levels {
		for _, f := range l.Fragments {
			found++
			if f.ToRow < f.FromRow {
				t.Errorf("%s ends at row %d, before it starts at %d", f.ID, f.ToRow, f.FromRow)
			}
			if f.ToCol < f.FromCol {
				t.Errorf("%s ends at column %d, before it starts at %d", f.ID, f.ToCol, f.FromCol)
			}
			if len(f.Operands) == 0 {
				t.Errorf("%s has no operand", f.ID)
			}
		}
	}
	if found == 0 {
		t.Error("the fixture produced no combined fragment, so the frame path is never taken")
	}
}

// TestStateDrawsACompositeAsBothBoxAndBand is the shape that looks like a
// duplicate and is not: in UML a composite state's own box is the frame its
// substates sit inside, and a transition has to be able to land on it.
func TestStateDrawsACompositeAsBothBoxAndBand(t *testing.T) {
	_, doc := composeFamily(t, family{uml.FamilyState, compose.State, invariant.State, "order-service"})

	for _, l := range doc.Levels {
		for _, region := range l.Regions {
			if _, ok := boxWithID(l, region.ID); !ok {
				t.Errorf("band %q has no box, so a transition to it would have nowhere to land", region.ID)
			}
			framed := 0
			for _, b := range l.Boxes {
				if b.Region == region.ID {
					framed++
				}
			}
			if framed == 0 {
				t.Errorf("band %q frames nothing", region.ID)
			}
		}
		if len(l.Regions) == 0 {
			t.Error("the fixture produced no composite state, so the band path is never taken")
		}
	}
}

// TestUsecaseKeepsTheBoundary is the use case family's own concern: where a box
// sits is the model's claim about which side of the system it is on.
func TestUsecaseKeepsTheBoundary(t *testing.T) {
	model, doc := composeFamily(t, family{uml.FamilyUsecase, compose.Usecase, invariant.Usecase, "order-service"})
	system := model.Usecase.System.ID

	inside := map[string]bool{}
	for _, u := range model.Usecase.Usecases {
		inside[u.ID] = true
	}
	for _, l := range doc.Levels {
		for _, b := range l.Boxes {
			if inside[b.ID] && b.Region != system {
				t.Errorf("use case %q is drawn outside the system", b.ID)
			}
			if !inside[b.ID] && b.Region != "" {
				t.Errorf("actor %q is drawn inside %q", b.ID, b.Region)
			}
		}
	}
}

func boxWithID(l diagram.Level, id string) (diagram.Box, bool) {
	for _, b := range l.Boxes {
		if b.ID == id {
			return b, true
		}
	}
	return diagram.Box{}, false
}
