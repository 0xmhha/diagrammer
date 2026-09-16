package render

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
)

// The viewer is embedded rather than linked, which is what makes the artifact
// self-contained: it opens from a file with no server, no network and nothing
// else on disk.
var (
	//go:embed viewer/viewer.css
	viewerCSS string
	//go:embed viewer/viewer.js
	viewerJS string
)

// HTML assembles the whole page.
//
// Every level is written out and all but one hidden, so moving between them is
// a class change rather than a fetch. The record of what could not be drawn is
// written beside the drawing rather than into a log, because the person who
// needs it is the one looking at the diagram.
func (p *Page) HTML() string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>" + esc(p.Title) + "</title>\n")
	b.WriteString("<style>\n" + viewerCSS + "</style>\n</head>\n<body>\n")

	b.WriteString("<header>\n<h1>" + esc(p.Title) + "</h1>\n")
	if p.Subtitle != "" {
		b.WriteString("<p class=\"subtitle\">" + esc(p.Subtitle) + "</p>\n")
	}
	b.WriteString("</header>\n")

	b.WriteString("<nav>\n")
	for _, scene := range p.Scenes {
		b.WriteString(`<button ` + attrLevelLink + `="` + esc(scene.Level) + `" aria-current="false">` +
			esc(scene.Title) + "</button>\n")
	}
	b.WriteString("</nav>\n<main>\n")

	byLevel := p.dropsByLevel()
	for _, scene := range p.Scenes {
		b.WriteString(`<section ` + attrLevel + `="` + esc(scene.Level) + `" hidden>` + "\n")
		b.WriteString("<div class=\"level-head\">\n")
		b.WriteString("<h2>" + esc(scene.Title) + "</h2>\n")
		fmt.Fprintf(&b, "<span class=\"count\">%d boxes, %d relationships drawn</span>\n",
			len(scene.Boxes), len(scene.Routes))
		b.WriteString(`<button class="back" ` + attrLevelBack + `="1">back</button>` + "\n")
		b.WriteString("</div>\n")
		b.WriteString(svgFor(scene))
		b.WriteString("\n")
		b.WriteString(recordFor(byLevel[scene.Level]))
		b.WriteString("</section>\n")
	}

	b.WriteString("</main>\n<script>\n" + viewerJS + "</script>\n</body>\n</html>\n")
	return b.String()
}

// recordFor writes what a page could not draw, beside the page.
//
// A relationship the geometry could not hold is not a secret to keep in a log
// file. The reader looking at the diagram is the one who needs to know that
// something is missing and why.
func recordFor(drops []Drop) string {
	if len(drops) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<div class=\"record\">\n<h3>Not drawn</h3>\n")
	fmt.Fprintf(&b,
		"<p>%d relationship(s) here could not be drawn. They are listed so the diagram does not quietly leave them out.</p>\n",
		len(drops))
	b.WriteString("<ul>\n")
	for _, d := range drops {
		b.WriteString("<li><code>" + esc(d.Route) + "</code> from <code>" + esc(d.Box) + "</code>: " +
			esc(d.Why) + "</li>\n")
	}
	b.WriteString("</ul>\n</div>\n")
	return b.String()
}

func (p *Page) dropsByLevel() map[string][]Drop {
	out := map[string][]Drop{}
	for _, d := range p.Dropped {
		out[d.Level] = append(out[d.Level], d)
	}
	for level := range out {
		sort.Slice(out[level], func(i, j int) bool { return out[level][i].Route < out[level][j].Route })
	}
	return out
}
