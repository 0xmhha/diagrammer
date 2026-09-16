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

// Every rule below has a document that breaks it. A rule that is never seen to
// fire cannot be told apart from a rule that was never written, and these are
// the only thing standing between a diagram and a quiet lie about what it
// shows.
//
// The documents are built by composing a real fixture and then damaging it in
// one specific way, so each case differs from a correct document in exactly the
// respect its rule is about.

func good(t *testing.T) (*uml.Model, *diagram.Document) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "codegraph", "nested-platform.codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("fixture does not validate: %v", err)
	}
	doc, err := compose.Component(path, model)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	return model, doc
}

// levelIndex finds a level by id, so a case can damage a named page rather than
// whichever one happens to be first.
func levelIndex(t *testing.T, doc *diagram.Document, id string) int {
	t.Helper()
	for i, l := range doc.Levels {
		if l.ID == id {
			return i
		}
	}
	t.Fatalf("the fixture no longer has a level called %q", id)
	return -1
}

func TestUndamagedDocumentPasses(t *testing.T) {
	model, doc := good(t)
	if problems := invariant.Component(model, doc); len(problems) > 0 {
		t.Fatalf("a correct document reported %d problems: %v", len(problems), problems)
	}
}

// damageCase is one way of breaking a correct document, and the rule that must
// notice.
type damageCase struct {
	name   string
	rule   string
	damage func(t *testing.T, model *uml.Model, doc *diagram.Document)
}

// cases is the table both tests below read, so the coverage check cannot drift
// from the cases it is checking.
func cases() []damageCase {
	return []damageCase{
		{
			name: "a dropped relationship is left out of the accounting",
			rule: invariant.RuleAccounting,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				doc.Levels[i].Accounting.Dropped--
				doc.Accounting.Dropped--
			},
		}, {
			name: "the document totals disagree with its levels",
			rule: invariant.RuleAccounting,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				doc.Accounting.Proven++
				doc.Accounting.Drawn++
			},
		}, {
			name: "a connection is removed but still counted as drawn",
			rule: invariant.RuleRecordMatch,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				doc.Levels[i].Connections = doc.Levels[i].Connections[1:]
			},
		}, {
			name: "a box's record is erased but still counted as dropped",
			rule: invariant.RuleRecordMatch,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				for j := range doc.Levels[i].Boxes {
					if len(doc.Levels[i].Boxes[j].Dropped) > 0 {
						doc.Levels[i].Boxes[j].Dropped = nil
						return
					}
				}
				t.Fatal("the fixture records nothing on any box, so this case proves nothing")
			},
		}, {
			name: "a connection ends at a box that is not on its page",
			rule: invariant.RuleEndpoints,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				doc.Levels[i].Connections[0].To = "somewhere-else"
			},
		}, {
			name: "two boxes share a cell",
			rule: invariant.RulePlacement,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				doc.Levels[i].Boxes[1].Row = doc.Levels[i].Boxes[0].Row
				doc.Levels[i].Boxes[1].Col = doc.Levels[i].Boxes[0].Col
			},
		}, {
			name: "a box sits outside the grid it is on",
			rule: invariant.RulePlacement,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				doc.Levels[i].Boxes[0].Col = doc.Levels[i].Grid.Cols + 1
			},
		}, {
			name: "a page names a parent that is not a level",
			rule: invariant.RuleDrilldown,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "level:web")
				doc.Levels[i].Parent = "no-such-level"
			},
		}, {
			name: "a page is opened from a box that does not open it",
			rule: invariant.RuleDrilldown,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "level:web")
				parent := levelIndex(t, doc, doc.Levels[i].Parent)
				for j := range doc.Levels[parent].Boxes {
					if doc.Levels[parent].Boxes[j].ID == doc.Levels[i].OpensFrom {
						doc.Levels[parent].Boxes[j].Opens = "somewhere-else"
						return
					}
				}
				t.Fatal("the fixture's drill-down no longer has an opener to damage")
			},
		}, {
			name: "there is no overview",
			rule: invariant.RuleDrilldown,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "overview")
				doc.Levels[i].Parent = "level:web"
				doc.Levels[i].OpensFrom = "web.ui"
			},
		}, {
			name: "a component appears on two pages",
			rule: invariant.RuleCompleteness,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "level:web")
				j := levelIndex(t, doc, "level:data")
				doc.Levels[j].Boxes = append(doc.Levels[j].Boxes, doc.Levels[i].Boxes[0])
			},
		}, {
			name: "a component appears on no page at all",
			rule: invariant.RuleCompleteness,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				i := levelIndex(t, doc, "level:data")
				doc.Levels[i].Boxes = doc.Levels[i].Boxes[1:]
			},
		}, {
			name: "the document is not a component diagram",
			rule: invariant.RuleCompleteness,
			damage: func(t *testing.T, _ *uml.Model, doc *diagram.Document) {
				doc.Family = "sequence"
			},
		},
	}
}

func TestEveryRuleFires(t *testing.T) {
	for _, c := range cases() {
		t.Run(c.name, func(t *testing.T) {
			model, doc := good(t)
			c.damage(t, model, doc)

			problems := invariant.Component(model, doc)
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

// TestEveryRuleHasACase keeps the table honest.
//
// A rule added to the checker without a case that fires it would otherwise sit
// there untested, which is the exact situation these tests exist to prevent.
// The covered set is read from the table rather than written out again, so the
// two cannot drift apart.
func TestEveryRuleHasACase(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range cases() {
		covered[c.rule] = true
	}
	for _, rule := range []string{
		invariant.RuleAccounting,
		invariant.RuleRecordMatch,
		invariant.RuleEndpoints,
		invariant.RulePlacement,
		invariant.RuleDrilldown,
		invariant.RuleCompleteness,
	} {
		if !covered[rule] {
			t.Errorf("rule %q has no case that fires it", rule)
		}
	}
}
