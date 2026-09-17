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
