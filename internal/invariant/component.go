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
