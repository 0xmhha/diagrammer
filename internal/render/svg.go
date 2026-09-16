package render

import (
	"html"
	"strings"

	"github.com/0xmhha/diagrammer/internal/artifact"
)

// svgFor draws one page.
//
// Order is fixed: bands first so they sit behind, then routes, then boxes on
// top, then labels. Anything else and a line would be drawn over the box it
// arrives at.
func svgFor(scene artifact.Scene) string {
	var b strings.Builder
	b.WriteString(`<svg class="scene" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	b.WriteString(num(scene.Width) + " " + num(scene.Height))
	b.WriteString(`" ` + attrFamily + `="` + esc(scene.Family) + `"`)
	b.WriteString(` ` + attrLevel + `="` + esc(scene.Level) + `" ` + attrLevelTitle + `="` + esc(scene.Title) + `">` + "\n")
	b.WriteString(arrowDefs())

	for _, r := range scene.Regions {
		b.WriteString(`  <g class="region" ` + attrRegionID + `="` + esc(r.ID) + `">` + "\n")
		b.WriteString(`    <rect class="region-frame" x="` + num(r.X) + `" y="` + num(r.Y) +
			`" width="` + num(r.W) + `" height="` + num(r.H) + `" rx="10"/>` + "\n")
		b.WriteString(`    <text class="region-label" x="` + num(r.X+12) + `" y="` + num(r.Y+18) + `">` +
			esc(r.Label) + `</text>` + "\n")
		b.WriteString("  </g>\n")
	}

	for _, r := range scene.Routes {
		b.WriteString(`  <path class="edge" ` + attrEdgeID + `="` + esc(r.ID) + `"`)
		b.WriteString(` ` + attrEdgeFrom + `="` + esc(r.From) + `"`)
		b.WriteString(` ` + attrEdgeTo + `="` + esc(r.To) + `"`)
		b.WriteString(` ` + attrEdgeFromSide + `="` + esc(r.FromSide) + `"`)
		b.WriteString(` ` + attrEdgeToSide + `="` + esc(r.ToSide) + `"`)
		b.WriteString(` ` + attrCompositionPoints + `="` + pointsValue(r.Points) + `"`)
		b.WriteString(` d="` + pathData(r.Points) + `" marker-end="url(#arrow)"/>` + "\n")
	}

	// A lifeline's stroke is furniture rather than a relationship, so it is
	// drawn and never entered into the scene. Messages cross lifelines all the
	// time, and a rule that counted those as crossings would refuse every
	// sequence diagram ever drawn.
	if scene.Family == "sequence" {
		for _, box := range scene.Boxes {
			b.WriteString(`  <line class="lifeline" x1="` + num(box.X+box.W/2) + `" y1="` + num(box.Bottom()) +
				`" x2="` + num(box.X+box.W/2) + `" y2="` + num(scene.Height-margin/2) + `"/>` + "\n")
		}
		for _, bar := range scene.Bars {
			b.WriteString(`  <rect class="activation" ` + attrBarID + `="` + esc(bar.ID) + `" ` +
				attrBarBox + `="` + esc(bar.Box) + `" x="` + num(bar.X) + `" y="` + num(bar.Y) +
				`" width="` + num(bar.W) + `" height="` + num(bar.H) + `"/>` + "\n")
		}
	}

	for _, box := range scene.Boxes {
		b.WriteString(`  <g class="box" ` + attrBoxID + `="` + esc(box.ID) + `"`)
		b.WriteString(` ` + attrBoxBounds + `="` + boundsValue(box.X, box.Y, box.W, box.H) + `"`)
		if box.Opens != "" {
			b.WriteString(` ` + attrBoxOpens + `="` + esc(box.Opens) + `"`)
		}
		b.WriteString(">\n")
		b.WriteString(shapeFor(scene.Family, box))
		b.WriteString(`    <text class="box-label" x="` + num(box.X+box.W/2) + `" y="` +
			num(labelY(scene.Family, box)) + `">` + esc(box.Label) + `</text>` + "\n")
		b.WriteString("  </g>\n")
	}

	for _, f := range scene.Frames {
		b.WriteString(`  <g class="fragment" ` + attrFrameID + `="` + esc(f.ID) + `" ` +
			attrFrameKind + `="` + esc(f.Kind) + `">` + "\n")
		b.WriteString(`    <rect class="fragment-frame" x="` + num(f.X) + `" y="` + num(f.Y) +
			`" width="` + num(f.W) + `" height="` + num(f.H) + `"/>` + "\n")
		label := f.Kind
		if f.Label != "" {
			label += " [" + f.Label + "]"
		}
		b.WriteString(`    <text class="fragment-label" x="` + num(f.X+8) + `" y="` + num(f.Y+16) + `">` +
			esc(label) + `</text>` + "\n")
		b.WriteString("  </g>\n")
	}

	for _, r := range scene.Routes {
		if r.LabelText == "" {
			continue
		}
		// x and y are the label's own position, and the lift off the line is a
		// dy. A reader of the artifact needs to know where the label is, and
		// baking the visual offset into y would mean it read back six pixels
		// from where the renderer decided to put it — enough to flip a
		// clearance verdict that was measured against ten.
		b.WriteString(`  <text class="edge-label" ` + attrEdgeLabelFor + `="` + esc(r.ID) +
			`" x="` + num(r.LabelAt.X) + `" y="` + num(r.LabelAt.Y) + `" dy="-6">` +
			esc(r.LabelText) + `</text>` + "\n")
	}

	b.WriteString("</svg>")
	return b.String()
}

// shapeFor draws the outline a reader recognises.
//
// UML gives each thing a shape and the shape is half the meaning: a ringed
// circle is an end, an ellipse is something the system does for somebody, a
// figure is the somebody. Drawing them all as rectangles would be legible and
// would say the wrong thing.
func shapeFor(family string, box artifact.Box) string {
	cx, cy := box.X+box.W/2, box.Y+box.H/2
	switch {
	case family == "state" && box.Stereotype == "initial":
		return `    <circle class="marker-filled" cx="` + num(cx) + `" cy="` + num(cy) +
			`" r="` + num(box.W/2) + `"/>` + "\n"
	case family == "state" && box.Stereotype == "final":
		return `    <circle class="marker-ring" cx="` + num(cx) + `" cy="` + num(cy) +
			`" r="` + num(box.W/2) + `"/>` + "\n" +
			`    <circle class="marker-filled" cx="` + num(cx) + `" cy="` + num(cy) +
			`" r="` + num(box.W/2-5) + `"/>` + "\n"
	case family == "state" && (box.Stereotype == "choice" || box.Stereotype == "junction"):
		return `    <polygon class="box-frame" points="` +
			num(cx) + "," + num(box.Y) + " " + num(box.Right()) + "," + num(cy) + " " +
			num(cx) + "," + num(box.Bottom()) + " " + num(box.X) + "," + num(cy) + `"/>` + "\n"
	case family == "usecase" && box.Stereotype == "":
		return `    <ellipse class="box-frame" cx="` + num(cx) + `" cy="` + num(cy) +
			`" rx="` + num(box.W/2) + `" ry="` + num(box.H/2) + `"/>` + "\n"
	case family == "usecase":
		return actorFigure(box)
	default:
		return `    <rect class="box-frame" x="` + num(box.X) + `" y="` + num(box.Y) +
			`" width="` + num(box.W) + `" height="` + num(box.H) + `" rx="8"/>` + "\n"
	}
}

// actorFigure draws the stick figure UML uses for a role outside the system.
func actorFigure(box artifact.Box) string {
	cx := box.X + box.W/2
	top := box.Y + 6
	head := 7.0
	body := top + head*2
	return `    <circle class="figure" cx="` + num(cx) + `" cy="` + num(top+head) + `" r="` + num(head) + `"/>` + "\n" +
		`    <path class="figure" d="M ` + num(cx) + ` ` + num(body) +
		` L ` + num(cx) + ` ` + num(body+16) +
		` M ` + num(cx-11) + ` ` + num(body+6) + ` L ` + num(cx+11) + ` ` + num(body+6) +
		` M ` + num(cx) + ` ` + num(body+16) + ` L ` + num(cx-9) + ` ` + num(body+28) +
		` M ` + num(cx) + ` ` + num(body+16) + ` L ` + num(cx+9) + ` ` + num(body+28) + `"/>` + "\n"
}

// labelY puts the text where the shape leaves room for it. A figure wears its
// name underneath; everything else holds it in the middle.
func labelY(family string, box artifact.Box) float64 {
	if family == "usecase" && box.Stereotype != "" {
		return box.Bottom() - 2
	}
	if family == "sequence" {
		return box.Y + box.H/2 + 5
	}
	return box.Y + box.H/2 + 5
}

func arrowDefs() string {
	return `  <defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5"` +
		` markerWidth="6" markerHeight="6" orient="auto-start-reverse">` +
		`<path d="M 0 0 L 10 5 L 0 10 z"/></marker></defs>` + "\n"
}

// pathData writes the polyline as an SVG path.
func pathData(points []artifact.Point) string {
	if len(points) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("M " + num(points[0].X) + " " + num(points[0].Y))
	for _, p := range points[1:] {
		b.WriteString(" L " + num(p.X) + " " + num(p.Y))
	}
	return b.String()
}

// pointsValue writes the polyline for the composition checker, which reads it
// back rather than re-deriving it from the path.
func pointsValue(points []artifact.Point) string {
	parts := make([]string, len(points))
	for i, p := range points {
		parts[i] = num(p.X) + "," + num(p.Y)
	}
	return strings.Join(parts, " ")
}

// boundsValue writes a rectangle as four numbers.
func boundsValue(x, y, w, h float64) string {
	return num(x) + "," + num(y) + "," + num(w) + "," + num(h)
}

func esc(s string) string { return html.EscapeString(s) }
