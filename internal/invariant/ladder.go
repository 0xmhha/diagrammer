package invariant

import (
	"fmt"
	"math"
	"sort"

	"github.com/0xmhha/diagrammer/internal/artifact"
)

// A ladder is judged differently from a grid.
//
// Most of the grid rules mean nothing here. Messages cross lifelines constantly
// and that is how the diagram works; a rule counting those as crossings would
// refuse every sequence diagram ever drawn. What matters instead is that each
// message has a rung of its own, that arrows reach the lifelines they name, and
// that nothing hangs off the edge of the page.
//
// Text is judged here too, and by a different standard from a grid's. A grid
// holds a label clear of every line but its own, because a label near a line is
// read as belonging to it. On a ladder a message's text sits above its rung and
// the lifelines beneath it are thin dashed strokes that every long message
// crosses; holding text clear of those would refuse the ordinary case. What
// would actually be unreadable is text on other text, text over a lifeline's
// head, or text off the page, and that is what is checked.

// Sequence composition rule names.
const (
	RuleRungDistinct   = "rung-distinct"
	RuleMessageSpan    = "message-span"
	RuleActivationLine = "activation-on-lifeline"
	RuleInsideCanvas   = "inside-canvas"
)

// SequenceRules lists the rules a ladder is held to.
func SequenceRules() []string {
	return []string{RuleRungDistinct, RuleMessageSpan, RuleActivationLine, RuleInsideCanvas, RuleLabelClear}
}

// CompositionFor picks the rules a drawing is judged by.
//
// Families are asked by name rather than assumed, so a family drawn without its
// own rules is refused rather than quietly held to another family's.
func CompositionFor(scene *artifact.Scene) []RouteProblem {
	switch scene.Family {
	case "component", "state", "usecase":
		return Composition(scene)
	case "sequence":
		return Sequence4(scene)
	default:
		return []RouteProblem{{
			Route:  scene.Level,
			Rule:   "unknown-family",
			Detail: fmt.Sprintf("the drawing says it is a %q diagram, which has no composition rules", scene.Family),
		}}
	}
}

// Sequence4 judges a drawn ladder.
func Sequence4(scene *artifact.Scene) []RouteProblem {
	var out []RouteProblem
	add := func(id, rule, format string, args ...any) {
		out = append(out, RouteProblem{Route: id, Rule: rule, Detail: fmt.Sprintf(format, args...)})
	}

	const slack = 0.5
	centres := map[string]float64{}
	for _, b := range scene.Boxes {
		centres[b.ID] = b.X + b.W/2
	}

	// Two messages on one rung read as one exchange. Ordering is the whole
	// content of a sequence diagram, so a shared rung destroys the meaning
	// rather than crowding the drawing.
	rungs := map[float64][]string{}
	for _, r := range scene.Routes {
		if len(r.Points) < 2 {
			add(r.ID, RuleMessageSpan, "has fewer than two points, so it is not an arrow")
			continue
		}
		y := r.Points[0].Y
		rungs[y] = append(rungs[y], r.ID)
	}
	for _, y := range sortedFloats(rungs) {
		if ids := rungs[y]; len(ids) > 1 {
			sort.Strings(ids)
			add(ids[0], RuleRungDistinct, "shares a rung with %v, so their order cannot be read", ids[1:])
		}
	}

	for _, r := range scene.Routes {
		if len(r.Points) < 2 {
			continue
		}
		from, hasFrom := centres[r.From]
		to, hasTo := centres[r.To]
		switch {
		case !hasFrom:
			add(r.ID, RuleMessageSpan, "leaves %q, which is not a lifeline here", r.From)
		case math.Abs(r.Points[0].X-from) > slack:
			add(r.ID, RuleMessageSpan, "starts away from the lifeline it names")
		}
		last := r.Points[len(r.Points)-1]
		switch {
		case !hasTo:
			add(r.ID, RuleMessageSpan, "arrives at %q, which is not a lifeline here", r.To)
		case math.Abs(last.X-to) > slack:
			add(r.ID, RuleMessageSpan, "ends away from the lifeline it names")
		}
		for _, p := range r.Points {
			if outside(p, scene) {
				add(r.ID, RuleInsideCanvas, "passes outside the page")
				break
			}
		}
	}

	// An execution bar off its lifeline says the wrong participant was busy.
	for _, bar := range scene.Bars {
		centre, ok := centres[bar.Box]
		if !ok {
			add(bar.ID, RuleActivationLine, "runs on %q, which is not a lifeline here", bar.Box)
			continue
		}
		if math.Abs(bar.X+bar.W/2-centre) > slack {
			add(bar.ID, RuleActivationLine, "sits away from the lifeline it names")
		}
		if bar.H <= 0 {
			add(bar.ID, RuleActivationLine, "has no height, so nothing is shown as running")
		}
		if bar.Y < 0 || bar.Bottom() > scene.Height {
			add(bar.ID, RuleInsideCanvas, "runs off the page")
		}
	}

	checkLadderText(scene, add)

	for _, f := range scene.Frames {
		if f.W <= 0 || f.H <= 0 {
			add(f.ID, RuleInsideCanvas, "frames nothing")
			continue
		}
		if f.X < 0 || f.Y < 0 || f.Right() > scene.Width || f.Bottom() > scene.Height {
			add(f.ID, RuleInsideCanvas, "reaches outside the page")
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Route != out[j].Route {
			return out[i].Route < out[j].Route
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

func outside(p artifact.Point, scene *artifact.Scene) bool {
	return p.X < 0 || p.Y < 0 || p.X > scene.Width || p.Y > scene.Height
}

func sortedFloats[V any](m map[float64]V) []float64 {
	out := make([]float64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Float64s(out)
	return out
}

// checkLadderText holds a message's text to the three things that would make it
// unreadable, and to nothing else.
//
// The rectangle is read from the page rather than worked out here, for the same
// reason the grid's rule reads it: how wide a string is depends on the size that
// one label was shrunk to, which nothing in the string says. A label with no
// rectangle is a label the page never drew.
func checkLadderText(scene *artifact.Scene, add func(string, string, string, ...any)) {
	for i, r := range scene.Routes {
		if r.LabelText == "" || (r.LabelBounds.W == 0 && r.LabelBounds.H == 0) {
			continue
		}
		box := r.LabelBounds

		if box.X < 0 || box.Y < 0 || box.Right() > scene.Width || box.Bottom() > scene.Height {
			add(r.ID, RuleLabelClear, "its text reaches outside the page")
			continue
		}
		// A lifeline's head carries the participant's name. Text over it makes
		// two names one.
		if head, ok := overlappingBox(scene, box); ok {
			add(r.ID, RuleLabelClear, "its text overlaps %s, which carries a name of its own", head)
			continue
		}
		for j, other := range scene.Routes {
			if i == j || other.LabelText == "" {
				continue
			}
			if box.Overlaps(other.LabelBounds) {
				add(r.ID, RuleLabelClear, "its text overlaps the text on %s", other.ID)
				break
			}
		}
	}
}

// overlappingBox names the first box a rectangle lands on.
func overlappingBox(scene *artifact.Scene, box artifact.Rect) (string, bool) {
	for _, b := range scene.Boxes {
		if box.Overlaps(artifact.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}) {
			return b.ID, true
		}
	}
	return "", false
}
