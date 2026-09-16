package invariant

import (
	"fmt"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// Rule names, so a test can assert which one fired rather than matching prose.
const (
	RuleAccounting   = "accounting"
	RuleRecordMatch  = "record-matches-accounting"
	RuleEndpoints    = "connection-endpoints"
	RulePlacement    = "box-placement"
	RuleDrilldown    = "drilldown"
	RuleCompleteness = "completeness"
	RuleKind         = "relationship-kind"
)

// reporter collects problems for one document.
type reporter struct {
	problems []Problem
}

func (r *reporter) add(rule, where, format string, args ...any) {
	r.problems = append(r.problems, Problem{Rule: rule, Where: where, Detail: fmt.Sprintf(format, args...)})
}

// checkStructure applies the rules that hold for every family, whatever the
// family means by completeness.
//
// It returns the set of ids that appeared somewhere, so each family can then
// ask its own question: whether everything the model declared is in there.
//
// The rules here are shared because they are not judgements about a diagram's
// meaning. A connection with an end nobody can see, two boxes in one cell, a
// page nobody can reach and an accounting that does not describe the document
// are wrong in the same way for a component diagram and a state machine alike.
func checkStructure(r *reporter, doc *diagram.Document, family diagram.Family, allowed map[diagram.RelationshipKind]bool) map[string]string {
	if doc.Family != family {
		r.add(RuleCompleteness, "/family", "document is %q, not a %s diagram", doc.Family, family)
		return nil
	}

	seen := make(map[string]string)
	levelsByID := make(map[string]diagram.Level, len(doc.Levels))
	for _, l := range doc.Levels {
		levelsByID[l.ID] = l
	}

	total := diagram.Accounting{}
	roots := 0

	for _, l := range doc.Levels {
		where := "/levels/" + l.ID

		// The record exists so that nothing can go missing without a trace. An
		// accounting that does not add up is the one failure that would let it.
		if l.Accounting.Drawn+l.Accounting.Dropped != l.Accounting.Proven {
			r.add(RuleAccounting, where, "drawn %d plus dropped %d is not proven %d",
				l.Accounting.Drawn, l.Accounting.Dropped, l.Accounting.Proven)
		}
		total.Proven += l.Accounting.Proven
		total.Drawn += l.Accounting.Drawn
		total.Dropped += l.Accounting.Dropped

		recorded := 0
		for _, b := range l.Boxes {
			recorded += len(b.Dropped)
		}
		if recorded != l.Accounting.Dropped {
			r.add(RuleRecordMatch, where, "accounting says %d dropped, the boxes record %d",
				l.Accounting.Dropped, recorded)
		}

		boxAt := make(map[string]diagram.Box, len(l.Boxes))
		cells := make(map[[2]int]string, len(l.Boxes))
		for _, b := range l.Boxes {
			boxAt[b.ID] = b
			if b.Row >= l.Grid.Rows || b.Col >= l.Grid.Cols {
				r.add(RulePlacement, where+"/boxes/"+b.ID, "sits at row %d col %d, outside a %dx%d grid",
					b.Row, b.Col, l.Grid.Rows, l.Grid.Cols)
			}
			cell := [2]int{b.Row, b.Col}
			if other, taken := cells[cell]; taken {
				r.add(RulePlacement, where+"/boxes/"+b.ID, "shares cell %d,%d with %s", b.Row, b.Col, other)
			}
			cells[cell] = b.ID
			if first, ok := seen[b.ID]; ok {
				r.add(RuleCompleteness, where+"/boxes/"+b.ID, "already appeared at %s", first)
			}
			seen[b.ID] = where + "/boxes"
		}

		for _, c := range l.Connections {
			if _, ok := boxAt[c.From]; !ok {
				r.add(RuleEndpoints, where+"/connections/"+c.ID, "starts at %q, which is not a box here", c.From)
			}
			if _, ok := boxAt[c.To]; !ok {
				r.add(RuleEndpoints, where+"/connections/"+c.ID, "ends at %q, which is not a box here", c.To)
			}
			if !allowed[c.Kind] {
				r.add(RuleKind, where+"/connections/"+c.ID, "is a %q, which means nothing in a %s diagram", c.Kind, family)
			}
		}

		if l.Parent == "" {
			roots++
			if l.OpensFrom != "" {
				r.add(RuleDrilldown, where, "has no parent but names %q as the box that opens it", l.OpensFrom)
			}
			continue
		}
		parent, ok := levelsByID[l.Parent]
		if !ok {
			r.add(RuleDrilldown, where, "names parent %q, which is not a level", l.Parent)
			continue
		}
		opener, ok := boxOf(parent, l.OpensFrom)
		switch {
		case l.OpensFrom == "":
			r.add(RuleDrilldown, where, "has a parent but names no box that opens it")
		case !ok:
			r.add(RuleDrilldown, where, "is opened from %q, which is not a box on %s", l.OpensFrom, parent.ID)
		case opener.Opens != l.ID:
			r.add(RuleDrilldown, where, "is opened from %q, but that box opens %q", l.OpensFrom, opener.Opens)
		}
	}

	if roots != 1 {
		r.add(RuleDrilldown, "/levels", "want exactly one level with no parent, found %d", roots)
	}
	if total != doc.Accounting {
		r.add(RuleAccounting, "/accounting", "document totals %+v do not match the sum of its levels %+v", doc.Accounting, total)
	}
	return seen
}

// noteRegions records the bands a level draws, so a family that frames things
// rather than boxing them still counts them as present.
func noteRegions(r *reporter, doc *diagram.Document, seen map[string]string, uniquePerDocument bool) {
	for _, l := range doc.Levels {
		where := "/levels/" + l.ID
		for _, reg := range l.Regions {
			if first, ok := seen[reg.ID]; ok && uniquePerDocument {
				r.add(RuleCompleteness, where+"/regions/"+reg.ID, "already appeared at %s", first)
				continue
			}
			if _, ok := seen[reg.ID]; !ok {
				seen[reg.ID] = where + "/regions"
			}
		}
	}
}

// requirePresent reports everything the model declared that never reached the
// document. It is the half of completeness a structural check cannot see: the
// document may be perfectly consistent and still be missing something.
func requirePresent(r *reporter, seen map[string]string, kind string, ids []string) {
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			r.add(RuleCompleteness, "/levels", "%s %q appears on no level", kind, id)
		}
	}
}

func boxOf(l diagram.Level, id string) (diagram.Box, bool) {
	for _, b := range l.Boxes {
		if b.ID == id {
			return b, true
		}
	}
	return diagram.Box{}, false
}
