package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A drawing is laid out in its own units and then shown at whatever size the
// page gives it. Those two were never connected: the stylesheet scaled every
// drawing to the width of the page, so a wide one was shrunk, and every label
// in it went under the size the fitting had just refused to go below.
//
// Nothing caught that. The composition rules judge the drawing in its own
// units, where the labels are the size they were fitted to, and a page nobody
// can read passes all of them. These hold the two together instead.

var sceneSize = regexp.MustCompile(
	`<svg class="scene"[^>]*viewBox="0 0 ([0-9.]+) ([0-9.]+)"[^>]*style="min-width:([0-9.]+)px;min-height:([0-9.]+)px"`)

func TestEveryDrawingDeclaresTheSizeItWasLaidOutAt(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.Family)+"/"+f.Fixture, func(t *testing.T) {
			page, err := Build(composeFor(t, f.Family, f.Fixture))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			html := page.HTML()

			found := sceneSize.FindAllStringSubmatch(html, -1)
			if len(found) != len(page.Scenes) {
				t.Fatalf("%d of %d drawings declare a size", len(found), len(page.Scenes))
			}
			for _, m := range found {
				for i, what := range []string{"width", "height"} {
					laidOut, declared := m[1+i], m[3+i]
					if laidOut != declared {
						t.Errorf("a drawing is laid out %s %s and declares %s", what, laidOut, declared)
					}
				}
			}
		})
	}
}

// TestTheWidePageIsWiderThanAPageAndSaysSo is the fixture that breaks the rule.
//
// Every other fixture draws something that fits, so every other fixture would
// pass whether the size were declared or not. This one is the case the unfold
// window cannot help with: a package with no generation between it and its
// members, so each member is a column and the drawing runs off the page.
func TestTheWidePageIsWiderThanAPageAndSaysSo(t *testing.T) {
	page, err := Build(composeFor(t, "component", "wide-page"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Wider than any plausible page, which is the property being held: if this
	// fixture ever starts fitting, it has stopped covering what it is for.
	const tooWide = 4000
	widest := 0.0
	for _, scene := range page.Scenes {
		widest = max(widest, scene.Width)
	}
	if widest < tooWide {
		t.Fatalf("the widest drawing is %.0f, which fits; this fixture no longer covers a page that does not", widest)
	}

	html := page.HTML()
	if !strings.Contains(html, `class="scene-scroll"`) {
		t.Error("the drawing has nothing to scroll inside, so a page too wide to fit is shrunk instead")
	}
	for _, m := range sceneSize.FindAllStringSubmatch(html, -1) {
		declared, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			t.Fatalf("parse the declared width %q: %v", m[3], err)
		}
		if declared == widest {
			return
		}
	}
	t.Errorf("the widest drawing is %.0f and no drawing declares that width", widest)
}
