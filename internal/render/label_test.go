package render

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// TestTextIsFittedToTheRoomItHas is the first half of the label bargain: text
// is measured against the space it has, as a box's text always was.
//
// A page drew "Merge(graphs) -> 536 nodes, 477 edges" at full width on a
// sequence diagram whose lifelines were 220px apart, so the name of one message
// lay across three lifelines and the text on them. Nothing caught it: the rule
// measured the label as its centre point, and the centre of a long string
// clears everything.
func TestTextIsFittedToTheRoomItHas(t *testing.T) {
	page := buildFixture(t, "order-service")
	scenes, err := artifact.Parse([]byte(page.HTML()))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}
	for i := range scenes {
		for _, r := range scenes[i].Routes {
			if r.LabelText == "" {
				continue
			}
			if r.LabelBounds.W == 0 {
				t.Errorf("%s carries text with no measured width, so nothing can check where it sits", r.ID)
			}
			if r.LabelSize <= 0 {
				t.Errorf("%s carries text with no size, so its width cannot be reproduced", r.ID)
			}
			if want := textWidth(r.LabelText, r.LabelSize); abs64(r.LabelBounds.W-want) > 0.5 {
				t.Errorf("%s says its text is %.1fpx wide; %q at %.1fpx is %.1fpx",
					r.ID, r.LabelBounds.W, r.LabelText, r.LabelSize, want)
			}
		}
	}
}

func abs64(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// TestALineKeepsItsPlaceWhenItsTextCannot is the second half.
//
// Losing a relationship because its name had nowhere to go was the earlier
// behaviour and it cost more than it saved: a reader of an unnamed line can
// still see that two things are joined, and a reader of no line cannot. So the
// text goes and the line stays, and the page says which names it could not
// write.
func TestALineKeepsItsPlaceWhenItsTextCannot(t *testing.T) {
	page, err := Build(composeFor(t, diagram.FamilyState, "crowded-text"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(page.Unwritten) == 0 {
		t.Fatal("every name found room on the fixture written to have none, so the record never fires")
	}
	for _, u := range page.Unwritten {
		if u.Route == "" || u.Text == "" || u.Why == "" || u.Level == "" {
			t.Errorf("a name is recorded without saying which line, where, or what it said: %+v", u)
		}
		for _, d := range page.Dropped {
			if d.Route == u.Route {
				t.Errorf("%s is recorded both as undrawn and as drawn without its text", u.Route)
			}
		}
	}

	// The record is on the page rather than in a log the reader never sees.
	html := page.HTML()
	if !strings.Contains(html, "Drawn without their text") {
		t.Error("the page does not say that some lines were drawn unnamed")
	}
	scenes, err := artifact.Parse([]byte(html))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}
	drawn := 0
	for i := range scenes {
		drawn += len(scenes[i].Routes)
		for _, p := range invariant.CompositionFor(&scenes[i]) {
			t.Errorf("%s", p)
		}
	}
	// The line is still on the page: losing a name is not losing a
	// relationship, and the accounting must not confuse the two.
	if drawn+page.Accounting.Dropped != page.Accounting.Proven {
		t.Errorf("drawn %d plus dropped %d is not proven %d",
			drawn, page.Accounting.Dropped, page.Accounting.Proven)
	}
	t.Logf("crowded-text: %d drawn, %d recorded, %d drawn without their text",
		page.Accounting.Drawn, page.Accounting.Dropped, len(page.Unwritten))
}
