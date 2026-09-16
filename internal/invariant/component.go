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

// Rule names, so a test can assert which one fired rather than matching prose.
const (
	RuleAccounting   = "accounting"
	RuleRecordMatch  = "record-matches-accounting"
	RuleEndpoints    = "connection-endpoints"
	RulePlacement    = "box-placement"
	RuleDrilldown    = "drilldown"
	RuleCompleteness = "completeness"
)

// Component checks a composed component document against the model it came
// from.
//
// The contract in full: every relationship the model proves is drawn, or
// recorded on the box it belongs to with a reason, and drawn plus dropped
// equals proven exactly. Every component appears once, as a box or as a band.
// Every connection has two ends on its own page, every box has a cell of its
// own, and every page below the overview is reachable from a box above it.
func Component(model *uml.Model, doc *diagram.Document) []Problem {
	var problems []Problem
	add := func(rule, where, format string, args ...any) {
		problems = append(problems, Problem{Rule: rule, Where: where, Detail: fmt.Sprintf(format, args...)})
	}

	if doc.Family != diagram.FamilyComponent {
		add(RuleCompleteness, "/family", "document is %q, not a component diagram", doc.Family)
		return problems
	}

	total := diagram.Accounting{}
	seen := make(map[string]string) // component id -> where it appeared
	roots := 0
	levelsByID := make(map[string]diagram.Level, len(doc.Levels))
	for _, l := range doc.Levels {
		levelsByID[l.ID] = l
	}

	for _, l := range doc.Levels {
		where := "/levels/" + l.ID

		// Accounting. This is the rule the whole record exists for: a
		// relationship missing from both the drawing and the record has
		// vanished, and nothing downstream could tell.
		if l.Accounting.Drawn+l.Accounting.Dropped != l.Accounting.Proven {
			add(RuleAccounting, where, "drawn %d plus dropped %d is not proven %d",
				l.Accounting.Drawn, l.Accounting.Dropped, l.Accounting.Proven)
		}
		total.Proven += l.Accounting.Proven
		total.Drawn += l.Accounting.Drawn
		total.Dropped += l.Accounting.Dropped

		// The counts must describe what is actually in the document, or they
		// are a claim rather than a measurement.
		if len(l.Connections) != l.Accounting.Drawn {
			add(RuleRecordMatch, where, "accounting says %d drawn, the level holds %d connections",
				l.Accounting.Drawn, len(l.Connections))
		}
		recorded := 0
		for _, b := range l.Boxes {
			recorded += len(b.Dropped)
		}
		if recorded != l.Accounting.Dropped {
			add(RuleRecordMatch, where, "accounting says %d dropped, the boxes record %d",
				l.Accounting.Dropped, recorded)
		}

		boxAt := make(map[string]diagram.Box, len(l.Boxes))
		cells := make(map[[2]int]string, len(l.Boxes))
		for _, b := range l.Boxes {
			boxAt[b.ID] = b

			if b.Row >= l.Grid.Rows || b.Col >= l.Grid.Cols {
				add(RulePlacement, where+"/boxes/"+b.ID, "sits at row %d col %d, outside a %dx%d grid",
					b.Row, b.Col, l.Grid.Rows, l.Grid.Cols)
			}
			cell := [2]int{b.Row, b.Col}
			if other, taken := cells[cell]; taken {
				add(RulePlacement, where+"/boxes/"+b.ID, "shares cell %d,%d with %s", b.Row, b.Col, other)
			}
			cells[cell] = b.ID

			if first, ok := seen[b.ID]; ok {
				add(RuleCompleteness, where+"/boxes/"+b.ID, "already appeared at %s", first)
			}
			seen[b.ID] = where + "/boxes"
		}
		for _, r := range l.Regions {
			if first, ok := seen[r.ID]; ok {
				add(RuleCompleteness, where+"/regions/"+r.ID, "already appeared at %s", first)
			}
			seen[r.ID] = where + "/regions"
		}

		// A connection has to have two ends on the page that draws it,
		// otherwise it is a line to somewhere the reader cannot see.
		for _, c := range l.Connections {
			if _, ok := boxAt[c.From]; !ok {
				add(RuleEndpoints, where+"/connections/"+c.ID, "starts at %q, which is not a box here", c.From)
			}
			if _, ok := boxAt[c.To]; !ok {
				add(RuleEndpoints, where+"/connections/"+c.ID, "ends at %q, which is not a box here", c.To)
			}
		}

		// Drill-down. A page nobody can reach is a page nobody will read.
		if l.Parent == "" {
			roots++
			if l.OpensFrom != "" {
				add(RuleDrilldown, where, "has no parent but names %q as the box that opens it", l.OpensFrom)
			}
			continue
		}
		parent, ok := levelsByID[l.Parent]
		if !ok {
			add(RuleDrilldown, where, "names parent %q, which is not a level", l.Parent)
			continue
		}
		opener, ok := boxOf(parent, l.OpensFrom)
		switch {
		case l.OpensFrom == "":
			add(RuleDrilldown, where, "has a parent but names no box that opens it")
		case !ok:
			add(RuleDrilldown, where, "is opened from %q, which is not a box on %s", l.OpensFrom, parent.ID)
		case opener.Opens != l.ID:
			add(RuleDrilldown, where, "is opened from %q, but that box opens %q", l.OpensFrom, opener.Opens)
		}
	}

	if roots != 1 {
		add(RuleDrilldown, "/levels", "want exactly one level with no parent, found %d", roots)
	}
	if total != doc.Accounting {
		add(RuleAccounting, "/accounting", "document totals %+v do not match the sum of its levels %+v", doc.Accounting, total)
	}

	// Nothing the model declared may be silently discarded.
	if model != nil && model.Component != nil {
		var missing []string
		for _, c := range model.Component.Components {
			if _, ok := seen[c.ID]; !ok {
				missing = append(missing, c.ID)
			}
		}
		sort.Strings(missing)
		for _, id := range missing {
			add(RuleCompleteness, "/levels", "component %q appears on no level, as neither a box nor a band", id)
		}
	}

	return problems
}

func boxOf(l diagram.Level, id string) (diagram.Box, bool) {
	for _, b := range l.Boxes {
		if b.ID == id {
			return b, true
		}
	}
	return diagram.Box{}, false
}
