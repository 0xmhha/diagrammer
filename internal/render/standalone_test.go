package render

import (
	"encoding/xml"
	"regexp"
	"strings"
	"testing"
)

// A standalone file is the page's drawing with the page taken away, and what
// can go wrong is exactly what the page was providing: the stylesheet, the
// frame, the interaction. These read the file back rather than trusting the
// writer, the way the page's own tests do.

func TestAStandaloneFileIsWellFormedAndStylesItself(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.Family)+"/"+f.Fixture, func(t *testing.T) {
			page, err := Build(composeFor(t, f.Family, f.Fixture))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, scene := range page.Scenes {
				svg, _ := Standalone(scene, Fit)

				// Well-formed, because a file is opened by things stricter
				// than a browser.
				if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
					t.Errorf("%s: not well-formed XML: %v", scene.Level, err)
				}
				// No reference to a stylesheet that is not there.
				if strings.Contains(svg, "var(") {
					t.Errorf("%s: the file still refers to a token by var()", scene.Level)
				}
				// Every class the drawing uses has a rule in the file. A class
				// added to svg.go and not styled here would ship unstyled and
				// nothing would say so.
				style := svg[strings.Index(svg, "<style>"):strings.Index(svg, "</style>")]
				for _, class := range classesIn(svg[strings.Index(svg, "</style>"):]) {
					if !strings.Contains(style, "."+class) {
						t.Errorf("%s: class %q is drawn and not styled", scene.Level, class)
					}
				}
				// And nothing the page alone can do is promised.
				for _, page := range []string{"data-opens]", ".focusing", "transition", ".scene-scroll"} {
					if strings.Contains(style, page) {
						t.Errorf("%s: the file's style speaks of %s, which a file cannot do", scene.Level, page)
					}
				}
			}
		})
	}
}

var classAttr = regexp.MustCompile(`class="([^"]+)"`)

func classesIn(svg string) []string {
	seen := map[string]bool{}
	for _, m := range classAttr.FindAllStringSubmatch(svg, -1) {
		for _, c := range strings.Fields(m[1]) {
			if c != "scene" {
				seen[c] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	return out
}

// TestFitIsTheDrawingsOwnSize holds the frame that promises nothing: no
// scaling, no letterbox, the drawing exactly as the page holds it.
func TestFitIsTheDrawingsOwnSize(t *testing.T) {
	page, err := Build(composeFor(t, "component", "hub"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	scene := page.Scenes[0]
	svg, framed := Standalone(scene, Fit)
	if framed.Scale != 1 || framed.W != scene.Width || framed.H != scene.Height {
		t.Errorf("fit framed the drawing as %+v; it is %v x %v", framed, scene.Width, scene.Height)
	}
	if !strings.Contains(svg, `viewBox="0 0 `+num(scene.Width)+" "+num(scene.Height)+`"`) {
		t.Error("the viewBox is not the drawing's own size")
	}
	if !strings.Contains(svg, `transform="translate(0 0) scale(1)"`) {
		t.Error("fit moved or scaled the drawing")
	}
}

// TestAFrameScalesToFitAndSaysWhatThatCost is the one thing about a framed file
// a reader cannot see. The page never scales a drawing; a file with one size
// must, and it reports how far and what the smallest label came to.
func TestAFrameScalesToFitAndSaysWhatThatCost(t *testing.T) {
	page, err := Build(composeFor(t, "component", "wide-page"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var widest *sceneRef
	for i := range page.Scenes {
		if widest == nil || page.Scenes[i].Width > page.Scenes[widest.i].Width {
			widest = &sceneRef{i}
		}
	}
	scene := page.Scenes[widest.i]
	slide, _ := SizeNamed("slide-16x9")
	svg, framed := Standalone(scene, slide)

	if framed.W != 1280 || framed.H != 720 {
		t.Errorf("the frame is %vx%v, want 1280x720", framed.W, framed.H)
	}
	if framed.Scale >= 1 {
		t.Fatalf("a %v-wide drawing was not scaled down into a 1280 frame: scale %v", scene.Width, framed.Scale)
	}
	if framed.SmallestLabel >= labelMinSize {
		t.Errorf("the smallest label is reported as %v, which is not under the %v floor a scale of %v puts it under",
			framed.SmallestLabel, labelMinSize, framed.Scale)
	}
	// Centred: the translation is half the slack on each axis.
	if !strings.Contains(svg, `viewBox="0 0 1280 720"`) {
		t.Error("the viewBox is not the frame")
	}
	if !strings.Contains(svg, "scale("+num(framed.Scale)+")") {
		t.Errorf("the drawing is not scaled by %v in the file", framed.Scale)
	}
}

type sceneRef struct{ i int }

func TestEveryPresetIsAMultipleOfFour(t *testing.T) {
	for _, name := range Sizes() {
		s, _ := SizeNamed(name)
		if int(s.W)%4 != 0 || int(s.H)%4 != 0 {
			t.Errorf("%s is %vx%v; every side is a multiple of four", name, s.W, s.H)
		}
	}
	if _, ok := SizeNamed("poster"); ok {
		t.Error("a frame nobody defined was found")
	}
}

// TestTheStandaloneStyleFollowsThePage holds the derivation: a value on the
// page is the value in the file. The box frame's stroke is the one to watch,
// because it is the rule most likely to be edited on the page and forgotten
// in the file, and the file has no way to be forgotten in since it is read
// out of the same text.
func TestTheStandaloneStyleFollowsThePage(t *testing.T) {
	style := sceneStyle()
	tokens := lightTokens(viewerCSS)
	if tokens["--ink"] == "" {
		t.Fatal("viewer.css no longer defines --ink in :root")
	}
	if !strings.Contains(style, ".box-frame { fill: "+tokens["--paper"]+"; stroke: "+tokens["--ink"]+";") {
		t.Errorf("the file's box frame does not carry the page's paper and ink:\n%s", style)
	}
}
