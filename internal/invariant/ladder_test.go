package invariant_test

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// soundLadder is two lifelines with one message and one execution bar.
func soundLadder() *artifact.Scene {
	return &artifact.Scene{
		Family: "sequence", Level: "overview", Width: 600, Height: 400,
		Boxes: []artifact.Box{
			{ID: "a", X: 40, Y: 40, W: 160, H: 52},
			{ID: "b", X: 300, Y: 40, W: 160, H: 52},
		},
		Routes: []artifact.Route{{
			ID: "m1", From: "a", To: "b", FromSide: "right", ToSide: "left",
			Points: []artifact.Point{{X: 120, Y: 160}, {X: 380, Y: 160}},
		}},
		Bars:   []artifact.Bar{{ID: "act1", Box: "b", X: 374, Y: 160, W: 12, H: 60}},
		Frames: []artifact.Frame{{ID: "f1", Kind: "opt", X: 100, Y: 130, W: 320, H: 120}},
	}
}

func TestASoundLadderPasses(t *testing.T) {
	if problems := invariant.CompositionFor(soundLadder()); len(problems) > 0 {
		t.Fatalf("a sound ladder reported %d problems: %v", len(problems), problems)
	}
}

func TestEverySequenceRuleFires(t *testing.T) {
	cases := []struct {
		name   string
		rule   string
		damage func(*artifact.Scene)
	}{
		{
			name: "two messages share a rung",
			rule: invariant.RuleRungDistinct,
			damage: func(s *artifact.Scene) {
				s.Routes = append(s.Routes, artifact.Route{
					ID: "m2", From: "b", To: "a", FromSide: "left", ToSide: "right",
					Points: []artifact.Point{{X: 380, Y: 160}, {X: 120, Y: 160}},
				})
			},
		}, {
			name: "an arrow ends away from the lifeline it names",
			rule: invariant.RuleMessageSpan,
			damage: func(s *artifact.Scene) {
				s.Routes[0].Points[1].X = 500
			},
		}, {
			name: "an arrow names a lifeline that is not here",
			rule: invariant.RuleMessageSpan,
			damage: func(s *artifact.Scene) {
				s.Routes[0].To = "ghost"
			},
		}, {
			name: "an execution bar sits off its lifeline",
			rule: invariant.RuleActivationLine,
			damage: func(s *artifact.Scene) {
				s.Bars[0].X = 200
			},
		}, {
			name: "an execution bar names a lifeline that is not here",
			rule: invariant.RuleActivationLine,
			damage: func(s *artifact.Scene) {
				s.Bars[0].Box = "ghost"
			},
		}, {
			name: "a message runs off the page",
			rule: invariant.RuleInsideCanvas,
			damage: func(s *artifact.Scene) {
				s.Width = 200
			},
		}, {
			name: "a fragment frames nothing",
			rule: invariant.RuleInsideCanvas,
			damage: func(s *artifact.Scene) {
				s.Frames[0].H = 0
			},
		},
	}

	covered := map[string]bool{}
	for _, c := range cases {
		covered[c.rule] = true
		t.Run(c.name, func(t *testing.T) {
			scene := soundLadder()
			c.damage(scene)

			problems := invariant.CompositionFor(scene)
			if len(problems) == 0 {
				t.Fatalf("the damaged ladder passed, so %q never fires", c.rule)
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

	for _, rule := range invariant.SequenceRules() {
		if !covered[rule] {
			t.Errorf("sequence rule %q has no case that fires it", rule)
		}
	}
}

// TestAFamilyWithoutRulesIsRefused keeps a drawing from being judged by another
// family's rules just because it arrived.
func TestAFamilyWithoutRulesIsRefused(t *testing.T) {
	scene := soundLadder()
	scene.Family = "something-else"
	problems := invariant.CompositionFor(scene)
	if len(problems) == 0 {
		t.Fatal("a drawing of an unknown family passed")
	}
	if problems[0].Rule != "unknown-family" {
		t.Errorf("want unknown-family, got %q", problems[0].Rule)
	}
}
