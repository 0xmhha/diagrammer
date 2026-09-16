package artifact

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Parse reads the drawn pages back out of an emitted document.
//
// It exists so the composition rules judge what was produced rather than what
// the renderer intended. Reading the artifact costs a parser; not reading it
// costs the meaning of every rule, because a renderer checking its own working
// state only ever proves that it agrees with itself.
//
// The drawings are SVG, which is XML, and this program wrote them, so they are
// parsed rather than pattern-matched. A regular expression over markup would
// hold right up until the first attribute someone reorders.
func Parse(document []byte) ([]Scene, error) {
	var scenes []Scene
	for _, block := range svgBlocks(string(document)) {
		scene, err := parseScene(block)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, scene)
	}
	return scenes, nil
}

// svgBlocks pulls each drawing out of the surrounding page.
func svgBlocks(document string) []string {
	var out []string
	rest := document
	for {
		start := strings.Index(rest, "<svg")
		if start < 0 {
			return out
		}
		end := strings.Index(rest[start:], "</svg>")
		if end < 0 {
			return out
		}
		end += start + len("</svg>")
		out = append(out, rest[start:end])
		rest = rest[end:]
	}
}

type svgElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr   `xml:",any,attr"`
	Text     string       `xml:",chardata"`
	Children []svgElement `xml:",any"`
}

func (e svgElement) attr(name string) string {
	for _, a := range e.Attrs {
		if a.Name.Local == localName(name) {
			return a.Value
		}
	}
	return ""
}

// localName strips the data- prefix, because Go's XML parser reports an
// attribute's local name without it only for namespaced names; data attributes
// keep theirs, so this is an identity for everything here and a guard against
// a caller passing a namespaced one.
func localName(name string) string { return name }

func parseScene(block string) (Scene, error) {
	var root svgElement
	if err := xml.Unmarshal([]byte(block), &root); err != nil {
		return Scene{}, fmt.Errorf("parse a drawing: %w", err)
	}

	scene := Scene{
		Level: root.attr("data-level"),
		Title: root.attr("data-level-title"),
	}
	if fields := strings.Fields(root.attr("viewBox")); len(fields) == 4 {
		scene.Width = number(fields[2])
		scene.Height = number(fields[3])
	}

	labels := map[string]string{}
	var walk func(e svgElement)
	walk = func(e svgElement) {
		switch {
		case e.attr("data-box-id") != "":
			scene.Boxes = append(scene.Boxes, parseBox(e))
		case e.attr("data-region-id") != "":
			scene.Regions = append(scene.Regions, parseRegion(e))
		case e.attr("data-edge-id") != "":
			scene.Routes = append(scene.Routes, parseRoute(e))
		case e.attr("data-edge-label-for") != "":
			labels[e.attr("data-edge-label-for")] = strings.TrimSpace(e.Text)
		}
		for _, child := range e.Children {
			walk(child)
		}
	}
	walk(root)

	// Labels are separate elements, so they are attached once every route is
	// known rather than guessed at from document order.
	for i := range scene.Routes {
		if text, ok := labels[scene.Routes[i].ID]; ok {
			scene.Routes[i].LabelText = text
			scene.Routes[i].LabelAt = labelPosition(scene.Routes[i])
		}
	}
	return scene, nil
}

func parseBox(e svgElement) Box {
	box := Box{ID: e.attr("data-box-id"), Opens: e.attr("data-opens")}
	for _, child := range e.Children {
		switch child.XMLName.Local {
		case "rect":
			box.X, box.Y = number(child.attr("x")), number(child.attr("y"))
			box.W, box.H = number(child.attr("width")), number(child.attr("height"))
		case "text":
			box.Label = strings.TrimSpace(child.Text)
		}
	}
	return box
}

func parseRegion(e svgElement) Region {
	region := Region{ID: e.attr("data-region-id")}
	for _, child := range e.Children {
		switch child.XMLName.Local {
		case "rect":
			region.X, region.Y = number(child.attr("x")), number(child.attr("y"))
			region.W, region.H = number(child.attr("width")), number(child.attr("height"))
		case "text":
			region.Label = strings.TrimSpace(child.Text)
		}
	}
	return region
}

func parseRoute(e svgElement) Route {
	return Route{
		ID:       e.attr("data-edge-id"),
		From:     e.attr("data-edge-from"),
		To:       e.attr("data-edge-to"),
		FromSide: e.attr("data-edge-from-side"),
		ToSide:   e.attr("data-edge-to-side"),
		Points:   parsePoints(e.attr("data-composition-points")),
	}
}

// parsePoints reads the routed polyline the renderer wrote out unrounded.
func parsePoints(value string) []Point {
	var out []Point
	for _, pair := range strings.Fields(value) {
		x, y, ok := strings.Cut(pair, ",")
		if !ok {
			continue
		}
		out = append(out, Point{X: number(x), Y: number(y)})
	}
	return out
}

// labelPosition recovers where a label sits, which is the middle of the route's
// longest straight run. The renderer puts it there and the checker has to look
// for it in the same place.
func labelPosition(r Route) Point {
	if len(r.Points) < 2 {
		return Point{}
	}
	best, bestLen := 0, -1.0
	for i := 0; i+1 < len(r.Points); i++ {
		dx := r.Points[i+1].X - r.Points[i].X
		dy := r.Points[i+1].Y - r.Points[i].Y
		if l := dx*dx + dy*dy; l > bestLen {
			best, bestLen = i, l
		}
	}
	a, b := r.Points[best], r.Points[best+1]
	return Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}

func number(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}
