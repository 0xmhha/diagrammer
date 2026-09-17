package render

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/diagram"
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

// TestTheChannelCeilingRefusesInTheOpen holds the other end of the same
// decision.
//
// Channels follow demand, so a hub no longer loses routes to a channel sized
// for the average. That widening has a ceiling, and a ceiling nothing has ever
// been seen to hit is indistinguishable from one that was never written.
// hub-overflow asks for more than the ceiling allows on purpose.
//
// It is deliberately past the limit, so it is not in the ratio table above: the
// floor asks whether ordinary models are drawn well, and this one is not an
// ordinary model. What it has to show is that going past the limit is refused
// where a reader can see it rather than quietly mangled.
func TestTheChannelCeilingRefusesInTheOpen(t *testing.T) {
	page, err := Build(composeFor(t, diagram.FamilyComponent, "hub-overflow"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	lanes := 0
	for _, d := range page.Dropped {
		if strings.Contains(d.Why, "no lane left") {
			lanes++
		}
	}
	if lanes == 0 {
		t.Error("nothing was refused for want of a lane, so the ceiling on channel widening never fired; " +
			"either the fixture no longer exceeds it or the ceiling is gone")
	}
	if page.Accounting.Drawn+page.Accounting.Dropped != page.Accounting.Proven {
		t.Errorf("drawn %d plus dropped %d is not proven %d",
			page.Accounting.Drawn, page.Accounting.Dropped, page.Accounting.Proven)
	}
	if len(page.Dropped) != page.Accounting.Dropped {
		t.Errorf("accounting says %d dropped, %d are recorded", page.Accounting.Dropped, len(page.Dropped))
	}
	t.Logf("hub-overflow: %d drawn, %d recorded, %d of them for want of a lane",
		page.Accounting.Drawn, page.Accounting.Dropped, lanes)
}
