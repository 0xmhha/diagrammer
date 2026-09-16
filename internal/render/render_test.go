package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

var fixtures = []string{"order-service", "nested-platform"}

// families are the four the composer emits, each drawn from the fixture that
// declares it. order-service declares all four; nested-platform is the dense
// component one.
func families() []struct {
	Family  diagram.Family
	Fixture string
} {
	return []struct {
		Family  diagram.Family
		Fixture string
	}{
		{diagram.FamilyComponent, "nested-platform"},
		{diagram.FamilyComponent, "order-service"},
		{diagram.FamilySequence, "order-service"},
		{diagram.FamilyState, "order-service"},
		{diagram.FamilyUsecase, "order-service"},
	}
}

func composeFor(t *testing.T, family diagram.Family, fixture string) *diagram.Document {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "codegraph", fixture+".codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("%s does not validate: %v", fixture, err)
	}
	build := map[diagram.Family]func(string, *uml.Model) (*diagram.Document, error){
		diagram.FamilyComponent: compose.Component,
		diagram.FamilySequence:  compose.Sequence,
		diagram.FamilyState:     compose.State,
		diagram.FamilyUsecase:   compose.Usecase,
	}[family]
	doc, err := build(path, model)
	if err != nil {
		t.Fatalf("compose %s: %v", family, err)
	}
	return doc
}

// TestEveryFamilyDraws is what closes the pipeline: a family the composer emits
// and the renderer cannot draw is a gap inside our own work.
func TestEveryFamilyDraws(t *testing.T) {
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
			if len(scenes) == 0 {
				t.Fatal("the artifact holds no drawing")
			}
			for _, scene := range scenes {
				if scene.Family != string(f.Family) {
					t.Errorf("%s: the drawing says it is a %q diagram", scene.Level, scene.Family)
				}
				for _, p := range invariant.CompositionFor(&scene) {
					t.Errorf("%s: %s", scene.Level, p)
				}
			}
			if page.Accounting.Drawn+page.Accounting.Dropped != page.Accounting.Proven {
				t.Errorf("drawn %d plus dropped %d is not proven %d",
					page.Accounting.Drawn, page.Accounting.Dropped, page.Accounting.Proven)
			}
		})
	}
}

// TestSequenceKeepsItsRungsInOrder is the ladder's own concern carried into the
// drawing: the same messages on different rungs describe a different
// interaction.
func TestSequenceKeepsItsRungsInOrder(t *testing.T) {
	doc := composeFor(t, diagram.FamilySequence, "order-service")
	page, err := Build(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	scenes, err := artifact.Parse([]byte(page.HTML()))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}

	y := map[string]float64{}
	for _, scene := range scenes {
		for _, r := range scene.Routes {
			if len(r.Points) > 0 {
				y[r.ID] = r.Points[0].Y
			}
		}
	}
	previous := -1.0
	for _, c := range doc.Levels[0].Connections {
		got, ok := y[c.ID]
		if !ok {
			t.Errorf("message %q was not drawn", c.ID)
			continue
		}
		if got <= previous {
			t.Errorf("message %q sits at %.0f, not below the one before it at %.0f", c.ID, got, previous)
		}
		previous = got
	}
}

func composeFixture(t *testing.T, name string) *diagram.Document {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "codegraph", name+".codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("%s does not validate: %v", name, err)
	}
	doc, err := compose.Component(path, model)
	if err != nil {
		t.Fatalf("compose %s: %v", name, err)
	}
	return doc
}

func buildFixture(t *testing.T, name string) *Page {
	t.Helper()
	page, err := Build(composeFixture(t, name))
	if err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return page
}

// TestTheDrawingObeysItsOwnRules reads the rules back off the emitted document,
// not off the renderer's working state. A renderer checking itself proves only
// that it agrees with itself.
func TestTheDrawingObeysItsOwnRules(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			scenes, err := artifact.Parse([]byte(buildFixture(t, name).HTML()))
			if err != nil {
				t.Fatalf("read back the artifact: %v", err)
			}
			if len(scenes) == 0 {
				t.Fatal("the artifact holds no drawing")
			}
			for _, scene := range scenes {
				for _, p := range invariant.CompositionFor(&scene) {
					t.Errorf("%s: %s", scene.Level, p)
				}
			}
		})
	}
}

// TestAccountingCarriesAcrossTheBoundary is the invariant following the work
// from one stage into the next: what stage 3 drew is what stage 4 had to show,
// and whatever it could not show is recorded.
func TestAccountingCarriesAcrossTheBoundary(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			doc := composeFixture(t, name)
			page, err := Build(doc)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if page.Accounting.Proven != doc.Accounting.Drawn {
				t.Errorf("stage 4 was responsible for %d, stage 3 drew %d",
					page.Accounting.Proven, doc.Accounting.Drawn)
			}
			if page.Accounting.Drawn+page.Accounting.Dropped != page.Accounting.Proven {
				t.Errorf("drawn %d plus dropped %d is not proven %d",
					page.Accounting.Drawn, page.Accounting.Dropped, page.Accounting.Proven)
			}
			if len(page.Dropped) != page.Accounting.Dropped {
				t.Errorf("accounting says %d dropped, %d are recorded",
					page.Accounting.Dropped, len(page.Dropped))
			}
			// Every drop names the box it left, so a reader has somewhere to
			// look rather than a count to wonder about.
			for _, d := range page.Dropped {
				if d.Box == "" || d.Why == "" || d.Rule == "" {
					t.Errorf("a drop is recorded without saying where or why: %+v", d)
				}
			}
		})
	}
}

// TestDrawingIsStructurallyLossless is the check byte equality is reaching for
// and cannot make: every box the document declared is in the artifact, every
// relationship is either drawn or recorded, and the artifact invents nothing.
func TestDrawingIsStructurallyLossless(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			doc := composeFixture(t, name)
			page, err := Build(doc)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			scenes, err := artifact.Parse([]byte(page.HTML()))
			if err != nil {
				t.Fatalf("read back the artifact: %v", err)
			}

			drawn := map[string]bool{}
			boxes := map[string]bool{}
			for _, scene := range scenes {
				for _, b := range scene.Boxes {
					boxes[scene.Level+"/"+b.ID] = true
				}
				for _, r := range scene.Routes {
					drawn[scene.Level+"/"+r.ID] = true
				}
			}
			recorded := map[string]bool{}
			for _, d := range page.Dropped {
				recorded[d.Level+"/"+d.Route] = true
			}

			for _, level := range doc.Levels {
				for _, b := range level.Boxes {
					if !boxes[level.ID+"/"+b.ID] {
						t.Errorf("%s: box %q is in the document and not in the drawing", level.ID, b.ID)
					}
				}
				for _, c := range level.Connections {
					key := level.ID + "/" + c.ID
					if !drawn[key] && !recorded[key] {
						t.Errorf("%s: %q is neither drawn nor recorded", level.ID, c.ID)
					}
					if drawn[key] && recorded[key] {
						t.Errorf("%s: %q is both drawn and recorded", level.ID, c.ID)
					}
				}
			}

			declared := map[string]bool{}
			for _, level := range doc.Levels {
				for _, b := range level.Boxes {
					declared[level.ID+"/"+b.ID] = true
				}
			}
			for key := range boxes {
				if !declared[key] {
					t.Errorf("the drawing holds %q, which the document does not declare", key)
				}
			}
		})
	}
}

func TestDrawingIsByteIdenticalAcrossRuns(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			want := buildFixture(t, name).HTML()
			for i := range 4 {
				if buildFixture(t, name).HTML() != want {
					t.Fatalf("run %d differs from the first", i+2)
				}
			}
		})
	}
}

// TestEveryLevelIsReachable keeps the drill-down working in the artifact. A
// page nobody can open is a page nobody will read, however correct it is.
func TestEveryLevelIsReachable(t *testing.T) {
	page := buildFixture(t, "nested-platform")
	html := page.HTML()
	scenes, err := artifact.Parse([]byte(html))
	if err != nil {
		t.Fatalf("read back the artifact: %v", err)
	}

	opens := map[string]bool{}
	for _, scene := range scenes {
		for _, b := range scene.Boxes {
			if b.Opens != "" {
				opens[b.Opens] = true
			}
		}
	}
	found := 0
	for _, scene := range scenes {
		if opens[scene.Level] {
			found++
		}
	}
	if found == 0 {
		t.Fatal("no level is opened from a box, so the drill-down never works")
	}
	// Every level also has a button of its own, so a reader is never stranded
	// on a page whose way back was a box they have to remember.
	for _, scene := range scenes {
		if !contains(html, `data-level-link="`+scene.Level+`"`) {
			t.Errorf("%s has no button in the navigation", scene.Level)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
