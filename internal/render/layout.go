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
	// several lanes side by side.
	channelX = 112
	channelY = 96
	// laneGap keeps two routes sharing a channel far enough apart to read as
	// two lines rather than one thick one.
	laneGap = 14
	margin  = 48

	// stub is how far a route travels straight out of a box before it turns.
	// Without it the first bend sits on the border and the arrow looks like it
	// grew sideways out of the box.
	stub = 20
)

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

// grid holds where each row and column starts, so a route can find the channel
// beside a cell without recomputing the layout.
type grid struct {
	colX   []float64 // left edge of each column's cell
	colW   []float64
	rowY   []float64
	rowH   []float64
	width  float64
	height float64
}

// channelLeftOf returns the centre of the vertical channel to the left of a
// column. Column zero has the left margin, which is a channel too.
func (g *grid) channelLeftOf(col int) float64 {
	if col == 0 {
		return g.colX[0] - channelX/2
	}
	prev := g.colX[col-1] + g.colW[col-1]
	return prev + (g.colX[col]-prev)/2
}

func (g *grid) channelRightOf(col int) float64 {
	if col >= len(g.colX)-1 {
		return g.colX[col] + g.colW[col] + channelX/2
	}
	return g.channelLeftOf(col + 1)
}

func (g *grid) channelAbove(row int) float64 {
	if row == 0 {
		return g.rowY[0] - channelY/2
	}
	prev := g.rowY[row-1] + g.rowH[row-1]
	return prev + (g.rowY[row]-prev)/2
}

func (g *grid) channelBelow(row int) float64 {
	if row >= len(g.rowY)-1 {
		return g.rowY[row] + g.rowH[row] + channelY/2
	}
	return g.channelAbove(row + 1)
}

// layOut turns a level's cells into pixels.
//
// Column widths follow the widest label in the column, so a long name widens
// its own column rather than every box on the page.
func layOut(level diagram.Level) ([]placedBox, []placedRegion, *grid) {
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
		want := textWidth(b.Label, labelSize) + textPadding*2
		if want > colWidth[b.Col] {
			colWidth[b.Col] = math.Min(want, boxMaxWidth)
		}
	}

	g := &grid{
		colX: make([]float64, cols),
		colW: colWidth,
		rowY: make([]float64, rows),
		rowH: make([]float64, rows),
	}
	x := float64(margin + channelX)
	for c := range cols {
		g.colX[c] = x
		x += colWidth[c] + channelX
	}
	g.width = x - channelX + channelX + margin

	y := float64(margin + channelY)
	for r := range rows {
		g.rowY[r] = y
		g.rowH[r] = boxHeight
		y += boxHeight + channelY
	}
	g.height = y - channelY + channelY + margin

	boxes := make([]placedBox, 0, len(level.Boxes))
	for _, b := range level.Boxes {
		col, row := clamp(b.Col, cols), clamp(b.Row, rows)
		w := colWidth[col]
		p := placedBox{
			Box:  b,
			rect: rect{X: g.colX[col], Y: g.rowY[row], W: w, H: boxHeight},
			Row:  row,
			Col:  col,
		}
		p.FontSize = fittedFontSize(b.Label, w, labelSize, labelMinSize)
		// Shrinking has a floor. Past it the label is cut instead, because a
		// name nobody can read and a name that overflows its box are both
		// wrong, and only one of them also breaks the drawing.
		maxUnits := int((w - textPadding) / (p.FontSize * widthFactor))
		p.ShortLabel = truncate(b.Label, maxUnits)
		boxes = append(boxes, p)
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
