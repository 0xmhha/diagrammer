package render

import (
	"fmt"
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
	// The size the scene was laid out at, declared so the stylesheet can refuse
	// to draw it smaller. Everything in the drawing is sized against these
	// numbers: a label is fitted down to labelMinSize and no further, because
	// below that it is not worth drawing. Scaling the whole drawing to fit a
	// narrower page puts every label under that floor at once, and the floor
	// stops meaning anything. See docs/decisions.md.
	b.WriteString(`" style="min-width:` + num(scene.Width) + `px;min-height:` + num(scene.Height) + `px"`)
	b.WriteString(` ` + attrFamily + `="` + esc(scene.Family) + `"`)
	b.WriteString(` ` + attrLevel + `="` + esc(scene.Level) + `" ` + attrLevelTitle + `="` + esc(scene.Title) + `"`)
	// A drawing announces itself. role=img with a title and a description is
	// what a screen reader announces in place of the geometry, and the title
	// has to be the first child for that to work everywhere it is tried.
	titleID, descID := sceneIDs(scene.Level)
	b.WriteString(` role="img" aria-labelledby="` + titleID + ` ` + descID + `">` + "\n")
	b.WriteString(`  <title id="` + titleID + `">` + esc(scene.Title) + "</title>\n")
	b.WriteString(`  <desc id="` + descID + `">` + esc(describe(scene)) + "</desc>\n")
	b.WriteString(sceneBody(scene))
	b.WriteString("</svg>")
	return b.String()
}

// sceneBody is the drawing itself: the defs, then bands, lines, boxes and
// labels in that order. It is what the page wraps in a scene and what a
// standalone file wraps in a frame, and it is one function so the two cannot
// draw differently.
func sceneBody(scene artifact.Scene) string {
	var b strings.Builder
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
		//
		// The size and the bounds travel with it. The label rule measures the
		// text, and how wide text is depends on the size this one label was
		// shrunk to, which nothing in the string says.
		// Beside a vertical line, centred above a horizontal one. The dy is
		// what moves the baseline: text hangs off its baseline rather than
		// sitting on its top edge, so the offset differs with the anchor.
		x, dy := r.LabelAt.X, -float64(labelLift)
		if r.LabelAnchor == anchorStart {
			x, dy = r.LabelAt.X+labelLift, r.LabelSize*baselineShare
		}
		b.WriteString(`  <text class="edge-label" ` + attrEdgeLabelFor + `="` + esc(r.ID) +
			`" x="` + num(x) + `" y="` + num(r.LabelAt.Y) +
			`" dy="` + num(dy) + `" text-anchor="` + esc(anchorOf(r)) +
			`" font-size="` + num(r.LabelSize) +
			`" ` + attrLabelBounds + `="` +
			boundsValue(r.LabelBounds.X, r.LabelBounds.Y, r.LabelBounds.W, r.LabelBounds.H) + `">` +
			esc(r.LabelText) + `</text>` + "\n")
	}

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
// baselineShare is how far below the middle of a line of text its baseline
// sits, as a share of the font size. It is what puts text written beside a line
// level with the point it names rather than above it.
const baselineShare = 0.35

// anchorOf is the anchor the page should use, defaulting to the one the
// stylesheet already sets so a route written before anchors existed still
// draws where it always did.
func anchorOf(r artifact.Route) string {
	if r.LabelAnchor == "" {
		return anchorMiddle
	}
	return r.LabelAnchor
}

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

// sceneIDs names the title and the description of one drawing, uniquely on
// the page. A level id is any text; an HTML id is not, so it is reduced to the
// characters every reader of an id accepts, and prefixed so it can be told from
// an id something else on the page chose.
func sceneIDs(level string) (title, desc string) {
	slug := idSlug(level)
	return "diagram-" + slug + "-title", "diagram-" + slug + "-desc"
}

func idSlug(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// describe is what a reader who cannot see the drawing is told about it.
func describe(scene artifact.Scene) string {
	return fmt.Sprintf("%s diagram, %s: %d boxes and %d relationships drawn",
		scene.Family, scene.Title, len(scene.Boxes), len(scene.Routes))
}
