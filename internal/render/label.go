package render

import (
	"math"
	"sort"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// placeLabels moves each connection's text to a spot on its own route where
// there is room for it.
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
func placeLabels(scene *artifact.Scene) {
	for i := range scene.Routes {
		if scene.Routes[i].LabelText == "" {
			continue
		}
		if at, ok := clearPosition(scene, i); ok {
			scene.Routes[i].LabelAt = at
		}
		// No clear position anywhere on the route: the label stays at the
		// middle and the composition rule refuses it, which is the honest
		// outcome. Moving it somewhere still crowded would hide the problem
		// rather than solve it.
	}
}

// labelSteps is how many positions along a run are tried. More would find room
// more often and put labels in stranger places; this is enough to clear a
// single crossing without the text drifting to a corner.
const labelSteps = 7

// clearPosition walks the route's runs, longest first and each from the middle
// outward, and returns the first point far enough from every other route.
//
// Longest first because a long run is where text is easiest to associate with
// its own line. Trying only that one run was the earlier behaviour, and it gave
// up on a route whose longest run happened to be crowded along its whole
// length, although a detour has as many as five runs and the others were often
// empty. A crowded run is a reason to write somewhere else on the same line,
// not a reason to refuse the relationship.
func clearPosition(scene *artifact.Scene, index int) (artifact.Point, bool) {
	fractions := middleOutFractions(labelSteps)
	for _, run := range runsByLength(scene.Routes[index].Points) {
		a, b := run[0], run[1]
		for _, t := range fractions {
			at := artifact.Point{X: quantize(a.X + (b.X-a.X)*t), Y: quantize(a.Y + (b.Y-a.Y)*t)}
			if clearOfOthers(scene, index, at) {
				return at, true
			}
		}
	}
	return artifact.Point{}, false
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

func clearOfOthers(scene *artifact.Scene, index int, at artifact.Point) bool {
	for j, other := range scene.Routes {
		if j == index {
			continue
		}
		for k := 0; k+1 < len(other.Points); k++ {
			if segmentDistance(at, other.Points[k], other.Points[k+1]) < invariant.LabelClearance {
				return false
			}
		}
	}
	return true
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

func segmentDistance(p, a, b artifact.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}
