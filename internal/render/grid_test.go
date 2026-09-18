package render

import (
	"math"
	"testing"
)

// Every coordinate in a drawing sits on a four-pixel grid. It is the rule that
// separates a schematic from a wiring diagram, and it is the kind that erodes:
// a width computed from a label, a gap chosen for how it looked, and the grid is
// gone without any one change having broken it. So it is held two ways: on the
// constants, which is where it comes from, and on the drawings, which is where
// it is seen.

const gridStep = 4

func onGrid(v float64) bool { return math.Abs(v/gridStep-math.Round(v/gridStep)) < 1e-6 }

// TestTheConstantsAreOnTheGrid holds the source. A distance is a multiple of
// four. A size that something is centred in is a multiple of eight, because a
// line leaves a box from its centre, and half of a multiple of four is not
// necessarily on the grid.
func TestTheConstantsAreOnTheGrid(t *testing.T) {
	distances := map[string]float64{
		"laneGap": laneGap, "stub": stub, "channelX": channelX, "channelY": channelY,
		"margin": margin, "lifelineGap": lifelineGap, "headHeight": headHeight,
		"firstRung": firstRung, "rungHeight": rungHeight, "barWidth": barWidth,
	}
	for name, v := range distances {
		if math.Mod(v, 4) != 0 {
			t.Errorf("%s = %v is not a multiple of 4", name, v)
		}
	}
	centred := map[string]float64{
		"boxMinWidth": boxMinWidth, "boxMaxWidth": boxMaxWidth, "boxHeight": boxHeight,
		"markerSize": markerSize, "lifelineWidth": lifelineWidth,
	}
	for name, v := range centred {
		if math.Mod(v, 8) != 0 {
			t.Errorf("%s = %v is not a multiple of 8, so its centre is off the grid", name, v)
		}
	}
}

// TestEveryDrawingIsOnTheGrid holds the drawings. Every box, every band and
// every point a line passes through, in every fixture of every family. The
// sequence ladder is in here too, which is what found its own constants were
// off after the grid families were on.
func TestEveryDrawingIsOnTheGrid(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.Family)+"/"+f.Fixture, func(t *testing.T) {
			page, err := Build(composeFor(t, f.Family, f.Fixture))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, scene := range page.Scenes {
				if !onGrid(scene.Width) || !onGrid(scene.Height) {
					t.Errorf("%s: the drawing is %vx%v", scene.Level, scene.Width, scene.Height)
				}
				for _, b := range scene.Boxes {
					for what, v := range map[string]float64{"x": b.X, "y": b.Y, "w": b.W, "h": b.H} {
						if !onGrid(v) {
							t.Errorf("%s: box %s has %s = %v", scene.Level, b.ID, what, v)
						}
					}
				}
				for _, r := range scene.Regions {
					for what, v := range map[string]float64{"x": r.X, "y": r.Y, "w": r.W, "h": r.H} {
						if !onGrid(v) {
							t.Errorf("%s: band %s has %s = %v", scene.Level, r.ID, what, v)
						}
					}
				}
				for _, route := range scene.Routes {
					for i, p := range route.Points {
						if !onGrid(p.X) || !onGrid(p.Y) {
							t.Errorf("%s: line %s point %d is at (%v, %v)", scene.Level, route.ID, i, p.X, p.Y)
						}
					}
				}
			}
		})
	}
}
