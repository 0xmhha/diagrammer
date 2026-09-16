package render

import (
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// TestOneRouteCanSettleManyCrossings is the case the choice exists for.
//
// One route crossing three others is four conflicts and one answer: take out
// the one in the middle. Blaming whichever route of each pair sorts first takes
// out three, and the three it takes out are the ones that were fine.
func TestOneRouteCanSettleManyCrossings(t *testing.T) {
	// "a" crosses b, c and d; nothing else crosses anything.
	got := fewestCrossingRoutes([][2]string{{"a", "b"}, {"a", "c"}, {"a", "d"}})
	if len(got) != 1 {
		t.Fatalf("leaving out %d routes settles three crossings that one route causes: %+v", len(got), got)
	}
	if got[0].route != "a" {
		t.Errorf("left out %q, which is one of the three that were fine; %q is the one in the middle",
			got[0].route, "a")
	}
	if got[0].crosses == "" {
		t.Error("the record does not say what it crossed, so a reader has a route id and no reason")
	}
}

// TestTheChoiceIsTheSameEveryTime guards the part that is easy to lose. The
// greedy step reads a map of degrees, and a page whose text moves between runs
// is a page nobody can diff.
func TestTheChoiceIsTheSameEveryTime(t *testing.T) {
	pairs := [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}, {"d", "e"}, {"a", "e"}}
	first := fewestCrossingRoutes(pairs)
	for range 20 {
		again := fewestCrossingRoutes(pairs)
		if len(again) != len(first) {
			t.Fatalf("one run left out %d routes and another %d", len(first), len(again))
		}
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("run to run, position %d changed from %+v to %+v", i, first[i], again[i])
			}
		}
	}
}

// TestATangledPageStillSettles carries the same thing end to end. tangle is the
// only fixture here that produces crossings in quantity, which is what every
// real project produces and what the choice was written for.
//
// It is deliberately worse than an ordinary model, so it is not in the
// drawn-ratio table; what it has to show is that a page full of crossings is
// resolved into one with none, and that everything taken out is recorded.
func TestATangledPageStillSettles(t *testing.T) {
	page, err := Build(composeFor(t, diagram.FamilyComponent, "tangle"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	scenes, err := artifact.Parse([]byte(page.HTML()))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}

	crossings := 0
	for i := range scenes {
		crossings += len(invariant.Crossings(&scenes[i]))
	}
	if crossings != 0 {
		t.Errorf("%d crossings are left on the page the checker was supposed to have settled", crossings)
	}

	left := 0
	for _, d := range page.Dropped {
		if d.Rule == invariant.RuleCrossing {
			left++
		}
	}
	if left == 0 {
		t.Error("nothing was left out for crossing, so the fixture no longer tangles and the choice is untested")
	}
	if page.Accounting.Drawn+page.Accounting.Dropped != page.Accounting.Proven {
		t.Errorf("drawn %d plus dropped %d is not proven %d",
			page.Accounting.Drawn, page.Accounting.Dropped, page.Accounting.Proven)
	}
	t.Logf("tangle: %d drawn, %d recorded, %d of them for crossing",
		page.Accounting.Drawn, page.Accounting.Dropped, left)
}
