package render

import (
	"fmt"
	"sort"
	"testing"
)

// How much of a model a drawing actually shows.
//
// The accounting invariant says nothing about this: a page that draws one
// relationship and records nine is perfectly consistent and perfectly useless.
// So the ratio is measured, and held above a floor.
//
// A floor rather than an equality, deliberately. The rules that decide what to
// draw are the contract; the ones that decide where to put things are free to
// improve. Freezing the ratio would turn every layout improvement into a
// failing test, which is how a tuning knob becomes an API.

// drawnFloor is the share of a model's relationships a drawing must show.
//
// Measured rather than chosen: see docs/thresholds.md for what the layout
// scored before and after the placement work, and why this is the line.
const drawnFloor = 0.80

func ratio(page *Page) float64 {
	if page.Accounting.Proven == 0 {
		return 1
	}
	return float64(page.Accounting.Drawn) / float64(page.Accounting.Proven)
}

// TestDrawnRatio reports what every family manages on every fixture, and fails
// when one falls below the floor.
//
// It prints the whole table whether or not it passes, because the number worth
// watching is the trend and a test that only speaks up when it breaks hides it.
func TestDrawnRatio(t *testing.T) {
	type row struct {
		name          string
		proven, drawn int
		ratio         float64
	}
	var rows []row

	for _, f := range families() {
		page, err := Build(composeFor(t, f.Family, f.Fixture))
		if err != nil {
			t.Fatalf("render %s from %s: %v", f.Family, f.Fixture, err)
		}
		rows = append(rows, row{
			name:   fmt.Sprintf("%s / %s", f.Family, f.Fixture),
			proven: page.Accounting.Proven,
			drawn:  page.Accounting.Drawn,
			ratio:  ratio(page),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ratio < rows[j].ratio })

	t.Log("drawn ratio, worst first:")
	for _, r := range rows {
		t.Logf("  %-34s %3d/%-3d  %5.1f%%", r.name, r.drawn, r.proven, r.ratio*100)
	}
	for _, r := range rows {
		if r.ratio < drawnFloor {
			t.Errorf("%s draws %.1f%% of what the model proves, below the %.0f%% floor",
				r.name, r.ratio*100, drawnFloor*100)
		}
	}
}
