package invariant

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/0xmhha/diagrammer/internal/artifact"
)

// The composition rules judge a drawing rather than a document. They read the
// routed polylines out of the artifact, which is the only place the geometry
// actually exists, and they are what decides whether a page may be called
// drawn.
//
// A route that breaks one is not a failure of the page. It is a relationship
// the geometry could not hold, and stage 4 records it on the box exactly as
// stage 3 records what a level could not hold. Routing every relationship on
// every diagram is a hard problem and nothing requires it; what is required is
// that nothing goes missing without a trace.

// Composition rule names.
const (
	RuleEndpointSide = "endpoint-side"
	RulePassThrough  = "pass-through"
	RuleCrossing     = "crossing"
	RuleMinSegment   = "minimum-segment"
	RuleSeparation   = "separation"
	RuleLabelClear   = "label-clearance"
	RuleBorderRun    = "border-run"
)

// Thresholds the rules test against. See docs/thresholds.md.
const (
	// minSegment is the shortest a bend-to-bend run may be and still read as a
	// deliberate turn rather than a wobble.
	minSegment = 16
	// separation is how far a route keeps from a box it is not attached to.
	// Closer than this and the line looks joined to it.
	separation = 8
	// LabelClearance is how far a connection's text stays from a route that is
	// not its own, so a reader cannot attach the label to the wrong line.
	//
	// Exported because the renderer places labels against the same number. Two
	// copies of it would let the placer aim at one distance while the rule
	// tested another, and the difference would only show as relationships the
	// drawing could not hold.
	LabelClearance = 10
	// borderRun is how long a route may travel alongside a band's border
	// before it reads as tracing the border instead of crossing it.
	borderRun = 24
)

// CompositionRules lists every rule, so a caller can report which ones a page
// was held to and a test can assert each of them fires.
func CompositionRules() []string {
	return []string{
		RuleEndpointSide, RulePassThrough, RuleCrossing,
		RuleMinSegment, RuleSeparation, RuleLabelClear, RuleBorderRun,
	}
}

// RouteProblem is one rule a single route breaks.
//
// It names the route rather than the page because the caller's next move is to
// drop that route and record it, and a page-level complaint would not say which
// one to drop.
type RouteProblem struct {
	Route  string
	Rule   string
	Detail string
}

func (p RouteProblem) String() string {
	return fmt.Sprintf("%s: %s: %s", p.Route, p.Rule, p.Detail)
}

// Composition judges every route on a page and reports the ones that break a
// rule.
//
// Reported in a stable order, because the caller drops what is reported and a
// drawing that differed between runs would not be byte-identical.
func Composition(scene *artifact.Scene) []RouteProblem {
	var out []RouteProblem
	add := func(route, rule, format string, args ...any) {
		out = append(out, RouteProblem{Route: route, Rule: rule, Detail: fmt.Sprintf(format, args...)})
	}

	for _, r := range scene.Routes {
		if len(r.Points) < 2 {
			add(r.ID, RuleMinSegment, "has fewer than two points, so it is not a line")
			continue
		}
		checkEndpointSides(scene, r, add)
		checkSegments(r, add)
		checkBoxes(scene, r, add)
		checkBorders(scene, r, add)
	}
	checkCrossings(scene, add)
	checkLabels(scene, add)

	sort.Slice(out, func(i, j int) bool {
		if out[i].Route != out[j].Route {
			return out[i].Route < out[j].Route
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// checkEndpointSides holds a route to the sides it claims to leave and arrive
// on. A line that says it leaves the right edge and travels left crosses its own
// box on the way out.
func checkEndpointSides(scene *artifact.Scene, r artifact.Route, add func(string, string, string, ...any)) {
	first, second := r.Points[0], r.Points[1]
	last, penultimate := r.Points[len(r.Points)-1], r.Points[len(r.Points)-2]

	if !leaves(first, second, r.FromSide) {
		add(r.ID, RuleEndpointSide, "claims to leave the %s edge but travels the other way", r.FromSide)
	}
	if !arrives(penultimate, last, r.ToSide) {
		add(r.ID, RuleEndpointSide, "claims to arrive at the %s edge but approaches from the other way", r.ToSide)
	}

	// The endpoints have to sit on the boxes they name, or the line starts in
	// mid-air and the sides mean nothing.
	if from, ok := scene.BoxByID(r.From); ok && !onEdge(first, from) {
		add(r.ID, RuleEndpointSide, "starts away from the box it names")
	}
	if to, ok := scene.BoxByID(r.To); ok && !onEdge(last, to) {
		add(r.ID, RuleEndpointSide, "ends away from the box it names")
	}
}

func leaves(from, to artifact.Point, s string) bool {
	switch s {
	case "top":
		return to.Y < from.Y
	case "bottom":
		return to.Y > from.Y
	case "left":
		return to.X < from.X
	case "right":
		return to.X > from.X
	}
	return false
}

func arrives(from, to artifact.Point, s string) bool {
	switch s {
	case "top":
		return from.Y < to.Y
	case "bottom":
		return from.Y > to.Y
	case "left":
		return from.X < to.X
	case "right":
		return from.X > to.X
	}
	return false
}

func onEdge(p artifact.Point, b artifact.Box) bool {
	const slack = 0.5
	onX := p.X >= b.X-slack && p.X <= b.Right()+slack
	onY := p.Y >= b.Y-slack && p.Y <= b.Bottom()+slack
	touchesX := math.Abs(p.X-b.X) <= slack || math.Abs(p.X-b.Right()) <= slack
	touchesY := math.Abs(p.Y-b.Y) <= slack || math.Abs(p.Y-b.Bottom()) <= slack
	return (onX && touchesY) || (onY && touchesX)
}

// checkSegments holds a route's rhythm. A run too short to see is a bend that
// looks like a kink.
func checkSegments(r artifact.Route, add func(string, string, string, ...any)) {
	for i := 0; i+1 < len(r.Points); i++ {
		a, b := r.Points[i], r.Points[i+1]
		l := math.Hypot(b.X-a.X, b.Y-a.Y)
		if l < minSegment {
			add(r.ID, RuleMinSegment, "has a %.1fpx run between bends, below the %dpx a turn needs to read", l, minSegment)
			return // one report per route is enough to decide to drop it
		}
	}
}

// checkBoxes keeps a route out of every box that is not one of its ends, and a
// readable distance from them.
func checkBoxes(scene *artifact.Scene, r artifact.Route, add func(string, string, string, ...any)) {
	for _, b := range scene.Boxes {
		if b.ID == r.From || b.ID == r.To {
			continue
		}
		for i := 0; i+1 < len(r.Points); i++ {
			if segmentCrossesBox(r.Points[i], r.Points[i+1], b, 0) {
				add(r.ID, RulePassThrough, "passes through %s, which is not one of its ends", b.ID)
				return
			}
			if segmentCrossesBox(r.Points[i], r.Points[i+1], b, separation) {
				add(r.ID, RuleSeparation, "runs within %dpx of %s, close enough to read as joined to it", separation, b.ID)
				return
			}
		}
	}
}

// checkBorders catches a route that traces a band's edge instead of crossing
// it, which reads as the band having a side the diagram never meant.
func checkBorders(scene *artifact.Scene, r artifact.Route, add func(string, string, string, ...any)) {
	for _, reg := range scene.Regions {
		for _, border := range regionBorders(reg) {
			for i := 0; i+1 < len(r.Points); i++ {
				if overlap := collinearOverlap(r.Points[i], r.Points[i+1], border[0], border[1]); overlap > borderRun {
					add(r.ID, RuleBorderRun, "runs %.0fpx along the border of %s instead of crossing it", overlap, reg.ID)
					return
				}
			}
		}
	}
}

func regionBorders(r artifact.Region) [][2]artifact.Point {
	tl := artifact.Point{X: r.X, Y: r.Y}
	tr := artifact.Point{X: r.Right(), Y: r.Y}
	br := artifact.Point{X: r.Right(), Y: r.Bottom()}
	bl := artifact.Point{X: r.X, Y: r.Bottom()}
	return [][2]artifact.Point{{tl, tr}, {tr, br}, {br, bl}, {bl, tl}}
}

// checkCrossings reports a proper intersection between two routes that share no
// endpoint. Routes meeting at a box they both touch are not a crossing: they
// are two lines arriving at the same place, which a reader expects.
func checkCrossings(scene *artifact.Scene, add func(string, string, string, ...any)) {
	for i := range scene.Routes {
		for j := i + 1; j < len(scene.Routes); j++ {
			a, b := scene.Routes[i], scene.Routes[j]
			if shareEndpoint(a, b) {
				continue
			}
			if crosses(a, b) {
				add(a.ID, RuleCrossing, "crosses %s, which it shares no end with", b.ID)
				break
			}
		}
	}
}

func shareEndpoint(a, b artifact.Route) bool {
	return a.From == b.From || a.From == b.To || a.To == b.From || a.To == b.To
}

func crosses(a, b artifact.Route) bool {
	for i := 0; i+1 < len(a.Points); i++ {
		for j := 0; j+1 < len(b.Points); j++ {
			if properIntersection(a.Points[i], a.Points[i+1], b.Points[j], b.Points[j+1]) {
				return true
			}
		}
	}
	return false
}

// checkLabels keeps a connection's text off everything but its own line. A
// label sitting on another line attaches itself to the wrong relationship, and
// the reader has no way to tell; one sitting on another label or on a box is
// simply two pieces of text in one place.
//
// One report per label, and every label examined. An earlier version returned
// from the whole function at the first finding, which meant a page with two
// misplaced labels reported one: the caller dropped that route, drew the rest,
// and the check over the emitted artifact then found the other. A checker has
// to report everything it can see in one pass, or the caller acting on its
// answer is acting on a fragment of it.
func checkLabels(scene *artifact.Scene, add func(string, string, string, ...any)) {
	for _, r := range scene.Routes {
		if r.LabelText == "" {
			continue
		}
		if why, ok := LabelClear(scene, r.ID, r.LabelBounds); !ok {
			add(r.ID, RuleLabelClear, "%s", why)
		}
	}
}

// LabelClear reports whether a rectangle of text sits clear of everything on
// the page but the route it belongs to, and says what it hit when it does not.
//
// The renderer asks this while it is choosing where to put a label and the
// checker asks it of the label the renderer chose, which is deliberate: two
// implementations of the same question would let the drawing satisfy one and
// fail the other, and the page would lose a relationship for a reason its own
// author did not agree with.
//
// The rectangle is the text, not a point. Measuring a label as its centre was
// the earlier answer and it passed a page where eight of eight labels lay
// across another line: a string forty characters long has a centre that clears
// everything and two ends that clear nothing.
//
// Clearance is kept from lines and not from text or boxes. A label is read as
// belonging to the nearest line, so it has to be nearer its own than any other
// by a margin; a label merely touching another label or a box is already
// wrong, and demanding a gap there would move text about for no gain.
func LabelClear(scene *artifact.Scene, route string, box artifact.Rect) (string, bool) {
	if box.W == 0 && box.H == 0 {
		return "", true // no label, nothing to keep clear
	}
	padded := artifact.Rect{
		X: box.X - LabelClearance, Y: box.Y - LabelClearance,
		W: box.W + 2*LabelClearance, H: box.H + 2*LabelClearance,
	}
	for _, other := range scene.Routes {
		if other.ID == route {
			continue
		}
		for i := 0; i+1 < len(other.Points); i++ {
			if segmentMeetsRect(other.Points[i], other.Points[i+1], padded) {
				return "its text comes within " + itoa(LabelClearance) + "px of " + other.ID, false
			}
		}
		if other.LabelText != "" && other.LabelBounds.Overlaps(box) {
			return "its text overlaps the text on " + other.ID, false
		}
	}
	for _, b := range scene.Boxes {
		if (artifact.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}).Overlaps(box) {
			return "its text overlaps " + b.ID, false
		}
	}
	return "", true
}

func itoa(n int) string { return strconv.Itoa(n) }

// segmentMeetsRect reports whether an axis-aligned segment enters a rectangle.
// The segments here are axis-aligned, so an overlap test is exact.
func segmentMeetsRect(a, b artifact.Point, r artifact.Rect) bool {
	loX, hiX := math.Min(a.X, b.X), math.Max(a.X, b.X)
	loY, hiY := math.Min(a.Y, b.Y), math.Max(a.Y, b.Y)
	return loX < r.Right() && hiX > r.X && loY < r.Bottom() && hiY > r.Y
}

// --- geometry -----------------------------------------------------------------

func segmentCrossesBox(a, b artifact.Point, box artifact.Box, gap float64) bool {
	x1, y1 := box.X-gap, box.Y-gap
	x2, y2 := box.Right()+gap, box.Bottom()+gap
	// The segments here are axis-aligned, so an overlap test is exact and needs
	// no clipping.
	loX, hiX := math.Min(a.X, b.X), math.Max(a.X, b.X)
	loY, hiY := math.Min(a.Y, b.Y), math.Max(a.Y, b.Y)
	return loX < x2 && hiX > x1 && loY < y2 && hiY > y1
}

// properIntersection reports whether two segments cross at a point interior to
// both. Touching at an end is not a crossing.
func properIntersection(a, b, c, d artifact.Point) bool {
	d1 := cross(c, d, a)
	d2 := cross(c, d, b)
	d3 := cross(a, b, c)
	d4 := cross(a, b, d)
	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}

func cross(a, b, p artifact.Point) float64 {
	return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
}

// collinearOverlap returns how far two parallel, collinear segments run
// together. Segments that are not collinear overlap by nothing.
func collinearOverlap(a, b, c, d artifact.Point) float64 {
	const slack = 0.5
	if math.Abs(a.X-b.X) < slack && math.Abs(c.X-d.X) < slack && math.Abs(a.X-c.X) < slack {
		return spanOverlap(a.Y, b.Y, c.Y, d.Y)
	}
	if math.Abs(a.Y-b.Y) < slack && math.Abs(c.Y-d.Y) < slack && math.Abs(a.Y-c.Y) < slack {
		return spanOverlap(a.X, b.X, c.X, d.X)
	}
	return 0
}

func spanOverlap(a1, a2, b1, b2 float64) float64 {
	lo := math.Max(math.Min(a1, a2), math.Min(b1, b2))
	hi := math.Min(math.Max(a1, a2), math.Max(b1, b2))
	if hi <= lo {
		return 0
	}
	return hi - lo
}

// Crossings returns every pair of routes that cross although they share no
// endpoint, in a stable order.
//
// Composition reports one route per crossing, which is all a check over a
// finished drawing needs: it answers whether the page is sound. A caller
// deciding what to leave out needs the pair. A crossing condemns two routes
// jointly and which of them goes is a choice, where every other rule condemns
// one route on its own and leaves nothing to decide.
func Crossings(scene *artifact.Scene) [][2]string {
	var out [][2]string
	for i := range scene.Routes {
		for j := i + 1; j < len(scene.Routes); j++ {
			a, b := scene.Routes[i], scene.Routes[j]
			if shareEndpoint(a, b) || !crosses(a, b) {
				continue
			}
			out = append(out, [2]string{a.ID, b.ID})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}
