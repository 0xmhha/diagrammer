package render

import (
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// TestADetourGoesRoundWhatIsAlreadyDrawn is the case choosing a channel exists
// for.
//
// A route between two boxes at either end of a row runs the width of the page
// in the channel beneath them. A detour crossing that row has to climb through
// that channel somewhere, and climbing where the run is crosses it. The channel
// beside the row does not, so that is the one to take.
//
// Taking the channel left of the target whatever else was there was the earlier
// behaviour, and on this repository's own diagram four detours climbed straight
// through one 560px run.
func TestADetourGoesRoundWhatIsAlreadyDrawn(t *testing.T) {
	level := diagram.Level{
		ID: "overview", Title: "Round it", Grid: diagram.Grid{Rows: 3, Cols: 3},
		Boxes: []diagram.Box{
			{ID: "src", Label: "Src", Row: 0, Col: 0},
			{ID: "near", Label: "Near", Row: 1, Col: 0},
			{ID: "far", Label: "Far", Row: 1, Col: 2},
			{ID: "dst", Label: "Dst", Row: 2, Col: 1},
		},
		Connections: []diagram.Connection{
			// Across the whole row, in the channel beneath it.
			{ID: "across", From: "near", To: "far", Kind: diagram.KindDependency},
			// Down past that row, and it has to climb somewhere.
			{ID: "down", From: "src", To: "dst", Kind: diagram.KindDependency},
		},
		Accounting: diagram.Accounting{Proven: 2, Drawn: 2},
	}
	doc := &diagram.Document{
		SchemaVersion: 1, Family: diagram.FamilyComponent,
		Meta:       diagram.Meta{Title: "Round it"},
		Levels:     []diagram.Level{level},
		Accounting: diagram.Accounting{Proven: 2, Drawn: 2},
	}

	page, err := Build(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(page.Dropped) != 0 {
		t.Errorf("both routes fit on this page and %d were left out: %+v", len(page.Dropped), page.Dropped)
	}

	scenes, err := artifact.Parse([]byte(page.HTML()))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}
	drawn := 0
	for i := range scenes {
		drawn += len(scenes[i].Routes)
		if n := len(invariant.Crossings(&scenes[i])); n != 0 {
			t.Errorf("%d crossing(s) left on a page where one of the two had somewhere else to go", n)
		}
		for _, p := range invariant.CompositionFor(&scenes[i]) {
			t.Errorf("%s", p)
		}
	}
	if drawn != 2 {
		t.Errorf("%d of 2 routes are on the page", drawn)
	}
}

// TestLinesLeaveInTheOrderTheyGo is the other half of keeping a run short.
//
// Positions along a box edge used to be handed out in the order routes asked
// for them. A route to the far right that was handed the leftmost position had
// to travel the width of its own box before it started, across everything else
// leaving that side. They are now laid along the edge in the order of where
// they go, which is decided for the whole side before any of it is drawn.
func TestLinesLeaveInTheOrderTheyGo(t *testing.T) {
	// One box above three, joined to each. Left to right below, so left to
	// right along the edge above.
	level := diagram.Level{
		ID: "overview", Title: "Fan out", Grid: diagram.Grid{Rows: 2, Cols: 3},
		Boxes: []diagram.Box{
			{ID: "hub", Label: "Hub", Row: 0, Col: 1},
			{ID: "left", Label: "Left", Row: 1, Col: 0},
			{ID: "mid", Label: "Mid", Row: 1, Col: 1},
			{ID: "right", Label: "Right", Row: 1, Col: 2},
		},
		Connections: []diagram.Connection{
			{ID: "r.right", From: "hub", To: "right", Kind: diagram.KindDependency},
			{ID: "r.left", From: "hub", To: "left", Kind: diagram.KindDependency},
			{ID: "r.mid", From: "hub", To: "mid", Kind: diagram.KindDependency},
		},
		Accounting: diagram.Accounting{Proven: 3, Drawn: 3},
	}
	doc := &diagram.Document{
		SchemaVersion: 1, Family: diagram.FamilyComponent,
		Meta:       diagram.Meta{Title: "Fan out"},
		Levels:     []diagram.Level{level},
		Accounting: diagram.Accounting{Proven: 3, Drawn: 3},
	}

	page, err := Build(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	scenes, err := artifact.Parse([]byte(page.HTML()))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}

	leaves := map[string]float64{}
	for i := range scenes {
		for _, r := range scenes[i].Routes {
			if len(r.Points) > 0 && r.From == "hub" {
				leaves[r.To] = r.Points[0].X
			}
		}
	}
	for _, want := range []string{"left", "mid", "right"} {
		if _, ok := leaves[want]; !ok {
			t.Fatalf("the line to %s was not drawn, so the order cannot be read", want)
		}
	}
	if !(leaves["left"] < leaves["mid"] && leaves["mid"] < leaves["right"]) {
		t.Errorf("the lines leave the hub at x %.0f, %.0f and %.0f for left, mid and right, "+
			"which is not the order they are going in",
			leaves["left"], leaves["mid"], leaves["right"])
	}
}
