package render

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
)

// The page answers "what is this joined to" by itself, and these are the two
// things that has to rest on.
//
// The viewer walks from the box the pointer is over to the lines that name it,
// and from those lines to the boxes at their other ends. Nothing in the drawing
// rules cares whether that walk arrives anywhere: a line naming a box that is
// not on the page draws perfectly and leaves the highlight dark, in silence.

// TestEveryLineNamesBoxesOnItsOwnPage is the walk's first step. A line whose end
// names something that is not on this page is a line the highlight can never
// reach, and a reader hovering that box would be told it is joined to nothing.
func TestEveryLineNamesBoxesOnItsOwnPage(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.Family)+" from "+f.Fixture, func(t *testing.T) {
			page, err := Build(composeFor(t, f.Family, f.Fixture))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			scenes, err := artifact.Parse([]byte(page.HTML()))
			if err != nil {
				t.Fatalf("read back the artifact: %v", err)
			}
			for i := range scenes {
				scene := &scenes[i]
				here := map[string]bool{}
				for _, b := range scene.Boxes {
					here[b.ID] = true
				}
				for _, r := range scene.Routes {
					if !here[r.From] {
						t.Errorf("%s: %s leaves %q, which is not a box on this page", scene.Level, r.ID, r.From)
					}
					if !here[r.To] {
						t.Errorf("%s: %s arrives at %q, which is not a box on this page", scene.Level, r.ID, r.To)
					}
				}
				for _, bar := range scene.Bars {
					if !here[bar.Box] {
						t.Errorf("%s: execution %s runs on %q, which is not a box on this page",
							scene.Level, bar.ID, bar.Box)
					}
				}
			}
		})
	}
}

// TestEveryLabelNamesALineThatIsDrawn is the second step. A label pointing at a
// line that was left out would stay lit when everything around it faded, which
// says the reader is looking at something that is not there.
func TestEveryLabelNamesALineThatIsDrawn(t *testing.T) {
	for _, f := range families() {
		page, err := Build(composeFor(t, f.Family, f.Fixture))
		if err != nil {
			t.Fatalf("render %s from %s: %v", f.Family, f.Fixture, err)
		}
		html := page.HTML()
		drawn := map[string]bool{}
		for _, id := range attributeValues(html, attrEdgeID) {
			drawn[id] = true
		}
		for _, id := range attributeValues(html, attrEdgeLabelFor) {
			if !drawn[id] {
				t.Errorf("%s from %s: a label names %q, which is not a line on the page",
					f.Family, f.Fixture, id)
			}
		}
	}
}

// attributeValues pulls every value of one attribute out of the emitted page.
func attributeValues(html, attribute string) []string {
	var out []string
	needle := attribute + `="`
	for rest := html; ; {
		i := strings.Index(rest, needle)
		if i < 0 {
			return out
		}
		rest = rest[i+len(needle):]
		j := strings.Index(rest, `"`)
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j:]
	}
}
