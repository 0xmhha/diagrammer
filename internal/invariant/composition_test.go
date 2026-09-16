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
				})
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
