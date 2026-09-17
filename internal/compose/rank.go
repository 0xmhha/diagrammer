package compose

import (
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// Laying a page out by dependency depth.
//
// A page used to fill a square grid in walk order: whatever the walk reached
// first took the next cell. Nothing about the picture then said which way
// anything depended on anything, and the router paid for it. A relationship
// between two boxes a walk happened to put three rows and two columns apart is
// a detour across the page, and detours are what cross each other.
//
// So the row a box sits on is its depth in the dependency graph, which is what
// makes a component diagram read downward: whatever nothing depends on is at
// the top, and what it rests on is beneath it.
//
// The columns are grouped by band. A region's members are given a range of
// columns of their own, wide enough for the most it has on any one row, so a
// band frames a block of the page rather than a scatter of cells with other
// people's boxes between them.

// ranksOf assigns each id a dependency depth by longest path.
//
// A cycle has no depth, so the edge that closes one is left out of the
// reckoning. Which edge that is depends on the order ids are visited, and they
// are visited in sorted order for that reason: a page whose rows move between
// runs is a page nobody can diff.
func ranksOf(ids []string, edges [][2]string) map[string]int {
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	out := make(map[string][]string, len(ids))
	for _, e := range edges {
		if !present[e[0]] || !present[e[1]] || e[0] == e[1] {
			continue
		}
		out[e[0]] = append(out[e[0]], e[1])
	}
	for id := range out {
		sort.Strings(out[id])
	}

	rank := make(map[string]int, len(ids))
	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(ids))
	var depth func(id string) int
	depth = func(id string) int {
		switch state[id] {
		case done:
			return rank[id]
		case onStack:
			// The edge that led here closes a cycle. Following it would ask
			// this question again and never stop, so it contributes nothing.
			return 0
		}
		state[id] = onStack
		best := 0
		for _, next := range out[id] {
			if d := depth(next) + 1; d > best {
				best = d
			}
		}
		state[id] = done
		rank[id] = best
		return best
	}

	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	for _, id := range sorted {
		depth(id)
	}

	// depth counts downward from a box to the deepest thing it rests on, so the
	// most dependent box has the largest number. A reader expects the opposite,
	// so the rows are turned over.
	deepest := 0
	for _, d := range rank {
		if d > deepest {
			deepest = d
		}
	}
	for id, d := range rank {
		rank[id] = deepest - d
	}
	return rank
}

// cellsByRank turns ranks and bands into a row and column for every id, and the
// grid that holds them.
//
// Within a row, members of one band sit together in the columns that band owns.
// Order inside a band follows the order the caller gives, which is the
// adjacency walk, so boxes joined by a line stay near each other.
func cellsByRank(ordered []string, rank map[string]int, bandOf func(string) string) (map[string][2]int, diagram.Grid) {
	rows := 0
	for _, id := range ordered {
		if rank[id]+1 > rows {
			rows = rank[id] + 1
		}
	}
	if rows < 1 {
		rows = 1
	}

	// Bands are laid left to right in the order they first appear, so the same
	// page always puts the same band in the same place.
	var bands []string
	seen := map[string]bool{}
	for _, id := range ordered {
		b := bandOf(id)
		if !seen[b] {
			seen[b] = true
			bands = append(bands, b)
		}
	}

	// A band is as wide as the most it holds on any one row.
	width := map[string]int{}
	for _, b := range bands {
		perRow := make([]int, rows)
		for _, id := range ordered {
			if bandOf(id) == b {
				perRow[rank[id]]++
			}
		}
		for _, n := range perRow {
			if n > width[b] {
				width[b] = n
			}
		}
	}
	start := map[string]int{}
	cols := 0
	for _, b := range bands {
		start[b] = cols
		cols += width[b]
	}
	if cols < 1 {
		cols = 1
	}

	// Members of one band on one row are spread across the columns that band
	// owns rather than packed against its left edge.
	//
	// A row with two boxes in a band five columns wide used to leave three
	// columns empty on the right and put the two boxes side by side on the
	// left. Everything they pointed at on the row below was spread over all
	// five, so both boxes had to fan their lines sideways across the page
	// before they could descend. Spread out, each sits over what it points at
	// and stage 4 can widen it into the room beside it.
	perRow := map[string][]string{}
	for _, id := range ordered {
		key := bandOf(id) + "\x00" + itoa(rank[id])
		perRow[key] = append(perRow[key], id)
	}
	cells := make(map[string][2]int, len(ordered))
	for _, id := range ordered {
		b, r := bandOf(id), rank[id]
		key := b + "\x00" + itoa(r)
		members := perRow[key]
		i := 0
		for n, m := range members {
			if m == id {
				i = n
				break
			}
		}
		cells[id] = [2]int{r, start[b] + i*width[b]/len(members)}
	}
	return cells, diagram.Grid{Rows: rows, Cols: cols}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// orderWithin reorders each row so that boxes joined by a line sit near each
// other in the rows above and below.
//
// The rows say which way things depend; they say nothing about which column
// anything takes, and a column chosen without looking at the neighbouring rows
// is what makes a run long. A long run is the thing that crosses: on this
// repository's own diagram one line travelled 560px along a row channel, and
// every route descending through that channel met it.
//
// This is the barycentre heuristic. A box wants to sit above the average of
// what it points at and below the average of what points at it, so each row is
// sorted by that average, and the sweep is repeated in both directions because
// moving one row invalidates the averages of its neighbours. The arrangement
// with the fewest crossings is kept, which is why doing it badly cannot make
// the page worse than not doing it.
//
// Bands are respected: a box only ever moves among the columns its own band
// owns, because a band that gave up its block would stop being a frame.
func orderWithin(ordered []string, rank map[string]int, bandOf func(string) string, edges [][2]string) []string {
	present := make(map[string]bool, len(ordered))
	for _, id := range ordered {
		present[id] = true
	}
	var live [][2]string
	up := make(map[string][]string, len(ordered))
	down := make(map[string][]string, len(ordered))
	for _, e := range edges {
		if !present[e[0]] || !present[e[1]] || e[0] == e[1] {
			continue
		}
		live = append(live, e)
		down[e[0]] = append(down[e[0]], e[1])
		up[e[1]] = append(up[e[1]], e[0])
	}
	if len(live) == 0 {
		return ordered
	}

	best := append([]string(nil), ordered...)
	bestCrossings := crossingProxy(best, rank, bandOf, live)
	current := append([]string(nil), ordered...)
	settle := func() {
		current = transpose(current, rank, bandOf, live)
		if n := crossingProxy(current, rank, bandOf, live); n < bestCrossings {
			bestCrossings = n
			best = append(best[:0], current...)
		}
	}
	settle()

	// Four passes each way. Beyond that the arrangement stopped moving on every
	// fixture measured, and a heuristic that is still wandering after eight
	// sweeps is not going to settle on the ninth.
	const sweeps = 4
	for i := range sweeps * 2 {
		side := down
		if i%2 == 1 {
			side = up
		}
		current = sweep(current, rank, bandOf, side)
		settle()
	}
	return best
}

// sweep sorts every row by where each box's neighbours on one side sit.
func sweep(ordered []string, rank map[string]int, bandOf func(string) string, side map[string][]string) []string {
	position := columnsOf(ordered, rank, bandOf)
	key := func(id string) float64 {
		nb := side[id]
		if len(nb) == 0 {
			// Nothing to be near. It keeps the place it had, so a box with no
			// neighbours on this side does not drift about between sweeps.
			return float64(position[id])
		}
		sum := 0
		for _, n := range nb {
			sum += position[n]
		}
		return float64(sum) / float64(len(nb))
	}

	out := append([]string(nil), ordered...)
	sort.SliceStable(out, func(a, b int) bool {
		x, y := out[a], out[b]
		if rank[x] != rank[y] {
			return rank[x] < rank[y]
		}
		if bandOf(x) != bandOf(y) {
			return position[x] < position[y]
		}
		kx, ky := key(x), key(y)
		if kx != ky {
			return kx < ky
		}
		return x < y
	})
	return out
}

// transpose swaps neighbouring boxes whenever the swap crosses fewer lines.
//
// The barycentre sweep moves a box to where its neighbours average out, which
// is a good guess and only a guess: an average says nothing about two boxes
// whose neighbours average to the same place, and nothing about a box whose
// neighbours pull it two ways at once. Trying the swap answers both, because it
// asks the question the sweep was approximating.
//
// Measured on this repository's own component diagram the sweep alone left 29
// pairs of lines ordered one way at the top and the other at the bottom, and
// swapping brought it to 19. Nineteen is what this graph costs in this
// layering: no arrangement of the columns removes them, because two boxes on one
// row each reach across the other's target.
//
// Only neighbours in one row and one band are ever swapped, so a band keeps its
// block and a row keeps its depth.
func transpose(ordered []string, rank map[string]int, bandOf func(string) string, edges [][2]string) []string {
	out := append([]string(nil), ordered...)
	best := crossingProxy(out, rank, bandOf, edges)
	// Enough passes to let a box travel the length of its row, and no more. A
	// pass that changes nothing ends it.
	for range len(out) {
		improved := false
		for i := 0; i+1 < len(out); i++ {
			a, b := out[i], out[i+1]
			if rank[a] != rank[b] || bandOf(a) != bandOf(b) {
				continue
			}
			out[i], out[i+1] = b, a
			if n := crossingProxy(out, rank, bandOf, edges); n < best {
				best, improved = n, true
				continue
			}
			out[i], out[i+1] = a, b
		}
		if !improved {
			break
		}
	}
	return out
}

// columnsOf is which column each box would take, given an order.
//
// This is the number the ordering is trying to improve, and it is not the
// box's place in the list: columns are handed out within a row and a band, so
// two boxes far apart in the list can be neighbours on the page and two boxes
// side by side in the list can be in different rows entirely. Counting
// crossings by list position measured something the page does not have, which
// is why swapping boxes appeared to change nothing.
func columnsOf(ordered []string, rank map[string]int, bandOf func(string) string) map[string]int {
	cells, _ := cellsByRank(ordered, rank, bandOf)
	out := make(map[string]int, len(cells))
	for id, cell := range cells {
		out[id] = cell[1]
	}
	return out
}

// crossingProxy counts the pairs of lines that would cross, from the cells
// alone.
//
// Two shapes have to be told apart, and the first version of this told them
// apart badly. A line between two rows meets another between the same two rows
// when their ends are ordered one way at the top and the other at the bottom,
// which is the classic test. A line between two boxes on **one** row is nothing
// like that: it drops into the channel below the row, runs along it and comes
// back, so what it occupies is an interval of columns, and two of them cross
// when their intervals interleave rather than when one contains the other.
//
// Counting the second kind with the first kind's test was a real error, not an
// approximation. On a use case page, where an actor's associations all sit on
// one row, it modelled almost nothing that was there, and arranging the page to
// improve it made the page worse.
func crossingProxy(ordered []string, rank map[string]int, bandOf func(string) string, edges [][2]string) int {
	position := columnsOf(ordered, rank, bandOf)
	n := 0
	for i := range edges {
		for j := i + 1; j < len(edges); j++ {
			if cross(edges[i], edges[j], rank, position) {
				n++
			}
		}
	}
	return n
}

// cross reports whether two lines would meet, given where their ends sit.
func cross(a, b [2]string, rank, position map[string]int) bool {
	if a[0] == b[0] || a[0] == b[1] || a[1] == b[0] || a[1] == b[1] {
		return false // two lines meeting at a box they both touch is expected
	}
	aFlat, aRow := flat(a, rank)
	bFlat, bRow := flat(b, rank)

	switch {
	case aFlat && bFlat:
		if aRow != bRow {
			return false // different rows, different channels
		}
		return interleave(span(a, position), span(b, position))
	case aFlat:
		return spans(b, rank, aRow) && within(span(a, position), position[endOn(b, rank, aRow)])
	case bFlat:
		return spans(a, rank, bRow) && within(span(b, position), position[endOn(a, rank, bRow)])
	default:
		top := position[a[0]] - position[b[0]]
		bottom := position[a[1]] - position[b[1]]
		if top == 0 || bottom == 0 {
			return false // one pair of ends shares a column, so nothing crosses
		}
		return (top > 0) != (bottom > 0)
	}
}

// flat reports whether both ends of a line are on one row, and which.
func flat(e [2]string, rank map[string]int) (bool, int) {
	return rank[e[0]] == rank[e[1]], rank[e[0]]
}

// spans reports whether a line has an end on the given row.
func spans(e [2]string, rank map[string]int, row int) bool {
	return rank[e[0]] == row || rank[e[1]] == row
}

// endOn is the end of a line that sits on the given row.
func endOn(e [2]string, rank map[string]int, row int) string {
	if rank[e[0]] == row {
		return e[0]
	}
	return e[1]
}

func span(e [2]string, position map[string]int) [2]int {
	lo, hi := position[e[0]], position[e[1]]
	if lo > hi {
		lo, hi = hi, lo
	}
	return [2]int{lo, hi}
}

func within(s [2]int, at int) bool { return at > s[0] && at < s[1] }

// interleave reports whether two column intervals overlap partially. One
// entirely inside the other is two lines nested in the same channel, which the
// lanes keep apart; one end of each inside the other is a crossing.
func interleave(a, b [2]int) bool {
	return (within(a, b[0]) && !within(a, b[1])) || (within(a, b[1]) && !within(a, b[0]))
}
