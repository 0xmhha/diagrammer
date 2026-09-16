package invariant_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// The rules the other three families add to the shared ones each get a document
// that breaks them, for the same reason the component rules do: a rule never
// seen to fire cannot be told apart from one that was never written.

func orderService(t *testing.T) *uml.Model {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "codegraph", "order-service.codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("fixture does not validate: %v", err)
	}
	return model
}

type familyCase struct {
	name   string
	rule   string
	family uml.Family
	build  func(string, *uml.Model) (*diagram.Document, error)
	check  func(*uml.Model, *diagram.Document) []invariant.Problem
	damage func(t *testing.T, doc *diagram.Document)
}

func familyCases() []familyCase {
	seq := func(name, rule string, damage func(*testing.T, *diagram.Document)) familyCase {
		return familyCase{name, rule, uml.FamilySequence, compose.Sequence, invariant.Sequence, damage}
	}
	st := func(name, rule string, damage func(*testing.T, *diagram.Document)) familyCase {
		return familyCase{name, rule, uml.FamilyState, compose.State, invariant.State, damage}
	}
	uc := func(name, rule string, damage func(*testing.T, *diagram.Document)) familyCase {
		return familyCase{name, rule, uml.FamilyUsecase, compose.Usecase, invariant.Usecase, damage}
	}

	return []familyCase{
		seq("a message is moved to the wrong rung", invariant.RuleOrdering, func(t *testing.T, doc *diagram.Document) {
			row := 99
			doc.Levels[0].Connections[0].Order = &row
		}),
		seq("a message carries no row at all", invariant.RuleOrdering, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections[0].Order = nil
		}),
		seq("an activation ends before it starts", invariant.RuleOrdering, func(t *testing.T, doc *diagram.Document) {
			if len(doc.Levels[0].Activations) == 0 {
				t.Fatal("the fixture has no activation to damage")
			}
			doc.Levels[0].Activations[0].FromRow, doc.Levels[0].Activations[0].ToRow =
				doc.Levels[0].Activations[0].ToRow, doc.Levels[0].Activations[0].FromRow
			doc.Levels[0].Activations[0].FromRow++
		}),
		seq("an activation is left out", invariant.RuleCompleteness, func(t *testing.T, doc *diagram.Document) {
			if len(doc.Levels[0].Activations) == 0 {
				t.Fatal("the fixture has no activation to remove")
			}
			doc.Levels[0].Activations = doc.Levels[0].Activations[1:]
			doc.Levels[0].Accounting.Drawn--
			doc.Levels[0].Accounting.Proven--
			doc.Accounting.Drawn--
			doc.Accounting.Proven--
		}),
		seq("a message is dropped, which a ladder has no way to do", invariant.RuleNoDrops, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections = doc.Levels[0].Connections[1:]
			doc.Levels[0].Boxes[0].Dropped = []diagram.DroppedRelationship{{
				To: doc.Levels[0].Boxes[0].ID, Kind: diagram.KindMessage, Reason: diagram.ReasonSelfReference,
			}}
			doc.Levels[0].Accounting.Drawn--
			doc.Levels[0].Accounting.Dropped++
			doc.Accounting.Drawn--
			doc.Accounting.Dropped++
		}),
		seq("a connection is not a message", invariant.RuleKind, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections[0].Kind = diagram.KindDependency
		}),

		st("a state is left out", invariant.RuleCompleteness, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Boxes = doc.Levels[0].Boxes[1:]
		}),
		st("a transition is left out", invariant.RuleCompleteness, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections = doc.Levels[0].Connections[1:]
			doc.Levels[0].Accounting.Drawn--
			doc.Levels[0].Accounting.Proven--
			doc.Accounting.Drawn--
			doc.Accounting.Proven--
		}),
		st("a final state is drawn as an ordinary one", invariant.RuleCompleteness, func(t *testing.T, doc *diagram.Document) {
			for i := range doc.Levels[0].Boxes {
				if doc.Levels[0].Boxes[i].Stereotype == string(uml.StateFinal) {
					doc.Levels[0].Boxes[i].Stereotype = string(uml.StateSimple)
					return
				}
			}
			t.Fatal("the fixture declares no final state")
		}),
		st("a connection is not a transition", invariant.RuleKind, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections[0].Kind = diagram.KindMessage
		}),

		uc("a use case is drawn outside the system", invariant.RuleBoundary, func(t *testing.T, doc *diagram.Document) {
			for i := range doc.Levels[0].Boxes {
				if doc.Levels[0].Boxes[i].Region != "" {
					doc.Levels[0].Boxes[i].Region = ""
					return
				}
			}
			t.Fatal("the fixture places nothing inside the system")
		}),
		uc("an actor is drawn inside the system", invariant.RuleBoundary, func(t *testing.T, doc *diagram.Document) {
			for i := range doc.Levels[0].Boxes {
				if doc.Levels[0].Boxes[i].Region == "" {
					doc.Levels[0].Boxes[i].Region = doc.Levels[0].Regions[0].ID
					return
				}
			}
			t.Fatal("the fixture places nothing outside the system")
		}),
		uc("an include ends on an actor", invariant.RuleBoundary, func(t *testing.T, doc *diagram.Document) {
			var actor string
			for _, b := range doc.Levels[0].Boxes {
				if b.Region == "" {
					actor = b.ID
					break
				}
			}
			for i := range doc.Levels[0].Connections {
				if doc.Levels[0].Connections[i].Kind == diagram.KindInclude {
					doc.Levels[0].Connections[i].To = actor
					return
				}
			}
			t.Fatal("the fixture has no include to damage")
		}),
		uc("an association is left out", invariant.RuleCompleteness, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections = doc.Levels[0].Connections[1:]
			doc.Levels[0].Accounting.Drawn--
			doc.Levels[0].Accounting.Proven--
			doc.Accounting.Drawn--
			doc.Accounting.Proven--
		}),
		uc("a connection is a dependency", invariant.RuleKind, func(t *testing.T, doc *diagram.Document) {
			doc.Levels[0].Connections[0].Kind = diagram.KindDependency
		}),
	}
}

func TestUndamagedFamilyDocumentsPass(t *testing.T) {
	model := orderService(t)
	for _, c := range familyCases() {
		t.Run(string(c.family), func(t *testing.T) {
			doc, err := c.build("test", model)
			if err != nil {
				t.Fatalf("compose: %v", err)
			}
			if problems := c.check(model, doc); len(problems) > 0 {
				t.Fatalf("a correct document reported %d problems: %v", len(problems), problems)
			}
		})
	}
}

func TestEveryFamilyRuleFires(t *testing.T) {
	for _, c := range familyCases() {
		t.Run(string(c.family)+": "+c.name, func(t *testing.T) {
			model := orderService(t)
			doc, err := c.build("test", model)
			if err != nil {
				t.Fatalf("compose: %v", err)
			}
			c.damage(t, doc)

			problems := c.check(model, doc)
			if len(problems) == 0 {
				t.Fatalf("the damaged document passed, so %q never fires", c.rule)
			}
			for _, p := range problems {
				if p.Rule == c.rule {
					return
				}
			}
			var fired []string
			for _, p := range problems {
				fired = append(fired, p.Rule)
			}
			t.Errorf("want rule %q to fire, got %s", c.rule, strings.Join(fired, ", "))
		})
	}
}

// TestEveryFamilyRuleHasACase keeps the table honest, the same way the component
// one does. The covered set is read from the table rather than written out
// again, so the two cannot drift.
func TestEveryFamilyRuleHasACase(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range familyCases() {
		covered[c.rule] = true
	}
	for _, rule := range []string{
		invariant.RuleNoDrops,
		invariant.RuleOrdering,
		invariant.RuleBoundary,
		invariant.RuleKind,
	} {
		if !covered[rule] {
			t.Errorf("rule %q has no case that fires it", rule)
		}
	}
}
