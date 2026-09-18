package render

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/0xmhha/diagrammer/internal/artifact"
)

// Size is a frame a drawing is delivered in.
//
// A page has no frame: the drawing is laid out at its own size and the page
// scrolls to it. A file handed to a deck or a document does have one, and it is
// the destination's rather than ours, so it is named for where it is going and
// not for what it measures. Every side is a multiple of four.
type Size struct {
	Name string
	W, H float64
}

// Fit is the frame that takes the drawing's own size: no scaling and no
// letterbox, which is what a vector hand-off wants.
var Fit = Size{Name: "fit"}

// LabelFloor is the smallest size a label is fitted to, in the drawing's own
// units. It is the number a framed file is measured against: a frame that
// scales the drawing scales its labels under this, and the command says so.
const LabelFloor = labelMinSize

// presets are the frames somebody is likely to want, in the order they are
// listed to a person. The dimensions are the ones a slide, a post and a printed
// page actually have.
var presets = []Size{
	Fit,
	{Name: "doc-inline", W: 960, H: 600},
	{Name: "doc-wide", W: 1280, H: 720},
	{Name: "slide-16x9", W: 1280, H: 720},
	{Name: "slide-4x3", W: 1024, H: 768},
	{Name: "social-og", W: 1200, H: 632},
	{Name: "social-square", W: 1080, H: 1080},
	{Name: "print-a4-landscape", W: 1120, H: 792},
	{Name: "print-letter-landscape", W: 1056, H: 816},
}

// Sizes lists every frame by name.
func Sizes() []string {
	out := make([]string, 0, len(presets))
	for _, s := range presets {
		out = append(out, s.Name)
	}
	return out
}

// SizeNamed returns the frame called name.
func SizeNamed(name string) (Size, bool) {
	for _, s := range presets {
		if s.Name == name {
			return s, true
		}
	}
	return Size{}, false
}

// Framed says what putting a drawing in a frame did to it.
//
// A frame smaller than the drawing scales it down, and every label with it,
// below the size the fitting refused to go under. That is the one thing about
// a framed file the reader cannot see and should be told: the page never does
// this, and this file does it on purpose because the destination has one size.
type Framed struct {
	// Scale is the factor the drawing was drawn at inside the frame. One is
	// the drawing's own size; under one is smaller.
	Scale float64
	// SmallestLabel is the size in the frame's own units of the smallest label
	// the fitting allows, after scaling. Under labelMinSize it is below the
	// floor the page holds.
	SmallestLabel float64
	W, H          float64
}

// Standalone writes one scene as a file that needs no page around it.
//
// It is the same drawing the page holds, with its styles resolved to plain
// values so nothing refers to a stylesheet that is not there, framed at size
// and centred in it. Interaction does not survive: there is no page to move
// between levels and nothing under the pointer.
func Standalone(scene artifact.Scene, size Size) (string, Framed) {
	frame := Framed{Scale: 1, W: scene.Width, H: scene.Height}
	if size.W > 0 && size.H > 0 {
		frame.W, frame.H = size.W, size.H
		frame.Scale = math.Min(size.W/scene.Width, size.H/scene.Height)
	}
	frame.SmallestLabel = labelMinSize * frame.Scale

	tx := (frame.W - scene.Width*frame.Scale) / 2
	ty := (frame.H - scene.Height*frame.Scale) / 2

	titleID, descID := sceneIDs(scene.Level)
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + num(frame.W) + " " + num(frame.H) + `"`)
	b.WriteString(` width="` + num(frame.W) + `" height="` + num(frame.H) + `"`)
	b.WriteString(` role="img" aria-labelledby="` + titleID + ` ` + descID + `">` + "\n")
	b.WriteString(`  <title id="` + titleID + `">` + esc(scene.Title) + "</title>\n")
	b.WriteString(`  <desc id="` + descID + `">` + esc(describe(scene)) + "</desc>\n")
	b.WriteString("  <style>\n" + sceneStyle() + "  </style>\n")
	fmt.Fprintf(&b, `  <g transform="translate(%s %s) scale(%s)">`+"\n", num(tx), num(ty), num(frame.Scale))
	b.WriteString(sceneBody(scene))
	b.WriteString("  </g>\n</svg>\n")
	return b.String(), frame
}

// SVGFileName is the file a level's standalone drawing is written to.
func SVGFileName(level string) string {
	return idSlug(level) + ".svg"
}

// sceneStyle is the page's stylesheet reduced to what a drawing needs on its
// own: the rules for what is inside the svg, with every token resolved to its
// light value, and nothing about hovering, opening or scrolling, none of which
// a file can do.
//
// It is derived from viewer.css rather than written a second time, so a rule
// changed on the page is changed in the file, and a test holds that nothing
// here still says var(.
var sceneStyle = sync.OnceValue(func() string {
	tokens := lightTokens(viewerCSS)
	var out []string
	for _, rule := range cssRule.FindAllStringSubmatch(viewerCSS, -1) {
		selector, body := strings.TrimSpace(rule[1]), strings.TrimSpace(rule[2])
		if !sceneSelector(selector) {
			continue
		}
		body = cssVar.ReplaceAllStringFunc(body, func(v string) string {
			name := cssVar.FindStringSubmatch(v)[1]
			if value, ok := tokens[name]; ok {
				return value
			}
			return v
		})
		out = append(out, "    "+selector+" { "+body+" }")
	}
	return strings.Join(out, "\n") + "\n"
})

var (
	cssRule     = regexp.MustCompile(`(?s)([^{}]+)\{([^}]*)\}`)
	cssVar      = regexp.MustCompile(`var\((--[a-z0-9-]+)\)`)
	cssComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssTokenDef = regexp.MustCompile(`(--[a-z0-9-]+):\s*([^;]+);`)
)

// lightTokens reads the first :root block, which is the light scheme. A file
// has no way to ask which scheme its reader prefers, so it gets the one that
// prints.
func lightTokens(css string) map[string]string {
	css = cssComment.ReplaceAllString(css, "")
	start := strings.Index(css, ":root")
	end := strings.Index(css[start:], "}")
	block := css[start : start+end]
	out := map[string]string{}
	for _, m := range cssTokenDef.FindAllStringSubmatch(block, -1) {
		out[m[1]] = strings.TrimSpace(m[2])
	}
	return out
}

// sceneSelector reports whether a rule is about something inside the drawing
// and true at rest. Rules about the page around it, about the pointer, about
// motion, and about a box that opens a page are for the page and are left out.
func sceneSelector(selector string) bool {
	selector = strings.TrimSpace(cssComment.ReplaceAllString(selector, ""))
	switch {
	case selector == "", strings.HasPrefix(selector, "@"), strings.HasPrefix(selector, ":root"):
		return false
	case strings.Contains(selector, ".focusing"), strings.Contains(selector, "data-opens"):
		return false
	case strings.HasPrefix(selector, "svg.scene"):
		return false // the page's sizing and its transitions
	}
	for _, part := range strings.Split(selector, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, ".") && !strings.HasPrefix(part, "#arrow") && !strings.HasPrefix(part, "circle.") {
			return false
		}
	}
	return !pageOnly[firstClass(selector)]
}

// pageOnly are the classes that style the page around the drawing rather than
// the drawing. They begin with a dot like the rest and are told apart by name.
var pageOnly = map[string]bool{
	".eyebrow": true, ".subtitle": true, ".revision": true, ".level-head": true,
	".back": true, ".scene-scroll": true, ".record": true, ".count": true,
}

func firstClass(selector string) string {
	end := strings.IndexAny(selector, " ,:[>")
	if end < 0 {
		return selector
	}
	return selector[:end]
}

// SortedSizes is Sizes, for a usage line: the names in the order listed.
func SortedSizes() string {
	names := Sizes()
	sort.Strings(names[1:]) // fit first, the rest alphabetical
	return strings.Join(names, ", ")
}
