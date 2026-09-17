package compose

import (
	"testing"
)

// TestDependenciesPointDownward is what the rows are for. A component diagram
// is read top to bottom, and a page where that says nothing is a page whose
// arrangement carries no information at all.
func TestDependenciesPointDownward(t *testing.T) {
	ids := []string{"cli", "command", "render", "artifact", "diagram"}
	edges := [][2]string{
		{"cli", "command"},
		{"command", "render"},
		{"render", "artifact"},
		{"render", "diagram"},
	}
	rank := ranksOf(ids, edges)
	for _, e := range edges {
		if rank[e[0]] >= rank[e[1]] {
			t.Errorf("%s depends on %s but sits on row %d against row %d",
				e[0], e[1], rank[e[0]], rank[e[1]])
		}
	}
	if rank["cli"] != 0 {
		t.Errorf("nothing depends on cli, so it belongs at the top; it is on row %d", rank["cli"])
	}
}

// TestACycleStillGetsRows is the case longest-path layering has no answer for.
//
// Two packages that import each other have no depth between them. The edge that
// closes the cycle is left out of the reckoning rather than followed, because
// following it never ends; what matters here is that every box still lands
// somewhere and that it lands in the same place every run.
func TestACycleStillGetsRows(t *testing.T) {
	ids := []string{"a", "b", "c"}
	edges := [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}}
	first := ranksOf(ids, edges)
	if len(first) != len(ids) {
		t.Fatalf("%d of %d boxes were given a row", len(first), len(ids))
	}
	for range 20 {
		again := ranksOf(ids, edges)
		for _, id := range ids {
			if again[id] != first[id] {
				t.Fatalf("%s moved from row %d to row %d between runs", id, first[id], again[id])
			}
		}
	}
}

// TestABandKeepsItsOwnColumns is what makes a region a frame around a block
// rather than a box drawn round a scatter.
//
// Two bands sharing columns would each have to reach across the other's boxes,
// and the two frames would overlap, which says the two groups overlap.
func TestABandKeepsItsOwnColumns(t *testing.T) {
	ordered := []string{"cli", "command", "render", "artifact", "docscheck"}
	band := map[string]string{
		"cli": "cmd", "command": "internal", "render": "internal",
		"artifact": "internal", "docscheck": "internal",
	}
	rank := ranksOf(ordered, [][2]string{
		{"cli", "command"}, {"command", "render"}, {"render", "artifact"},
	})
	cells, grid := cellsByRank(ordered, rank, func(id string) string { return band[id] })

	columns := map[string]map[int]bool{}
	for id, cell := range cells {
		b := band[id]
		if columns[b] == nil {
			columns[b] = map[int]bool{}
		}
		columns[b][cell[1]] = true
		if cell[0] < 0 || cell[0] >= grid.Rows || cell[1] < 0 || cell[1] >= grid.Cols {
			t.Errorf("%s is placed at row %d column %d, outside a %dx%d grid",
				id, cell[0], cell[1], grid.Rows, grid.Cols)
		}
	}
	for a := range columns {
		for b := range columns {
			if a >= b {
				continue
			}
			for col := range columns[a] {
				if columns[b][col] {
					t.Errorf("bands %q and %q both use column %d, so their frames would overlap", a, b, col)
				}
			}
		}
	}

	// Two boxes in one cell is the other way a grid goes wrong.
	seen := map[[2]int]string{}
	for id, cell := range cells {
		if other, taken := seen[cell]; taken {
			t.Errorf("%s and %s are both at row %d column %d", id, other, cell[0], cell[1])
		}
		seen[cell] = id
	}
}

// TestSwappingNeighboursIsMeasuredInColumns is the bug that made the whole
// arrangement step do nothing.
//
// Columns are handed out within a row and a band, so two boxes far apart in the
// order can be neighbours on the page. Counting crossings by place in the order
// measured something the page does not have: every swap looked like no
// improvement and the step returned what it was given.
func TestSwappingNeighboursIsMeasuredInColumns(t *testing.T) {
	// Two lines that cross: left points right and right points left.
	ordered := []string{"a", "b", "x", "y"}
	rank := map[string]int{"a": 0, "b": 0, "x": 1, "y": 1}
	band := func(string) string { return "" }
	edges := [][2]string{{"a", "y"}, {"b", "x"}}

	if n := crossingProxy(ordered, rank, band, edges); n != 1 {
		t.Fatalf("two lines that swap sides count as %d crossings, not 1", n)
	}
	got := orderWithin(ordered, rank, band, edges)
	if n := crossingProxy(got, rank, band, edges); n != 0 {
		t.Errorf("swapping one pair removes the crossing and %d were left: %v", n, got)
	}

	// Rows are never mixed and a box stays on its own.
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	if len(seen) != len(ordered) {
		t.Errorf("the arrangement lost or duplicated a box: %v", got)
	}
}

// TestARowSpreadsAcrossItsBand is what gives a box room over what it points at.
//
// A row with two boxes in a band five columns wide used to put them side by
// side on the left and leave three columns empty on the right. Everything they
// pointed at on the row below was spread across all five, so both had to fan
// their lines sideways before they could descend, and sideways is where lines
// cross.
func TestARowSpreadsAcrossItsBand(t *testing.T) {
	ordered := []string{"top", "other", "a", "b", "c", "d", "e"}
	rank := map[string]int{"top": 0, "other": 0, "a": 1, "b": 1, "c": 1, "d": 1, "e": 1}
	band := func(string) string { return "" }
	cells, grid := cellsByRank(ordered, rank, band)

	if grid.Cols != 5 {
		t.Fatalf("five boxes on the widest row need five columns, not %d", grid.Cols)
	}
	top, other := cells["top"][1], cells["other"][1]
	if top >= other {
		t.Fatalf("the two boxes on the thin row are at columns %d and %d, out of order", top, other)
	}
	if other-top < 2 {
		t.Errorf("two boxes on a five-column row sit at columns %d and %d, packed together "+
			"rather than spread over what they point at", top, other)
	}

	// Nothing shares a cell, whatever the spreading does.
	seen := map[[2]int]string{}
	for id, cell := range cells {
		if was, taken := seen[cell]; taken {
			t.Errorf("%s and %s are both at row %d column %d", id, was, cell[0], cell[1])
		}
		seen[cell] = id
	}
}

// Two lines on one row are nothing like two lines between two rows, and telling
// them apart is what the count exists to do.
//
// A line between rows meets another between the same rows when their ends are
// ordered one way at the top and the other at the bottom. A line between two
// boxes on **one** row drops into the channel below it, runs along, and comes
// back: what it occupies is an interval of columns. Two of those cross when
// their intervals interleave, and not when one sits inside the other, which the
// lanes keep apart.
//
// Counting the second with the first's test was a real error rather than an
// approximation. On a use case page, where an actor's associations all sit on
// one row, it modelled almost nothing that was there.

func columnsAt(cells map[string][2]int) (map[string]int, map[string]int) {
	rank := map[string]int{}
	position := map[string]int{}
	for id, cell := range cells {
		rank[id], position[id] = cell[0], cell[1]
	}
	return rank, position
}

func TestTwoLinesOnOneRowCrossWhenTheyInterleave(t *testing.T) {
	// Four boxes in a row: a b c d.
	rank, position := columnsAt(map[string][2]int{
		"a": {0, 0}, "b": {0, 1}, "c": {0, 2}, "d": {0, 3},
	})

	for _, c := range []struct {
		name string
		x, y [2]string
		want bool
	}{
		{"interleaved: a-c and b-d", [2]string{"a", "c"}, [2]string{"b", "d"}, true},
		{"nested: a-d and b-c", [2]string{"a", "d"}, [2]string{"b", "c"}, false},
		{"apart: a-b and c-d", [2]string{"a", "b"}, [2]string{"c", "d"}, false},
		{"written the other way round", [2]string{"c", "a"}, [2]string{"d", "b"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := cross(c.x, c.y, rank, position); got != c.want {
				t.Errorf("cross(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
			}
		})
	}
}

func TestALineFromAnotherRowCrossesARunItLandsInside(t *testing.T) {
	// a b c on row 0, and d below b.
	rank, position := columnsAt(map[string][2]int{
		"a": {0, 0}, "b": {0, 1}, "c": {0, 2}, "d": {1, 1}, "e": {1, 3},
	})

	// b sits strictly inside a-c, so the line that climbs to it crosses the run.
	if !cross([2]string{"a", "c"}, [2]string{"d", "b"}, rank, position) {
		t.Error("a line arriving inside a same-row run does not cross it")
	}
	// e is outside a-c, so that one does not.
	if cross([2]string{"a", "c"}, [2]string{"d", "e"}, rank, position) {
		t.Error("a line that never enters the run's columns crosses it")
	}
	// Sharing a box is not a crossing, whichever shape the two lines are.
	if cross([2]string{"a", "c"}, [2]string{"d", "c"}, rank, position) {
		t.Error("two lines meeting at a box they both touch count as crossing")
	}
}
