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
	b.WriteString(`" ` + attrLevel + `="` + esc(scene.Level) + `" ` + attrLevelTitle + `="` + esc(scene.Title) + `">` + "\n")
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

	for _, box := range scene.Boxes {
		b.WriteString(`  <g class="box" ` + attrBoxID + `="` + esc(box.ID) + `"`)
		if box.Opens != "" {
			b.WriteString(` ` + attrBoxOpens + `="` + esc(box.Opens) + `"`)
		}
		b.WriteString(">\n")
		b.WriteString(`    <rect class="box-frame" x="` + num(box.X) + `" y="` + num(box.Y) +
			`" width="` + num(box.W) + `" height="` + num(box.H) + `" rx="8"/>` + "\n")
		b.WriteString(`    <text class="box-label" x="` + num(box.X+box.W/2) + `" y="` +
			num(box.Y+box.H/2+5) + `">` + esc(box.Label) + `</text>` + "\n")
		b.WriteString("  </g>\n")
	}

	for _, r := range scene.Routes {
		if r.LabelText == "" {
			continue
		}
		b.WriteString(`  <text class="edge-label" ` + attrEdgeLabelFor + `="` + esc(r.ID) +
			`" x="` + num(r.LabelAt.X) + `" y="` + num(r.LabelAt.Y-6) + `">` + esc(r.LabelText) + `</text>` + "\n")
	}

	b.WriteString("</svg>")
	return b.String()
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

func esc(s string) string { return html.EscapeString(s) }
