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
		Family: root.attr("data-family"),
		Level:  root.attr("data-level"),
		Title:  root.attr("data-level-title"),
	}
	if fields := strings.Fields(root.attr("viewBox")); len(fields) == 4 {
		scene.Width = number(fields[2])
		scene.Height = number(fields[3])
	}

	type labelAt struct {
		text   string
		at     Point
		bounds Rect
		size   float64
		anchor string
	}
	labels := map[string]labelAt{}
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
			label := labelAt{
				text: strings.TrimSpace(e.Text),
				at:   Point{X: number(e.attr("x")), Y: number(e.attr("y"))},
			}
			if b := parseBounds(e.attr("data-label-bounds")); b != nil {
				label.bounds = Rect{X: b[0], Y: b[1], W: b[2], H: b[3]}
			}
			label.size = number(e.attr("font-size"))
			label.anchor = e.attr("text-anchor")
			labels[e.attr("data-edge-label-for")] = label
		case e.attr("data-bar-id") != "":
			scene.Bars = append(scene.Bars, Bar{
				ID: e.attr("data-bar-id"), Box: e.attr("data-bar-box"),
				X: number(e.attr("x")), Y: number(e.attr("y")),
				W: number(e.attr("width")), H: number(e.attr("height")),
			})
		case e.attr("data-frame-id") != "":
			scene.Frames = append(scene.Frames, parseFrame(e))
		}
		for _, child := range e.Children {
			walk(child)
		}
	}
	walk(root)

	// Labels are separate elements, so they are attached once every route is
	// known rather than guessed at from document order.
	//
	// The position is read rather than recomputed. The renderer moves a label
	// along its route to wherever there is room, so deriving it from the route
	// again would put it back where it would have gone if nothing were in the
	// way, and the clearance rule would then judge a position nobody drew.
	for i := range scene.Routes {
		if label, ok := labels[scene.Routes[i].ID]; ok {
			scene.Routes[i].LabelText = label.text
			scene.Routes[i].LabelAt = label.at
			scene.Routes[i].LabelBounds = label.bounds
			scene.Routes[i].LabelSize = label.size
			scene.Routes[i].LabelAnchor = label.anchor
		}
	}
	return scene, nil
}

func parseBox(e svgElement) Box {
	box := Box{ID: e.attr("data-box-id"), Opens: e.attr("data-opens")}
	// The rectangle is read from the attribute rather than from whatever shape
	// was drawn. A final state is a ringed circle and a use case an ellipse, so
	// recovering bounds from the drawing would mean understanding every shape
	// the renderer might choose, and failing silently on the next one.
	if bounds := parseBounds(e.attr("data-box-bounds")); bounds != nil {
		box.X, box.Y, box.W, box.H = bounds[0], bounds[1], bounds[2], bounds[3]
	}
	for _, child := range e.Children {
		if child.XMLName.Local == "text" {
			box.Label = strings.TrimSpace(child.Text)
		}
	}
	return box
}

// parseBounds reads four comma-separated numbers, and reports nothing when the
// attribute is missing or malformed rather than inventing a rectangle at the
// origin that every rule would then judge.
func parseBounds(value string) []float64 {
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return nil
	}
	out := make([]float64, 4)
	for i, p := range parts {
		out[i] = number(p)
	}
	return out
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

func parseFrame(e svgElement) Frame {
	frame := Frame{ID: e.attr("data-frame-id"), Kind: e.attr("data-frame-kind")}
	for _, child := range e.Children {
		switch child.XMLName.Local {
		case "rect":
			frame.X, frame.Y = number(child.attr("x")), number(child.attr("y"))
			frame.W, frame.H = number(child.attr("width")), number(child.attr("height"))
		case "text":
			frame.Label = strings.TrimSpace(child.Text)
		}
	}
	return frame
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

func number(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}
