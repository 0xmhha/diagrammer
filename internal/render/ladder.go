package render

import (
	"math"
	"sort"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/diagram"
)

// A sequence diagram is a ladder, not a grid.
//
// Lifelines stand in columns and every message has a rung of its own, in the
// order the model wrote them. Nothing is ever dropped here: a grid runs out of
// room to route a line, and that is what the component family's record is for,
// but a ladder has a rung for every message and no reason to refuse one.
const (
	lifelineWidth = 156
	headHeight    = 52
	lifelineGap   = 64
	// rungHeight is the vertical space one message gets. It has to hold the
	// arrow and the label above it without the two runs touching.
	rungHeight = 58
	// firstRung is how far below the heads the first message sits, so an
	// activation starting at the top has somewhere to begin.
	firstRung = 56
	// barWidth is the execution bar drawn on a lifeline.
	barWidth = 12
	// framePad is how far a fragment's frame stands outside what it holds.
	framePad = 20
)

// headLabelUnits is how much text a lifeline head can hold before it is cut.
func headLabelUnits() int {
	return int(math.Floor(float64(lifelineWidth-textPadding) / (labelSize * widthFactor)))
}

// buildLadder draws a sequence document.
func buildLadder(doc *diagram.Document) (*Page, error) {
	page := &Page{Title: doc.Meta.Title, Subtitle: doc.Meta.Subtitle, Family: doc.Family}
	for _, level := range doc.Levels {
		scene := ladderScene(level)
		page.Scenes = append(page.Scenes, scene)
		page.Accounting.Proven += level.Accounting.Drawn
		page.Accounting.Drawn += len(scene.Routes) + len(scene.Bars)
	}
	sort.Slice(page.Scenes, func(i, j int) bool { return page.Scenes[i].Level < page.Scenes[j].Level })
	return page, nil
}

func ladderScene(level diagram.Level) artifact.Scene {
	columnX := func(col int) float64 {
		return margin + float64(col)*(lifelineWidth+lifelineGap)
	}
	centreOf := func(col int) float64 { return columnX(col) + lifelineWidth/2 }
	rungY := func(order int) float64 {
		return margin + headHeight + firstRung + float64(order)*rungHeight
	}

	scene := artifact.Scene{
		Family: string(diagram.FamilySequence),
		Level:  level.ID,
		Title:  level.Title,
	}

	centre := map[string]float64{}
	for _, b := range level.Boxes {
		x := columnX(b.Col)
		centre[b.ID] = centreOf(b.Col)
		scene.Boxes = append(scene.Boxes, artifact.Box{
			ID: b.ID, Label: truncate(b.Label, headLabelUnits()),
			Stereotype: b.Stereotype, Opens: b.Opens,
			X: quantize(x), Y: margin, W: lifelineWidth, H: headHeight,
		})
	}
	sort.Slice(scene.Boxes, func(i, j int) bool { return scene.Boxes[i].ID < scene.Boxes[j].ID })

	rows := 0
	for _, c := range level.Connections {
		order := 0
		if c.Order != nil {
			order = *c.Order
		}
		if order+1 > rows {
			rows = order + 1
		}
		y := rungY(order)
		from, to := centre[c.From], centre[c.To]
		// A message to the lifeline it came from would be a zero-length arrow.
		// It is drawn as a short loop out and back so it reads as a call the
		// participant makes on itself.
		if from == to {
			scene.Routes = append(scene.Routes, artifact.Route{
				ID: c.ID, From: c.From, To: c.To, FromSide: "right", ToSide: "right",
				Points: []artifact.Point{
					{X: quantize(from), Y: quantize(y)},
					{X: quantize(from + 40), Y: quantize(y)},
					{X: quantize(from + 40), Y: quantize(y + 18)},
					{X: quantize(from), Y: quantize(y + 18)},
				},
				LabelText: c.Label,
				LabelAt:   artifact.Point{X: quantize(from + 40), Y: quantize(y + 9)},
			})
			continue
		}
		side, otherSide := "right", "left"
		if to < from {
			side, otherSide = "left", "right"
		}
		scene.Routes = append(scene.Routes, artifact.Route{
			ID: c.ID, From: c.From, To: c.To, FromSide: side, ToSide: otherSide,
			Points: []artifact.Point{
				{X: quantize(from), Y: quantize(y)}, {X: quantize(to), Y: quantize(y)},
			},
			LabelText: c.Label,
			LabelAt:   artifact.Point{X: quantize((from + to) / 2), Y: quantize(y)},
		})
	}
	sort.Slice(scene.Routes, func(i, j int) bool { return scene.Routes[i].ID < scene.Routes[j].ID })

	for _, a := range level.Activations {
		x := centre[a.Box] - barWidth/2
		top, bottom := rungY(a.FromRow), rungY(a.ToRow)
		if bottom < top {
			top, bottom = bottom, top
		}
		scene.Bars = append(scene.Bars, artifact.Bar{
			ID: a.ID, Box: a.Box,
			X: quantize(x), Y: quantize(top), W: barWidth, H: quantize(bottom - top + rungHeight/3),
		})
	}
	sort.Slice(scene.Bars, func(i, j int) bool { return scene.Bars[i].ID < scene.Bars[j].ID })

	for _, f := range level.Fragments {
		left, right := centreOf(f.FromCol), centreOf(f.ToCol)
		top, bottom := rungY(f.FromRow), rungY(f.ToRow)
		scene.Frames = append(scene.Frames, artifact.Frame{
			ID: f.ID, Kind: string(f.Kind), Label: operandGuard(f),
			X: quantize(left - framePad), Y: quantize(top - framePad - 14),
			W: quantize(right - left + framePad*2), H: quantize(bottom - top + framePad*2 + 14),
		})
	}
	sort.Slice(scene.Frames, func(i, j int) bool { return scene.Frames[i].ID < scene.Frames[j].ID })

	scene.Width = quantize(columnX(maxCol(level)) + lifelineWidth + margin)
	scene.Height = quantize(rungY(max(rows, 1)) + margin)
	return scene
}

// operandGuard is the condition a fragment's first branch carries, which is the
// part a reader needs beside the frame's name.
func operandGuard(f diagram.Fragment) string {
	if len(f.Operands) == 0 {
		return ""
	}
	return f.Operands[0].Guard
}

func maxCol(level diagram.Level) int {
	high := 0
	for _, b := range level.Boxes {
		if b.Col > high {
			high = b.Col
		}
	}
	return high
}
