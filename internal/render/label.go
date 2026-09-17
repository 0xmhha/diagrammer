package render

import (
	"math"
	"sort"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// placeLabels fits each connection's text and puts it where there is room.
//
// The label used to go to the middle of the longest run and stay there. If
// something else passed close to that one point, the clearance rule refused the
// route and the relationship was recorded as undrawn — for want of a few pixels
// along a line that had plenty of other places to write on. Measured on the
// fixtures, that was the largest remaining cause of relationships the drawing
// could not hold.
//
// Positions are tried from the middle outward, so a label stays near the centre
// of its run when it can and only wanders when it must. Ties never arise: the
// first clear position in a fixed order wins, so the same page always puts its
// text in the same place.
func placeLabels(scene *artifact.Scene) []Unwritten {
	var silent []Unwritten
	for i := range scene.Routes {
		if scene.Routes[i].LabelText == "" {
			continue
		}
		if fitted, ok := clearPosition(scene, i); ok {
			scene.Routes[i].LabelText = fitted.text
			scene.Routes[i].LabelSize = fitted.size
			scene.Routes[i].LabelAt = fitted.at
			scene.Routes[i].LabelAnchor = fitted.anchor
			scene.Routes[i].LabelBounds = fitted.bounds
			continue
		}
		// Nowhere clear on the whole route. The line stays and the text goes,
		// and the page says so: a reader of a line with no name can still see
		// the two boxes are joined, which is more than a reader of neither
		// gets. Writing it somewhere still crowded would put the name on
		// somebody else's line, which is worse than not writing it.
		silent = append(silent, Unwritten{
			Route: scene.Routes[i].ID,
			Text:  scene.Routes[i].LabelText,
			Why:   "no room on the route to write it without landing on something else",
		})
		scene.Routes[i].LabelText = ""
		scene.Routes[i].LabelBounds = artifact.Rect{}
	}
	return silent
}

// edgeLabelSize is the size a connection's text is written at when it fits.
// viewer.css names the same number, and docscheck holds the two together.
const edgeLabelSize = 11

// labelLift is how far the text sits above the line it belongs to, so the line
// does not strike through it.
const labelLift = 6

// labelSteps is how many positions along a run are tried. More would find room
// more often and put labels in stranger places; this is enough to clear a
// single crossing without the text drifting to a corner.
const labelSteps = 7

// placedLabel is one label with everything the page and the checker need.
type placedLabel struct {
	text   string
	size   float64
	at     artifact.Point
	anchor string
	bounds artifact.Rect
}

// Text sits above a line that runs across the page and beside one that runs
// down it.
//
// Centring it on the line either way was the earlier answer, and on a vertical
// run it put half the text into the channel on each side: a transition between
// two states in one column had its name written across the routes either side
// of it. Beside the line the text occupies one side only, and it is the side a
// reader scans towards.
const (
	anchorMiddle = "middle"
	anchorStart  = "start"
)

// labelRect is the rectangle text occupies at a position, given how it is
// anchored there.
func labelRect(at artifact.Point, text string, size float64, anchor string) artifact.Rect {
	w := textWidth(text, size)
	h := size * lineHeight
	if anchor == anchorStart {
		return artifact.Rect{X: at.X + labelLift, Y: at.Y - h/2, W: w, H: h}
	}
	return artifact.Rect{X: at.X - w/2, Y: at.Y - labelLift - h, W: w, H: h}
}

// runIsAcross reports whether a run travels more across the page than down it.
func runIsAcross(a, b artifact.Point) bool {
	return math.Abs(b.X-a.X) > math.Abs(b.Y-a.Y)
}

// lineHeight is how tall a line of text is as a multiple of its size. It is the
// browser's own default for a single line, and the label is one line.
const lineHeight = 1.2

// labelBudget is how wide a label may be on a given run.
//
// Text is written across the page whichever way the line runs, so the room it
// has is room along x. A run that is mostly horizontal has its own length; one
// that is mostly vertical travels in a channel, and channelX is the narrowest
// a channel gets.
func labelBudget(a, b artifact.Point) float64 {
	if runIsAcross(a, b) {
		return math.Abs(b.X - a.X)
	}
	return channelX
}

// clearPosition walks the route's runs, longest first and each from the middle
// outward, and returns the first fitted label that sits clear of everything
// else on the page.
//
// Longest first because a long run is where text is easiest to associate with
// its own line. Trying only that one run was the earlier behaviour, and it gave
// up on a route whose longest run happened to be crowded along its whole
// length, although a detour has as many as five runs and the others were often
// empty. A crowded run is a reason to write somewhere else on the same line,
// not a reason to refuse the relationship.
//
// The text is fitted to each run rather than written at one size everywhere.
// It used to be written at full length whatever the room: box labels have been
// measured and shrunk and cut since the first drawing, and the text on a line
// was simply emitted, so a message named after a function signature ran across
// three lifelines and the labels on them.
func clearPosition(scene *artifact.Scene, index int) (placedLabel, bool) {
	route := scene.Routes[index]
	fractions := middleOutFractions(labelSteps)
	for _, run := range runsByLength(route.Points) {
		a, b := run[0], run[1]
		budget := labelBudget(a, b)
		anchor := anchorStart
		if runIsAcross(a, b) {
			anchor = anchorMiddle
		}
		size := fittedFontSize(route.LabelText, budget, edgeLabelSize, labelMinSize)
		text := truncate(route.LabelText, int((budget-textPadding)/(size*widthFactor)))
		if text == "" {
			continue
		}
		for _, t := range fractions {
			at := artifact.Point{X: quantize(a.X + (b.X-a.X)*t), Y: quantize(a.Y + (b.Y-a.Y)*t)}
			bounds := labelRect(at, text, size, anchor)
			if _, ok := invariant.LabelClear(scene, route.ID, bounds); ok {
				return placedLabel{text: text, size: size, at: at, anchor: anchor, bounds: bounds}, true
			}
		}
	}
	return placedLabel{}, false
}

// middleOutFractions returns positions along a run, centre first, then
// alternating outward, never reaching either end where the text would sit on
// top of a bend or an arrowhead.
func middleOutFractions(steps int) []float64 {
	const margin = 0.15
	out := make([]float64, 0, steps)
	for i := range steps {
		offset := float64((i + 1) / 2)
		if i%2 == 1 {
			offset = -offset
		}
		t := 0.5 + offset*(0.5-margin)/float64((steps+1)/2)
		if t < margin || t > 1-margin {
			continue
		}
		out = append(out, t)
	}
	return out
}

// runsByLength returns a route's straight runs, longest first. Runs of equal
// length keep the order they appear in, so the same page always tries them in
// the same order and puts its text in the same place.
func runsByLength(points []artifact.Point) [][2]artifact.Point {
	if len(points) < 2 {
		return nil
	}
	index := make([]int, 0, len(points)-1)
	for i := 0; i+1 < len(points); i++ {
		index = append(index, i)
	}
	length := func(i int) float64 {
		return math.Hypot(points[i+1].X-points[i].X, points[i+1].Y-points[i].Y)
	}
	sort.SliceStable(index, func(a, b int) bool { return length(index[a]) > length(index[b]) })

	out := make([][2]artifact.Point, 0, len(index))
	for _, i := range index {
		out = append(out, [2]artifact.Point{points[i], points[i+1]})
	}
	return out
}
