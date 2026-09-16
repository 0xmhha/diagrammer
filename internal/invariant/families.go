package invariant

import (
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// RuleNoDrops is the rule the three families that never drop anything are held
// to.
//
// A ladder has a row for every message and a state machine has somewhere to put
// every transition, so there is no reason for either to refuse one. Asserting
// that rather than assuming it means the day something does start dropping, it
// is a failure rather than a silence.
const RuleNoDrops = "no-drops"

// RuleOrdering is the sequence family's own rule: the ladder's rungs are in the
// order the model wrote them.
const RuleOrdering = "message-ordering"

// RuleBoundary is the use case family's own rule: what the model placed inside
// the system is drawn inside it.
const RuleBoundary = "system-boundary"

var (
	sequenceKinds = map[diagram.RelationshipKind]bool{diagram.KindMessage: true}
	stateKinds    = map[diagram.RelationshipKind]bool{diagram.KindTransition: true}
	usecaseKinds  = map[diagram.RelationshipKind]bool{
		diagram.KindAssociation: true,
		diagram.KindInclude:     true,
		diagram.KindExtend:      true,
	}
)

// Sequence checks a composed sequence document.
//
// The rule: every message and activation in the model appears on the diagram,
// ordering is preserved, and no lifeline a message refers to is missing.
func Sequence(model *uml.Model, doc *diagram.Document) []Problem {
	r := &reporter{}
	seen := checkStructure(r, doc, diagram.FamilySequence, sequenceKinds)
	if seen == nil {
		return r.problems
	}
	requireNoDrops(r, doc)
	requireDrawnMatchesContents(r, doc, func(l diagram.Level) int { return len(l.Activations) })

	if model == nil || model.Sequence == nil {
		return r.problems
	}
	m := model.Sequence

	lifelines := make([]string, 0, len(m.Lifelines))
	for _, l := range m.Lifelines {
		lifelines = append(lifelines, l.ID)
	}
	sort.Strings(lifelines)
	requirePresent(r, seen, "lifeline", lifelines)

	// Ordering is the one thing a sequence diagram cannot get wrong quietly:
	// the same messages in a different order describe a different interaction.
	orderOf := make(map[string]int, len(m.Messages))
	for i, msg := range m.Messages {
		orderOf[msg.ID] = i
	}
	drawn := make(map[string]bool, len(m.Messages))
	for _, l := range doc.Levels {
		for _, c := range l.Connections {
			drawn[c.ID] = true
			want, ok := orderOf[c.ID]
			if !ok {
				r.add(RuleCompleteness, "/levels/"+l.ID+"/connections/"+c.ID, "is not a message the model declared")
				continue
			}
			if c.Order == nil {
				r.add(RuleOrdering, "/levels/"+l.ID+"/connections/"+c.ID,
					"carries no row, so the ladder has no order")
				continue
			}
			if *c.Order != want {
				r.add(RuleOrdering, "/levels/"+l.ID+"/connections/"+c.ID,
					"sits at row %d, but the model puts it at %d", *c.Order, want)
			}
		}
	}
	for _, msg := range m.Messages {
		if !drawn[msg.ID] {
			r.add(RuleCompleteness, "/levels", "message %q appears on no level", msg.ID)
		}
	}

	shown := make(map[string]bool)
	for _, l := range doc.Levels {
		for _, a := range l.Activations {
			shown[a.ID] = true
			if a.ToRow < a.FromRow {
				r.add(RuleOrdering, "/levels/"+l.ID+"/activations/"+a.ID,
					"ends at row %d, before it starts at %d", a.ToRow, a.FromRow)
			}
		}
	}
	for _, a := range m.Activations {
		if !shown[a.ID] {
			r.add(RuleCompleteness, "/levels", "activation %q appears on no level", a.ID)
		}
	}
	return r.problems
}

// State checks a composed state document.
//
// The rule: every state and transition appears, every transition's source and
// target resolve, and initial and final states are present where the model
// declares them.
func State(model *uml.Model, doc *diagram.Document) []Problem {
	r := &reporter{}
	seen := checkStructure(r, doc, diagram.FamilyState, stateKinds)
	if seen == nil {
		return r.problems
	}
	requireNoDrops(r, doc)
	requireDrawnMatchesContents(r, doc, nil)
	// A composite state is both a box and the band its children sit in, so a
	// repeated id here is the one shape saying both things rather than a
	// duplicate.
	noteRegions(r, doc, seen, false)

	if model == nil || model.State == nil {
		return r.problems
	}
	m := model.State

	ids := make([]string, 0, len(m.States))
	for _, s := range m.States {
		ids = append(ids, s.ID)
	}
	sort.Strings(ids)
	requirePresent(r, seen, "state", ids)

	// The pseudostates are drawn differently from a plain state, and the only
	// thing carrying that difference into stage 4 is the stereotype. A final
	// state drawn as an ordinary box is a diagram that reads wrongly while
	// every count still adds up.
	kindOf := make(map[string]uml.StateKind, len(m.States))
	for _, s := range m.States {
		kindOf[s.ID] = s.Kind
	}
	for _, l := range doc.Levels {
		for _, b := range l.Boxes {
			want, ok := kindOf[b.ID]
			if ok && b.Stereotype != string(want) {
				r.add(RuleCompleteness, "/levels/"+l.ID+"/boxes/"+b.ID,
					"is drawn as %q, but the model calls it %q", b.Stereotype, want)
			}
		}
	}

	transitions := make([]string, 0, len(m.Transitions))
	for _, t := range m.Transitions {
		transitions = append(transitions, t.ID)
	}
	sort.Strings(transitions)
	requireConnections(r, doc, "transition", transitions)
	return r.problems
}

// Usecase checks a composed use case document.
//
// The rule: every actor, use case and association appears, include and extend
// resolve to declared use cases, and nothing sits outside the system boundary
// that the model placed inside it.
func Usecase(model *uml.Model, doc *diagram.Document) []Problem {
	r := &reporter{}
	seen := checkStructure(r, doc, diagram.FamilyUsecase, usecaseKinds)
	if seen == nil {
		return r.problems
	}
	requireNoDrops(r, doc)
	requireDrawnMatchesContents(r, doc, nil)
	// The system is a band and nothing else, so it never collides with a box.
	noteRegions(r, doc, seen, false)

	if model == nil || model.Usecase == nil {
		return r.problems
	}
	m := model.Usecase

	var ids []string
	for _, a := range m.Actors {
		ids = append(ids, a.ID)
	}
	for _, u := range m.Usecases {
		ids = append(ids, u.ID)
	}
	sort.Strings(ids)
	requirePresent(r, seen, "element", ids)

	var relations []string
	for _, a := range m.Associations {
		relations = append(relations, a.ID)
	}
	for _, inc := range m.Includes {
		relations = append(relations, inc.ID)
	}
	for _, ext := range m.Extends {
		relations = append(relations, ext.ID)
	}
	sort.Strings(relations)
	requireConnections(r, doc, "relationship", relations)

	// The boundary is the diagram's claim about what the system is responsible
	// for. A use case drawn outside it, or an actor drawn inside, reverses that
	// claim while every count still adds up.
	inside := make(map[string]bool, len(m.Usecases))
	for _, u := range m.Usecases {
		inside[u.ID] = true
	}
	outside := make(map[string]bool, len(m.Actors))
	for _, a := range m.Actors {
		outside[a.ID] = true
	}
	for _, l := range doc.Levels {
		for _, b := range l.Boxes {
			switch {
			case inside[b.ID] && b.Region != m.System.ID:
				r.add(RuleBoundary, "/levels/"+l.ID+"/boxes/"+b.ID,
					"is a use case the model placed inside %q, but it is drawn outside it", m.System.ID)
			case outside[b.ID] && b.Region != "":
				r.add(RuleBoundary, "/levels/"+l.ID+"/boxes/"+b.ID,
					"is an actor, which stands outside the system, but it is drawn inside %q", b.Region)
			}
		}
	}

	// include and extend run between use cases. One ending on an actor would
	// pass the endpoint rule, because an actor is a box, and still be nonsense.
	for _, l := range doc.Levels {
		for _, c := range l.Connections {
			if c.Kind != diagram.KindInclude && c.Kind != diagram.KindExtend {
				continue
			}
			for label, end := range map[string]string{"starts at": c.From, "ends at": c.To} {
				if !inside[end] {
					r.add(RuleBoundary, "/levels/"+l.ID+"/connections/"+c.ID,
						"%s %q, which is not a use case", label, end)
				}
			}
		}
	}
	return r.problems
}

// requireNoDrops holds a family that has no reason to drop anything to that
// claim.
func requireNoDrops(r *reporter, doc *diagram.Document) {
	if doc.Accounting.Dropped != 0 {
		r.add(RuleNoDrops, "/accounting", "%d relationship(s) were dropped, but this family has nowhere to drop one",
			doc.Accounting.Dropped)
	}
}

// requireDrawnMatchesContents holds the drawn count to what is actually on the
// page.
//
// What counts as drawn differs by family: a sequence page is responsible for
// its activations as well as its messages, because both have to appear, while
// the other families only draw connections. Passing the extra count in keeps
// that difference where it belongs rather than hiding it in a shared helper.
func requireDrawnMatchesContents(r *reporter, doc *diagram.Document, extra func(diagram.Level) int) {
	for _, l := range doc.Levels {
		drawn := len(l.Connections)
		if extra != nil {
			drawn += extra(l)
		}
		if drawn != l.Accounting.Drawn {
			r.add(RuleRecordMatch, "/levels/"+l.ID, "accounting says %d drawn, the level holds %d",
				l.Accounting.Drawn, drawn)
		}
	}
}

// requireConnections reports every relationship the model declared that reached
// no level.
func requireConnections(r *reporter, doc *diagram.Document, kind string, ids []string) {
	drawn := make(map[string]bool)
	for _, l := range doc.Levels {
		for _, c := range l.Connections {
			drawn[c.ID] = true
		}
	}
	for _, id := range ids {
		if !drawn[id] {
			r.add(RuleCompleteness, "/levels", "%s %q appears on no level", kind, id)
		}
	}
}
