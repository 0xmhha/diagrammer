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

	next := map[string]int{} // band and row to the next free column
	cells := make(map[string][2]int, len(ordered))
	for _, id := range ordered {
		b, r := bandOf(id), rank[id]
		key := b + "\x00" + itoa(r)
		col := start[b] + next[key]
		next[key]++
		cells[id] = [2]int{r, col}
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
