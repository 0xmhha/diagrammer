package invariant_test

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// Every composition rule gets a drawing that breaks it.
//
// These matter more than most: a rule that never fires lets a bad drawing
// through, and a drawing is the one artefact nobody diffs. The scenes are built
// by hand rather than composed, because each one has to be wrong in exactly one
// respect and a real fixture is wrong in none.

// soundScene is two boxes side by side with one straight line between them.
func soundScene() *artifact.Scene {
	return &artifact.Scene{
		Level:  "overview",
		Width:  600,
		Height: 300,
		Boxes: []artifact.Box{
			{ID: "a", X: 60, Y: 100, W: 160, H: 68},
			{ID: "b", X: 400, Y: 100, W: 160, H: 68},
		},
		Routes: []artifact.Route{{
			ID: "r1", From: "a", To: "b", FromSide: "right", ToSide: "left",
			Points: []artifact.Point{{X: 220, Y: 134}, {X: 400, Y: 134}},
		}},
	}
}

func TestASoundDrawingPasses(t *testing.T) {
	if problems := invariant.Composition(soundScene()); len(problems) > 0 {
		t.Fatalf("a sound drawing reported %d problems: %v", len(problems), problems)
	}
}

func TestEveryCompositionRuleFires(t *testing.T) {
	cases := []struct {
		name   string
		rule   string
		damage func(*artifact.Scene)
	}{
		{
			name: "a route leaves the opposite way from the edge it names",
			rule: invariant.RuleEndpointSide,
			damage: func(s *artifact.Scene) {
				s.Routes[0].FromSide = "left"
			},
		}, {
			name: "a route starts away from the box it names",
			rule: invariant.RuleEndpointSide,
			damage: func(s *artifact.Scene) {
				s.Routes[0].Points[0] = artifact.Point{X: 300, Y: 220}
			},
		}, {
			name: "a route passes through a box it is not attached to",
			rule: invariant.RulePassThrough,
			damage: func(s *artifact.Scene) {
				s.Boxes = append(s.Boxes, artifact.Box{ID: "c", X: 280, Y: 110, W: 60, H: 48})
			},
		}, {
			name: "a route runs close enough to a box to read as joined to it",
			rule: invariant.RuleSeparation,
			damage: func(s *artifact.Scene) {
				// Beside the line rather than across it: near enough to look
				// attached, not near enough to pass through.
				s.Boxes = append(s.Boxes, artifact.Box{ID: "c", X: 280, Y: 90, W: 60, H: 40})
			},
		}, {
			name: "a route has a run too short to read as a turn",
			rule: invariant.RuleMinSegment,
			damage: func(s *artifact.Scene) {
				s.Routes[0].Points = []artifact.Point{
					{X: 220, Y: 134}, {X: 224, Y: 134}, {X: 224, Y: 140}, {X: 400, Y: 140},
				}
			},
		}, {
			name: "two unrelated routes cross",
			rule: invariant.RuleCrossing,
			damage: func(s *artifact.Scene) {
				s.Boxes = append(s.Boxes,
					artifact.Box{ID: "c", X: 280, Y: 20, W: 80, H: 40},
					artifact.Box{ID: "d", X: 280, Y: 220, W: 80, H: 40})
				s.Routes = append(s.Routes, artifact.Route{
					ID: "r2", From: "c", To: "d", FromSide: "bottom", ToSide: "top",
					Points: []artifact.Point{{X: 320, Y: 60}, {X: 320, Y: 220}},
				})
			},
		}, {
			name: "a label sits on somebody else's route",
			rule: invariant.RuleLabelClear,
			damage: func(s *artifact.Scene) {
				s.Boxes = append(s.Boxes,
					artifact.Box{ID: "c", X: 60, Y: 200, W: 80, H: 40},
					artifact.Box{ID: "d", X: 400, Y: 200, W: 80, H: 40})
				s.Routes = append(s.Routes, artifact.Route{
					ID: "r2", From: "c", To: "d", FromSide: "right", ToSide: "left",
					Points:    []artifact.Point{{X: 140, Y: 220}, {X: 400, Y: 220}},
					LabelText: "misplaced",
					LabelAt:   artifact.Point{X: 300, Y: 134},
					// A label is a rectangle of text, and the rule measures the
					// rectangle. Giving only a point would describe a label the
					// page does not draw.
					LabelBounds: artifact.Rect{X: 274, Y: 115, W: 52, H: 13},
				})
			},
		}, {
			name: "two labels are written in the same place",
			rule: invariant.RuleLabelClear,
			damage: func(s *artifact.Scene) {
				// Far from every line, and on top of each other. A label
				// measured as its centre point would clear both tests; the
				// rule reads the rectangle, so it does not.
				s.Boxes = append(s.Boxes,
					artifact.Box{ID: "c", X: 60, Y: 320, W: 80, H: 40},
					artifact.Box{ID: "d", X: 400, Y: 320, W: 80, H: 40})
				s.Routes = append(s.Routes, artifact.Route{
					ID: "r2", From: "c", To: "d", FromSide: "right", ToSide: "left",
					Points:      []artifact.Point{{X: 140, Y: 340}, {X: 400, Y: 340}},
					LabelText:   "one",
					LabelAt:     artifact.Point{X: 260, Y: 340},
					LabelBounds: artifact.Rect{X: 240, Y: 320, W: 40, H: 13},
				})
				s.Routes[0].LabelText = "two"
				s.Routes[0].LabelAt = artifact.Point{X: 300, Y: 340}
				s.Routes[0].LabelBounds = artifact.Rect{X: 260, Y: 322, W: 40, H: 13}
			},
		}, {
			name: "a label is written over a box",
			rule: invariant.RuleLabelClear,
			damage: func(s *artifact.Scene) {
				s.Boxes = append(s.Boxes, artifact.Box{ID: "c", X: 280, Y: 320, W: 80, H: 40})
				s.Routes[0].LabelText = "over the box"
				s.Routes[0].LabelAt = artifact.Point{X: 300, Y: 348}
				s.Routes[0].LabelBounds = artifact.Rect{X: 290, Y: 330, W: 60, H: 13}
			},
		}, {
			name: "a route traces a band's border instead of crossing it",
			rule: invariant.RuleBorderRun,
			damage: func(s *artifact.Scene) {
				s.Regions = []artifact.Region{{ID: "band", X: 40, Y: 134, W: 500, H: 120}}
			},
		},
	}

	covered := map[string]bool{}
	for _, c := range cases {
		covered[c.rule] = true
		t.Run(c.name, func(t *testing.T) {
			scene := soundScene()
			c.damage(scene)

			problems := invariant.Composition(scene)
			if len(problems) == 0 {
				t.Fatalf("the damaged drawing passed, so %q never fires", c.rule)
			}
			for _, p := range problems {
				if p.Rule == c.rule {
					return
				}
			}
			var fired []string
			for _, p := range problems {
				fired = append(fired, p.Rule)
			}
			t.Errorf("want rule %q to fire, got %s", c.rule, strings.Join(fired, ", "))
		})
	}

	for _, rule := range invariant.CompositionRules() {
		if !covered[rule] {
			t.Errorf("composition rule %q has no case that fires it", rule)
		}
	}
}

// TestEveryMisplacedLabelIsReported is about the checker being complete rather
// than merely correct.
//
// It used to return from the whole function at the first misplaced label, so a
// page with two reported one. The caller drops what it is told about, draws the
// rest, and the check over the emitted artifact then finds the one that was
// never mentioned — which is how this was found: rendering a real model failed
// with "the renderer thought it had satisfied" these rules.
//
// A caller acting on a checker's answer is acting on all of it or on a fragment
// of it, and cannot tell which.
func TestEveryMisplacedLabelIsReported(t *testing.T) {
	scene := soundScene()
	// A second pair, with a route whose label lands on the first route, and a
	// third whose label lands on the second. Two independent violations.
	scene.Boxes = append(scene.Boxes,
		artifact.Box{ID: "c", X: 60, Y: 200, W: 80, H: 40},
		artifact.Box{ID: "d", X: 400, Y: 200, W: 80, H: 40},
		artifact.Box{ID: "e", X: 60, Y: 20, W: 80, H: 40},
		artifact.Box{ID: "f", X: 400, Y: 20, W: 80, H: 40})
	scene.Routes = append(scene.Routes,
		artifact.Route{
			ID: "r2", From: "c", To: "d", FromSide: "right", ToSide: "left",
			Points:    []artifact.Point{{X: 140, Y: 220}, {X: 400, Y: 220}},
			LabelText: "one", LabelAt: artifact.Point{X: 300, Y: 134},
			LabelBounds: artifact.Rect{X: 291, Y: 115, W: 18, H: 13},
		},
		artifact.Route{
			ID: "r3", From: "e", To: "f", FromSide: "right", ToSide: "left",
			Points:    []artifact.Point{{X: 140, Y: 40}, {X: 400, Y: 40}},
			LabelText: "two", LabelAt: artifact.Point{X: 300, Y: 220},
			LabelBounds: artifact.Rect{X: 291, Y: 201, W: 18, H: 13},
		})

	reported := map[string]bool{}
	for _, p := range invariant.Composition(scene) {
		if p.Rule == invariant.RuleLabelClear {
			reported[p.Route] = true
		}
	}
	for _, want := range []string{"r2", "r3"} {
		if !reported[want] {
			t.Errorf("%s's label is misplaced and was not reported", want)
		}
	}
}
