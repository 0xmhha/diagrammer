package render

import (
	"math"
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// Pixel constants. They are a house style rather than a contract: stage 3
// decided which cell a box sits in, and that is the part a rule depends on.
// How wide the box is and how much air is around it may change without
// breaking anything.
const (
	boxMinWidth  = 168
	boxMaxWidth  = 280
	boxHeight    = 68
	labelSize    = 14
	labelMinSize = 9
	subLabelSize = 10

	// Channels are the space routes travel in. Nothing is routed through a
	// cell that is not an endpoint, so they have to be wide enough to hold
	// several lanes side by side. These two are the smallest a channel gets;
	// one that more routes want is widened to hold them.
	channelX = 112
	channelY = 96
	// channelMaxLanes is the most lanes a channel is widened to hold. Past it
	// the page is mostly empty channel, and the routes that do not fit are
	// refused and recorded as usual.
	channelMaxLanes = 24
	// channelSlack is how many lanes a channel holds beyond the routes that
	// want it.
	//
	// Counting the routes and stopping there gives every route a lane and the
	// wrong one. A route is not looking for any free lane but for one that
	// clears what else is in the channel, and with no lane to spare the only
	// one left may be exactly the one that crosses. See docs/thresholds.md for
	// where the number comes from.
	channelSlack = 3
	// laneGap keeps two routes sharing a channel far enough apart to read as
	// two lines rather than one thick one.
	laneGap = 14
	margin  = 48

	// stub is how far a route travels straight out of a box before it turns.
	// Without it the first bend sits on the border and the arrow looks like it
	// grew sideways out of the box.
	stub = 20

	// markerSize is how wide a pseudostate is drawn. A start and an end are
	// points in a state machine rather than places it rests, and drawing them
	// the size of a state would say otherwise.
	markerSize = 26
)

// isMarker reports whether a box is drawn as a dot rather than as a shape with
// a label inside it.
func isMarker(family diagram.Family, stereotype string) bool {
	if family != diagram.FamilyState {
		return false
	}
	return stereotype == "initial" || stereotype == "final" ||
		stereotype == "choice" || stereotype == "junction"
}

// side names the edge of a box a route meets.
type side string

const (
	sideTop    side = "top"
	sideBottom side = "bottom"
	sideLeft   side = "left"
	sideRight  side = "right"
)

// point is a position in the diagram's own pixel space.
type point struct{ X, Y float64 }

// rect is a placed box or band.
type rect struct{ X, Y, W, H float64 }

func (r rect) right() float64   { return r.X + r.W }
func (r rect) bottom() float64  { return r.Y + r.H }
func (r rect) centerX() float64 { return r.X + r.W/2 }
func (r rect) centerY() float64 { return r.Y + r.H/2 }

// placedBox is one box with a position.
type placedBox struct {
	diagram.Box
	rect
	Row, Col   int
	FontSize   float64
	ShortLabel string
}

// placedRegion is a band framing the boxes that belong to it.
type placedRegion struct {
	diagram.Region
	rect
}

// channel is one gap between cells, with the number of routes it can hold.
// The two travel together because a caller that knows where a channel is and
// not how full it may get would have to look the second answer up somewhere
// else, and the two answers come from the same measurement.
type channel struct {
	centre float64
	lanes  int
}

// grid holds where each row and column starts, so a route can find the channel
// beside a cell without recomputing the layout.
//
// Channels are not all one size. gapW[c] is the vertical channel to the left of
// column c and gapH[r] the horizontal channel above row r, each with one more
// entry past the last cell for the channel on the far side.
type grid struct {
	colX   []float64 // left edge of each column's cell
	colW   []float64
	gapW   []float64
	rowY   []float64
	rowH   []float64
	gapH   []float64
	width  float64
	height float64
}

// channelLeftOf returns the vertical channel to the left of a column. Column
// zero has the left margin, which is a channel too.
func (g *grid) channelLeftOf(col int) channel {
	left := g.colX[0] - g.gapW[0]
	if col > 0 {
		left = g.colX[col-1] + g.colW[col-1]
	}
	return channel{centre: left + g.gapW[col]/2, lanes: lanesPerChannel(g.gapW[col])}
}

func (g *grid) channelAbove(row int) channel {
	top := g.rowY[0] - g.gapH[0]
	if row > 0 {
		top = g.rowY[row-1] + g.rowH[row-1]
	}
	return channel{centre: top + g.gapH[row]/2, lanes: lanesPerChannel(g.gapH[row])}
}

func (g *grid) channelBelow(row int) channel {
	top := g.rowY[row] + g.rowH[row]
	return channel{centre: top + g.gapH[row+1]/2, lanes: lanesPerChannel(g.gapH[row+1])}
}

// channelSizes decides how wide each channel is from how many routes will use
// it.
//
// Sizing every channel alike is sizing them for the average, and a hub is where
// the average stops being a guide: a component nine others depend on puts nine
// routes in one channel and none in its neighbours. The counting reads the
// cells the connections join, never a pixel, which is what lets it run before
// the pixels exist.
//
// The plain size is the floor, so a page with nothing crowded looks as it
// always did, and channelMaxLanes is the ceiling. A route that still finds its
// channel full is refused and recorded, as it was before.
func channelSizes(level diagram.Level, cols, rows int) (vertical, horizontal []float64) {
	cell := make(map[string][2]int, len(level.Boxes))
	for _, b := range level.Boxes {
		cell[b.ID] = [2]int{clamp(b.Row, rows), clamp(b.Col, cols)}
	}

	wantV, wantH := make([]int, cols+1), make([]int, rows+1)
	for _, c := range level.Connections {
		from, ok := cell[c.From]
		if !ok {
			continue
		}
		to, ok := cell[c.To]
		if !ok {
			continue
		}
		v, h := channelsWanted(from[0], from[1], to[0], to[1])
		for _, i := range v {
			wantV[i]++
		}
		for _, i := range h {
			wantH[i]++
		}
	}

	vertical = make([]float64, cols+1)
	for i, n := range wantV {
		vertical[i] = sizeForLanes(channelX, n+channelSlack)
	}
	horizontal = make([]float64, rows+1)
	for i, n := range wantH {
		horizontal[i] = sizeForLanes(channelY, n+channelSlack)
	}
	return vertical, horizontal
}

// sizeForLanes inverts lanesPerChannel: the size at which a channel holds n
// lanes, held between the plain size and the ceiling.
func sizeForLanes(base float64, n int) float64 {
	if n > channelMaxLanes {
		n = channelMaxLanes
	}
	return math.Max(base, 2*stub+float64(n)*laneGap)
}

// layOut turns a level's cells into pixels.
//
// Column widths follow the widest label in the column, so a long name widens
// its own column rather than every box on the page.
func layOut(family diagram.Family, level diagram.Level) ([]placedBox, []placedRegion, *grid) {
	cols, rows := level.Grid.Cols, level.Grid.Rows
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}

	colWidth := make([]float64, cols)
	for i := range colWidth {
		colWidth[i] = boxMinWidth
	}
	for _, b := range level.Boxes {
		if b.Col < 0 || b.Col >= cols {
			continue
		}
		if isMarker(family, b.Stereotype) {
			continue // a pseudostate is a dot; it should not widen a column
		}
		want := textWidth(b.Label, labelSize) + textPadding*2
		if want > colWidth[b.Col] {
			colWidth[b.Col] = math.Min(want, boxMaxWidth)
		}
	}

	gapW, gapH := channelSizes(level, cols, rows)
	g := &grid{
		colX: make([]float64, cols),
		colW: colWidth,
		gapW: gapW,
		rowY: make([]float64, rows),
		rowH: make([]float64, rows),
		gapH: gapH,
	}
	x := margin + gapW[0]
	for c := range cols {
		g.colX[c] = x
		x += colWidth[c] + gapW[c+1]
	}
	g.width = x + margin

	y := margin + gapH[0]
	for r := range rows {
		g.rowY[r] = y
		g.rowH[r] = boxHeight
		y += boxHeight + gapH[r+1]
	}
	g.height = y + margin

	boxes := make([]placedBox, 0, len(level.Boxes))
	for _, b := range level.Boxes {
		col, row := clamp(b.Col, cols), clamp(b.Row, rows)
		w, h := colWidth[col], float64(boxHeight)
		x, y := g.colX[col], g.rowY[row]
		if isMarker(family, b.Stereotype) {
			// A start or an end is a dot, not a box. It keeps the centre of its
			// cell so the lines still meet it where a reader expects.
			x, y = x+(w-markerSize)/2, y+(boxHeight-markerSize)/2
			w, h = markerSize, markerSize
		}
		p := placedBox{
			Box:  b,
			rect: rect{X: x, Y: y, W: w, H: h},
			Row:  row,
			Col:  col,
		}
		boxes = append(boxes, p)
	}
	for i := range boxes {
		b := &boxes[i]
		b.FontSize = fittedFontSize(b.Label, b.W, labelSize, labelMinSize)
		// Shrinking has a floor. Past it the label is cut instead, because a
		// name nobody can read and a name that overflows its box are both
		// wrong, and only one of them also breaks the drawing.
		maxUnits := int((b.W - textPadding) / (b.FontSize * widthFactor))
		b.ShortLabel = truncate(b.Label, maxUnits)
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].ID < boxes[j].ID })

	return boxes, layOutRegions(level, boxes), g
}

// layOutRegions frames each band around the boxes that belong to it.
func layOutRegions(level diagram.Level, boxes []placedBox) []placedRegion {
	if len(level.Regions) == 0 {
		return nil
	}
	const pad = 18
	out := make([]placedRegion, 0, len(level.Regions))
	for _, r := range level.Regions {
		var framed []placedBox
		for _, b := range boxes {
			if b.Region == r.ID {
				framed = append(framed, b)
			}
		}
		if len(framed) == 0 {
			continue
		}
		box := framed[0].rect
		for _, b := range framed[1:] {
			box = union(box, b.rect)
		}
		out = append(out, placedRegion{
			Region: r,
			rect:   rect{X: box.X - pad, Y: box.Y - pad - 14, W: box.W + pad*2, H: box.H + pad*2 + 14},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func union(a, b rect) rect {
	x := math.Min(a.X, b.X)
	y := math.Min(a.Y, b.Y)
	return rect{X: x, Y: y, W: math.Max(a.right(), b.right()) - x, H: math.Max(a.bottom(), b.bottom()) - y}
}

func clamp(v, n int) int {
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}
