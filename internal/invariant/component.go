// Package invariant holds the completeness rules each diagram family must obey.
//
// The general form is the same for all of them: every element of the model is
// either rendered or accounted for with a reason. What that means differs by
// family, and the rules are written per family rather than shared, because the
// component family's "drawn, or recorded on the box" is specific to a grid that
// cannot show everything at once. A sequence diagram does not drop messages the
// way a grid drops relationships.
//
// The rules live here rather than inside compose so that stage 4 checks the
// same ones over what it actually emitted. A rule enforced only by the code
// that produces the document proves nothing about the document.
//
// See docs/invariants.md for the contract these implement.
package invariant

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// Problem is one violation.
type Problem struct {
	// Rule names the rule that fired, so a report groups by cause rather than
	// by location.
	Rule string
	// Where locates it within the document.
	Where string
	// Detail says what is actually wrong.
	Detail string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s: %s", p.Rule, p.Where, p.Detail)
}

// componentKinds are the only relationships a component diagram can mean.
var componentKinds = map[diagram.RelationshipKind]bool{
	diagram.KindDependency: true,
	diagram.KindAssembly:   true,
}

// Component checks a composed component document against the model it came
// from.
//
// The contract in full: every relationship the model proves is drawn, or
// recorded on the box it belongs to with a reason, and drawn plus dropped
// equals proven exactly. Every component appears once, as a box or as a band.
// Every connection has two ends on its own page, every box has a cell of its
// own, and every page below the overview is reachable from a box above it.
func Component(model *uml.Model, doc *diagram.Document) []Problem {
	r := &reporter{}
	seen := checkStructure(r, doc, diagram.FamilyComponent, componentKinds)
	if seen == nil {
		return r.problems
	}
	// A component appears once and once only, whether it was drawn as a box or
	// framed as a band: two boxes for one component would say there are two of
	// it.
	noteRegions(r, doc, seen, true)

	// The counts must describe what is actually in the document, or they are a
	// claim rather than a measurement.
	for _, l := range doc.Levels {
		if len(l.Connections) != l.Accounting.Drawn {
			r.add(RuleRecordMatch, "/levels/"+l.ID, "accounting says %d drawn, the level holds %d connections",
				l.Accounting.Drawn, len(l.Connections))
		}
	}

	if model != nil && model.Component != nil {
		ids := make([]string, 0, len(model.Component.Components))
		for _, c := range model.Component.Components {
			ids = append(ids, c.ID)
		}
		sort.Strings(ids)
		requirePresent(r, seen, "component", ids)
	}
	return r.problems
}
