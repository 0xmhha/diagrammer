package render

import (
	"math"

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

// clearPosition walks the route's longest run, from the middle outward, and
// returns the first point far enough from every other route.
func clearPosition(scene *artifact.Scene, index int) (artifact.Point, bool) {
	route := scene.Routes[index]
	a, b, ok := longestRun(route.Points)
	if !ok {
		return artifact.Point{}, false
	}

	for _, t := range middleOutFractions(labelSteps) {
		at := artifact.Point{X: quantize(a.X + (b.X-a.X)*t), Y: quantize(a.Y + (b.Y-a.Y)*t)}
		if clearOfOthers(scene, index, at) {
			return at, true
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

func longestRun(points []artifact.Point) (a, b artifact.Point, ok bool) {
	if len(points) < 2 {
		return a, b, false
	}
	best, bestLen := 0, -1.0
	for i := 0; i+1 < len(points); i++ {
		if l := math.Hypot(points[i+1].X-points[i].X, points[i+1].Y-points[i].Y); l > bestLen {
			best, bestLen = i, l
		}
	}
	return points[best], points[best+1], true
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
