package render

import (
	"math"
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// routed is one connection with a path.
type routed struct {
	diagram.Connection
	Points   []point
	FromSide side
	ToSide   side
	// Label sits at the middle of the longest straight run, which is the only
	// part of a route with room for text.
	LabelAt point
}

// router assigns routes to lanes within the channels between cells.
//
// Nothing is routed through a cell that is not an endpoint, which is what makes
// the pass-through rule hold by construction rather than by checking. What the
// channels cannot do is hold an unbounded number of routes: two lines in one
// lane read as one thick line, so a channel with no lane left refuses the route
// and the caller records it.
type router struct {
	grid  *grid
	boxes map[string]placedBox
	// taken counts how many lanes each channel has handed out. A channel is
	// identified by its centre line, which is stable and unique.
	takenV map[float64]int
	takenH map[float64]int
	// takenEdge counts routes sharing one pair of facing box edges.
	takenEdge map[string]int
}

func newRouter(g *grid, boxes []placedBox) *router {
	byID := make(map[string]placedBox, len(boxes))
	for _, b := range boxes {
		byID[b.ID] = b
	}
	return &router{
		grid: g, boxes: byID,
		takenV: map[float64]int{}, takenH: map[float64]int{}, takenEdge: map[string]int{},
	}
}

// lanesPerChannel is how many routes a channel can carry before it stops
// reading as separate lines.
//
// A stub's worth of room is kept clear at each edge, so the outermost lane is
// still far enough from a box for the turn into it to be a turn rather than a
// kink. That is what keeps the minimum-segment rule from refusing routes the
// router itself created.
func lanesPerChannel(size float64) int {
	n := int((size - 2*stub) / laneGap)
	if n < 1 {
		return 1
	}
	return n
}

// laneFail says a channel had no lane left. It is a real reason a diagram
// cannot show something, and it is what the record on the box is for.
type laneFail struct{ channel string }

func (e *laneFail) Error() string { return "no lane left in the " + e.channel + " channel" }

// route builds a path from one box to another.
//
// Same row and adjacent columns get a straight line, because that is the case
// worth making look simple. Everything else leaves downward, crosses in the
// channel below, climbs a vertical channel and enters from above: a predictable
// shape is easier to read across a whole page than a clever one that differs
// every time.
func (r *router) route(c diagram.Connection) (routed, error) {
	from, ok := r.boxes[c.From]
	if !ok {
		return routed{}, &laneFail{channel: "unknown source"}
	}
	to, ok := r.boxes[c.To]
	if !ok {
		return routed{}, &laneFail{channel: "unknown target"}
	}

	switch {
	case from.Row == to.Row && abs(from.Col-to.Col) == 1:
		return r.straight(c, from, to)
	case abs(from.Row-to.Row) == 1 && from.Col == to.Col:
		return r.stacked(c, from, to)
	case abs(from.Row-to.Row) == 1:
		return r.overOneRow(c, from, to)
	default:
		return r.detour(c, from, to)
	}
}

// channelsWanted names the channels a route between two cells will travel in,
// as indices into the grid's gaps.
//
// It reads the cells rather than the pixels, which is what lets the channels be
// sized before any of them exists. Its four cases are route's four cases in the
// same order, and they have to stay that way: a channel sized for fewer routes
// than arrive refuses them, which is the failure this exists to prevent.
func channelsWanted(fromRow, fromCol, toRow, toCol int) (vertical, horizontal []int) {
	switch {
	case fromRow == toRow && abs(fromCol-toCol) == 1:
		return nil, nil // side by side, and the lane sits on the facing edges
	case abs(fromRow-toRow) == 1 && fromCol == toCol:
		return nil, nil // one above the other, and likewise
	case abs(fromRow-toRow) == 1:
		return nil, []int{max(fromRow, toRow)}
	default:
		return []int{toCol}, []int{fromRow + 1, toRow}
	}
}

// straight joins two boxes side by side with one line.
//
// Neighbours in a row sit at the same height, so the line has no reason to
// bend. Each route takes a lane anyway, offset along the shared edge, so that
// two relationships between the same pair are two lines rather than one drawn
// twice.
func (r *router) straight(c diagram.Connection, from, to placedBox) (routed, error) {
	fromSide, toSide := sideRight, sideLeft
	if from.Col > to.Col {
		fromSide, toSide = sideLeft, sideRight
	}
	// The lane is spread along the shorter of the two facing edges. A state
	// machine's start and end are drawn as dots, and a lane sized for a full
	// box would put the line's end beside the dot rather than on it.
	lane, err := r.takeEdge(from.ID+"|"+to.ID, math.Min(from.H, to.H))
	if err != nil {
		return routed{}, err
	}
	y := from.centerY() + lane

	start := point{X: from.right(), Y: y}
	end := point{X: to.X, Y: y}
	if fromSide == sideLeft {
		start = point{X: from.X, Y: y}
		end = point{X: to.right(), Y: y}
	}
	return finish(c, []point{start, end}, fromSide, toSide), nil
}

// stacked joins two boxes one above the other, in the same column.
//
// The lane goes across their facing edges rather than along the channel. A
// route between them runs straight down, so its horizontal legs have no length
// and a lane taken in the channel moves nothing: two relationships between the
// same pair came out as one line drawn twice, and a label on either sat exactly
// on the other.
func (r *router) stacked(c diagram.Connection, from, to placedBox) (routed, error) {
	downward := to.Row > from.Row
	fromSide, toSide := sideBottom, sideTop
	if !downward {
		fromSide, toSide = sideTop, sideBottom
	}
	lane, err := r.takeEdge(from.ID+"|"+to.ID, math.Min(from.W, to.W))
	if err != nil {
		return routed{}, err
	}
	x := from.centerX() + lane
	start, end := point{X: x, Y: from.bottom()}, point{X: x, Y: to.Y}
	if !downward {
		start, end = point{X: x, Y: from.Y}, point{X: x, Y: to.bottom()}
	}
	return finish(c, []point{start, end}, fromSide, toSide), nil
}

// overOneRow joins boxes in neighbouring rows through the single channel
// between them.
//
// Using one channel rather than two is what keeps the vertical leg from being
// the gap between two lanes, which is shorter than a turn can be drawn in.
func (r *router) overOneRow(c diagram.Connection, from, to placedBox) (routed, error) {
	downward := to.Row > from.Row
	fromSide, toSide := sideBottom, sideTop
	ch := r.grid.channelBelow(from.Row)
	if !downward {
		fromSide, toSide = sideTop, sideBottom
		ch = r.grid.channelAbove(from.Row)
	}

	lane, err := r.takeHorizontal(ch)
	if err != nil {
		return routed{}, err
	}

	start := point{X: from.centerX(), Y: from.bottom()}
	end := point{X: to.centerX(), Y: to.Y}
	if !downward {
		start = point{X: from.centerX(), Y: from.Y}
		end = point{X: to.centerX(), Y: to.bottom()}
	}
	points := []point{start, {X: start.X, Y: lane}, {X: end.X, Y: lane}, end}
	return finish(c, simplify(points), fromSide, toSide), nil
}

// detour is the general case: out of the bottom, across in one channel, up a
// vertical channel, and in from above.
//
// A predictable shape is easier to follow across a whole page than a clever one
// that differs every time, and it never enters a cell it is not attached to.
func (r *router) detour(c diagram.Connection, from, to placedBox) (routed, error) {
	below, err := r.takeHorizontal(r.grid.channelBelow(from.Row))
	if err != nil {
		return routed{}, err
	}
	vertical, err := r.takeVertical(r.grid.channelLeftOf(to.Col))
	if err != nil {
		return routed{}, err
	}
	above, err := r.takeHorizontal(r.grid.channelAbove(to.Row))
	if err != nil {
		return routed{}, err
	}

	start := point{X: from.centerX(), Y: from.bottom()}
	end := point{X: to.centerX(), Y: to.Y}
	points := simplify([]point{
		start,
		{X: start.X, Y: below},
		{X: vertical, Y: below},
		{X: vertical, Y: above},
		{X: end.X, Y: above},
		end,
	})
	return finish(c, points, sideBottom, sideTop), nil
}

// takeEdge spreads routes between the same pair of boxes along their facing
// edges, so two relationships do not land on one line.
func (r *router) takeEdge(pair string, height float64) (float64, error) {
	used := r.takenEdge[pair]
	// The box is only so tall, and a line leaving near its corner stops looking
	// attached to it.
	if used >= lanesPerChannel(height) {
		return 0, &laneFail{channel: "box edge"}
	}
	r.takenEdge[pair] = used + 1
	return laneOffset(used), nil
}

// takeVertical hands out the next free lane in a vertical channel.
func (r *router) takeVertical(ch channel) (float64, error) {
	used := r.takenV[ch.centre]
	if used >= ch.lanes {
		return 0, &laneFail{channel: "vertical"}
	}
	r.takenV[ch.centre] = used + 1
	return ch.centre + laneOffset(used), nil
}

func (r *router) takeHorizontal(ch channel) (float64, error) {
	used := r.takenH[ch.centre]
	if used >= ch.lanes {
		return 0, &laneFail{channel: "horizontal"}
	}
	r.takenH[ch.centre] = used + 1
	return ch.centre + laneOffset(used), nil
}

// laneOffset spreads lanes outward from the channel's centre, alternating
// sides, so the first few routes stay near the middle where there is most room.
func laneOffset(n int) float64 {
	step := float64((n + 1) / 2)
	if n%2 == 1 {
		return -step * laneGap
	}
	return step * laneGap
}

// simplify drops a point that sits between two others on the same straight run.
// Without it a route carries bends that do not bend, and the rhythm rule counts
// them as real.
func simplify(points []point) []point {
	if len(points) < 3 {
		return points
	}
	out := []point{points[0]}
	for i := 1; i < len(points)-1; i++ {
		prev, cur, next := out[len(out)-1], points[i], points[i+1]
		if collinear(prev, cur, next) {
			continue
		}
		out = append(out, cur)
	}
	return append(out, points[len(points)-1])
}

func collinear(a, b, c point) bool {
	return (near(a.X, b.X) && near(b.X, c.X)) || (near(a.Y, b.Y) && near(b.Y, c.Y))
}

func finish(c diagram.Connection, points []point, from, to side) routed {
	return routed{
		Connection: c,
		Points:     points,
		FromSide:   from,
		ToSide:     to,
		LabelAt:    longestRunMidpoint(points),
	}
}

// longestRunMidpoint finds the middle of the longest straight segment, which is
// the only part of a route with room to write on.
func longestRunMidpoint(points []point) point {
	if len(points) < 2 {
		return point{}
	}
	best, bestLen := 0, -1.0
	for i := 0; i+1 < len(points); i++ {
		if l := length(points[i], points[i+1]); l > bestLen {
			best, bestLen = i, l
		}
	}
	a, b := points[best], points[best+1]
	return point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}

func length(a, b point) float64 { return math.Hypot(b.X-a.X, b.Y-a.Y) }

// routeAll routes every connection on a level, in a deterministic order.
//
// Order matters: lanes are handed out first come, first served, so routing the
// same page twice has to visit the connections the same way or the drawing
// moves between runs.
func routeAll(level diagram.Level, g *grid, boxes []placedBox) ([]routed, map[string]error) {
	ordered := make([]diagram.Connection, len(level.Connections))
	copy(ordered, level.Connections)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	r := newRouter(g, boxes)
	var out []routed
	refused := map[string]error{}
	for _, c := range ordered {
		path, err := r.route(c)
		if err != nil {
			refused[c.ID] = err
			continue
		}
		out = append(out, path)
	}
	return out, refused
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
