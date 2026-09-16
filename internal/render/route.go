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
	// taken records which lanes of each channel are spoken for. A channel is
	// identified by its centre line, which is stable and unique.
	//
	// Which lanes rather than how many: a route picks the lane crossing the
	// least of what is already drawn, so it has to ask what is still free
	// rather than simply take the next one.
	takenV map[float64]map[int]bool
	takenH map[float64]map[int]bool
	// takenSide records which positions along one side of one box are taken,
	// and placed counts how many routes have met it. One allocator serves every
	// shape, so a route leaving a side and a route arriving at it cannot be
	// handed the same place.
	takenSide map[string]map[int]bool
	placed    map[string]int
	// drawn is what has already been routed on this level, so a route still
	// choosing its way can see what it would have to cross.
	drawn []routed
}

func newRouter(g *grid, boxes []placedBox) *router {
	byID := make(map[string]placedBox, len(boxes))
	for _, b := range boxes {
		byID[b.ID] = b
	}
	return &router{
		grid: g, boxes: byID,
		takenV: map[float64]map[int]bool{}, takenH: map[float64]map[int]bool{},
		takenSide: map[string]map[int]bool{}, placed: map[string]int{},
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
	case fromRow == toRow:
		return nil, []int{fromRow + 1} // under the row, and back up
	case toRow > fromRow:
		return []int{toCol}, []int{fromRow + 1, toRow}
	default:
		return []int{toCol}, []int{fromRow, toRow + 1}
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
	lane, err := r.takeEdgeLane(math.Min(from.H, to.H),
		sideKey(from.ID, fromSide), sideKey(to.ID, toSide))
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
	lane, err := r.takeEdgeLane(math.Min(from.W, to.W),
		sideKey(from.ID, fromSide), sideKey(to.ID, toSide))
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

	startX, endX := r.stubs(from, fromSide, to, toSide)
	start := point{X: startX, Y: from.bottom()}
	end := point{X: endX, Y: to.Y}
	if !downward {
		start.Y, end.Y = from.Y, to.bottom()
	}
	lane, err := r.takeLaneAvoiding(c, ch, func(lane float64) []point {
		return simplify([]point{start, {X: start.X, Y: lane}, {X: end.X, Y: lane}, end})
	})
	if err != nil {
		return routed{}, err
	}
	points := []point{start, {X: start.X, Y: lane}, {X: end.X, Y: lane}, end}
	return finish(c, simplify(points), fromSide, toSide), nil
}

// detour is the general case: out of the side that faces the target, across in
// the channel just outside that side, along a vertical channel, and in through
// the target's facing side.
//
// A predictable shape is easier to follow across a whole page than a clever one
// that differs every time, and it never enters a cell it is not attached to.
// Which side it leaves by is decided by where the target is. Leaving downward
// whatever the direction is also predictable, and it was what this did, but a
// route to a row above then had to climb back past its own row and travel the
// channel above the target: the full height of the page for a relationship
// between two rows, crossing everything in between. Both halves now stay inside
// the band the two rows bound.
func (r *router) detour(c diagram.Connection, from, to placedBox) (routed, error) {
	if from.Row == to.Row {
		return r.alongRow(c, from, to)
	}

	fromSide, toSide := sideBottom, sideTop
	nearFrom, nearTo := r.grid.channelBelow(from.Row), r.grid.channelAbove(to.Row)
	startY, endY := from.bottom(), to.Y
	if to.Row < from.Row {
		fromSide, toSide = sideTop, sideBottom
		nearFrom, nearTo = r.grid.channelAbove(from.Row), r.grid.channelBelow(to.Row)
		startY, endY = from.Y, to.bottom()
	}

	w, err := r.chooseWay(c, from, to, nearFrom, nearTo, startY, endY)
	if err != nil {
		return routed{}, err
	}

	startX, endX := r.stubs(from, fromSide, to, toSide)
	start := point{X: startX, Y: startY}
	end := point{X: endX, Y: endY}
	points := simplify([]point{
		start,
		{X: start.X, Y: w.first},
		{X: w.x, Y: w.first},
		{X: w.x, Y: w.second},
		{X: end.X, Y: w.second},
		end,
	})
	return finish(c, points, fromSide, toSide), nil
}

// alongRow joins two boxes in one row that are not neighbours.
//
// It drops into the channel below the row, runs under whatever sits between
// them, and comes straight back up. The general detour would climb a vertical
// channel to the channel above the row and come down into the target's top,
// which is a lap around the row for a relationship that never leaves it.
func (r *router) alongRow(c diagram.Connection, from, to placedBox) (routed, error) {
	startX, endX := r.stubs(from, sideBottom, to, sideBottom)
	start := point{X: startX, Y: from.bottom()}
	end := point{X: endX, Y: to.bottom()}
	lane, err := r.takeLaneAvoiding(c, r.grid.channelBelow(from.Row), func(lane float64) []point {
		return []point{start, {X: start.X, Y: lane}, {X: end.X, Y: lane}, end}
	})
	if err != nil {
		return routed{}, err
	}
	points := []point{start, {X: start.X, Y: lane}, {X: end.X, Y: lane}, end}
	return finish(c, simplify(points), sideBottom, sideBottom), nil
}

// stubs places the two ends of a bent route along the sides they meet.
//
// The ends are asked for separately, unlike a straight route's, because the
// route bends anyway and nothing is gained by holding them level.
func (r *router) stubs(from placedBox, fromSide side, to placedBox, toSide side) (startX, endX float64) {
	return from.centerX() + r.stubLane(sideKey(from.ID, fromSide), from.W),
		to.centerX() + r.stubLane(sideKey(to.ID, toSide), to.W)
}

// stubLane places one end of a bent route along a box side, taking the lowest
// position still free.
//
// A side with nothing free hands out a position already in use rather than
// refusing the route. Two stubs sharing a few pixels of box edge is a drawing
// that reads slightly worse; a refusal is a relationship the reader never sees
// at all, and the second is the larger loss. A straight or stacked route is
// refused in that case, because it has no bend to tell it apart from the line
// it would be drawn along.
func (r *router) stubLane(key string, extent float64) float64 {
	limit := lanesPerChannel(extent)
	n := r.placed[key]
	r.placed[key] = n + 1
	for i := range limit {
		if !r.takenSide[key][i] {
			if r.takenSide[key] == nil {
				r.takenSide[key] = map[int]bool{}
			}
			r.takenSide[key][i] = true
			return laneOffset(i)
		}
	}
	return laneOffset(n % limit)
}

// sideKey names one side of one box.
func sideKey(box string, s side) string { return box + "|" + string(s) }

// takeEdgeLane hands out a position along the sides of boxes a route meets, so
// that no two routes meet one side of one box in the same place.
//
// Every shape asks the same allocator. Two allocators is what this replaced,
// one keyed by pair for straight and stacked routes and one keyed by box side
// for bent ones, and each handed out its own first position: a route arriving
// at a box's top and a route leaving it were drawn along the same line for the
// length of their stubs. No rule saw it. Two parallel lines on one x do not
// properly intersect, so the crossing rule said nothing, and the two shared an
// endpoint in any case. What noticed was a label, sitting on a line that ran
// underneath it the whole way.
//
// A straight or stacked route passes both of the sides it joins and uses one
// position at both ends, because moving one end alone would bend it. A bent
// route asks for each end separately.
//
// The box is only so wide, and a line leaving near its corner stops looking
// attached to it, so a full side refuses the route.
func (r *router) takeEdgeLane(extent float64, sides ...string) (float64, error) {
	for n := range lanesPerChannel(extent) {
		taken := false
		for _, key := range sides {
			if r.takenSide[key][n] {
				taken = true
				break
			}
		}
		if taken {
			continue
		}
		for _, key := range sides {
			if r.takenSide[key] == nil {
				r.takenSide[key] = map[int]bool{}
			}
			r.takenSide[key][n] = true
		}
		return laneOffset(n), nil
	}
	return 0, &laneFail{channel: "box edge"}
}

// freeLanes lists the lanes of a channel nobody has taken, nearest the middle
// first, which is the order they were handed out in before anything chose.
func freeLanes(taken map[float64]map[int]bool, ch channel) []int {
	used := taken[ch.centre]
	out := make([]int, 0, ch.lanes)
	for i := range ch.lanes {
		if !used[i] {
			out = append(out, i)
		}
	}
	return out
}

func claim(taken map[float64]map[int]bool, centre float64, lane int) {
	if taken[centre] == nil {
		taken[centre] = map[int]bool{}
	}
	taken[centre][lane] = true
}

// way is one set of choices a detour can make.
type way struct {
	first, second, x float64
}

// chooseWay picks where a detour travels.
//
// Three things are open to it: which lane of the channel beside the source it
// drops into, which vertical channel it climbs, and which lane of the channel
// beside the target it arrives in. All three were settled blind. Lanes were
// handed out in the order routes arrived and the vertical was always the one
// left of the target, so a detour crossing a row went through whatever happened
// to be in the way.
//
// Every combination still free is tried and the one crossing the least of what
// is already drawn wins. Ties go to the lane nearest the middle of its channel
// and the vertical nearest the target, which is what this always chose, so a
// page with nothing in the way is routed exactly as it was.
//
// routeAll draws the routes with no choice first. That is what makes looking
// worth doing: what a detour can see is everything that could not have gone
// anywhere else.
func (r *router) chooseWay(c diagram.Connection, from, to placedBox, nearFrom, nearTo channel, startY, endY float64) (way, error) {
	firsts := freeLanes(r.takenH, nearFrom)
	seconds := freeLanes(r.takenH, nearTo)
	if len(firsts) == 0 || len(seconds) == 0 {
		return way{}, &laneFail{channel: "horizontal"}
	}

	type choice struct {
		w                  way
		firstI, secondI, v int
		centre             float64
	}
	var best choice
	bestScore, found := 0, false
	for _, col := range r.columnsNearest(to.Col) {
		ch := r.grid.channelLeftOf(col)
		for _, v := range freeLanes(r.takenV, ch) {
			for _, f := range firsts {
				for _, sc := range seconds {
					cand := choice{
						w: way{
							first:  nearFrom.centre + laneOffset(f),
							second: nearTo.centre + laneOffset(sc),
							x:      ch.centre + laneOffset(v),
						},
						firstI: f, secondI: sc, v: v, centre: ch.centre,
					}
					score := r.crossingsOf(c, from, to, cand.w, startY, endY)
					if !found || score < bestScore {
						found, bestScore, best = true, score, cand
					}
					if bestScore == 0 {
						goto done
					}
				}
			}
		}
	}
done:
	if !found {
		return way{}, &laneFail{channel: "vertical"}
	}
	claim(r.takenH, nearFrom.centre, best.firstI)
	claim(r.takenH, nearTo.centre, best.secondI)
	claim(r.takenV, best.centre, best.v)
	return best.w, nil
}

// takeLaneAvoiding picks a lane for a route that travels in one channel only,
// by the same rule: the free lane crossing the least of what is drawn.
func (r *router) takeLaneAvoiding(c diagram.Connection, ch channel, path func(lane float64) []point) (float64, error) {
	lanes := freeLanes(r.takenH, ch)
	if len(lanes) == 0 {
		return 0, &laneFail{channel: "horizontal"}
	}
	best, bestScore := lanes[0], -1
	for _, i := range lanes {
		score := r.crossings(c, path(ch.centre+laneOffset(i)))
		if bestScore < 0 || score < bestScore {
			best, bestScore = i, score
		}
		if bestScore == 0 {
			break
		}
	}
	claim(r.takenH, ch.centre, best)
	return ch.centre + laneOffset(best), nil
}

// columnsNearest lists every vertical channel, the target's own first and then
// outward, so a tie keeps the route close to where it is going.
func (r *router) columnsNearest(preferred int) []int {
	n := len(r.grid.colX)
	out := []int{preferred}
	for d := 1; d <= n; d++ {
		if preferred-d >= 0 {
			out = append(out, preferred-d)
		}
		if preferred+d <= n {
			out = append(out, preferred+d)
		}
	}
	return out
}

func (r *router) crossingsOf(c diagram.Connection, from, to placedBox, w way, startY, endY float64) int {
	return r.crossings(c, simplify([]point{
		{X: from.centerX(), Y: startY},
		{X: from.centerX(), Y: w.first},
		{X: w.x, Y: w.first},
		{X: w.x, Y: w.second},
		{X: to.centerX(), Y: w.second},
		{X: to.centerX(), Y: endY},
	}))
}

// crossings counts how many drawn routes a candidate path would cross. Routes
// sharing an end with it are not counted: two lines arriving at one box are
// what a reader expects, and the rule says the same.
func (r *router) crossings(c diagram.Connection, candidate []point) int {
	n := 0
	for _, other := range r.drawn {
		if other.From == c.From || other.From == c.To || other.To == c.From || other.To == c.To {
			continue
		}
		if pathsCross(candidate, other.Points) {
			n++
		}
	}
	return n
}

// pathsCross reports whether two polylines properly intersect anywhere.
func pathsCross(a, b []point) bool {
	for i := 0; i+1 < len(a); i++ {
		for j := 0; j+1 < len(b); j++ {
			if segmentsCross(a[i], a[i+1], b[j], b[j+1]) {
				return true
			}
		}
	}
	return false
}

func segmentsCross(a, b, c, d point) bool {
	s1, s2 := turn(c, d, a), turn(c, d, b)
	s3, s4 := turn(a, b, c), turn(a, b, d)
	return ((s1 > 0 && s2 < 0) || (s1 < 0 && s2 > 0)) && ((s3 > 0 && s4 < 0) || (s3 < 0 && s4 > 0))
}

// turn is which way the corner a-b-p bends, and zero when the three are in a
// line. It is the test the composition checker uses to judge a crossing, which
// is deliberate: a router choosing a path to avoid one and a rule deciding
// whether one happened must agree on what a crossing is.
func turn(a, b, p point) float64 {
	return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
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
	byID := make(map[string]placedBox, len(boxes))
	for _, b := range boxes {
		byID[b.ID] = b
	}
	ordered := make([]diagram.Connection, len(level.Connections))
	copy(ordered, level.Connections)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	// Routes go in order of how much choice they have, least first.
	//
	// A straight or stacked route has one shape and one position on each box
	// side, because both its ends have to move together. Letting the freer ones
	// allocate first meant a marker, narrow enough to hold a single position,
	// could have it taken by a route that had somewhere else to go, and the
	// straight route between two markers was then refused.
	//
	// The same argument runs on past the rigid ones. A detour may climb any
	// vertical channel it likes and picks the one that crosses the least of
	// what is drawn, so the more that is drawn when it picks, the better it
	// picks. Going last is what gives it something to look at.
	freedom := func(c diagram.Connection) int {
		from, ok := byID[c.From]
		if !ok {
			return 0
		}
		to, ok := byID[c.To]
		if !ok {
			return 0
		}
		switch {
		case from.Row == to.Row && abs(from.Col-to.Col) == 1,
			abs(from.Row-to.Row) == 1 && from.Col == to.Col:
			return 0 // one shape, one place on each box side
		case abs(from.Row-to.Row) == 1, from.Row == to.Row:
			return 1 // one channel to travel in, and a lane to pick in it
		default:
			return 2 // a lane in two channels, and any vertical channel it likes
		}
	}
	// Within a tier, the shortest first. A route between neighbours has fewer
	// ways to go than one across the page, and the same argument that puts the
	// rigid ones first puts the short ones ahead of the long.
	span := func(c diagram.Connection) int {
		from, ok := byID[c.From]
		if !ok {
			return 0
		}
		to, ok := byID[c.To]
		if !ok {
			return 0
		}
		return abs(from.Row-to.Row)*len(byID) + abs(from.Col-to.Col)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return span(ordered[i]) < span(ordered[j]) })
	sort.SliceStable(ordered, func(i, j int) bool { return freedom(ordered[i]) < freedom(ordered[j]) })

	r := newRouter(g, boxes)
	var out []routed
	refused := map[string]error{}
	for _, c := range ordered {
		path, err := r.route(c)
		if err != nil {
			refused[c.ID] = err
			continue
		}
		r.drawn = append(r.drawn, path)
		out = append(out, path)
	}
	// Allocation order is not drawing order. The page is emitted by id so that
	// the same document always produces the same bytes.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, refused
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
