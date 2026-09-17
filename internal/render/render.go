package render

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/vcs"
)

// Page is a whole document, drawn.
type Page struct {
	Title    string
	Subtitle string
	Family   diagram.Family
	// Revision is the commit the drawn source was checked out at, when the
	// document carried one. It is written into the page so that the person
	// looking at a box knows which version of the code it describes.
	Revision *vcs.Revision
	Scenes   []artifact.Scene
	// Accounting carries stage 3's drawn count forward as this stage's proven:
	// what the pages handed us is what we were responsible for showing.
	Accounting diagram.Accounting
	// Dropped records, per level and per box, the relationships the geometry
	// could not hold, each with the rule that refused it.
	Dropped []Drop
	// Unwritten records the connections that are drawn but carry no text,
	// because there was nowhere on the page to put it.
	//
	// It is kept apart from Dropped and out of the accounting on purpose. The
	// accounting counts relationships, and one of these is a relationship the
	// reader can see; what is missing is its name. Losing the whole line for
	// want of room for a word was the earlier behaviour and it cost more than
	// it saved.
	Unwritten []Unwritten
}

// Unwritten is one connection drawn without its text.
type Unwritten struct {
	Level string
	Box   string
	Route string
	Text  string
	Why   string
}

// Drop is one relationship a page could not draw.
type Drop struct {
	Level string
	Box   string
	Route string
	Rule  string
	Why   string
}

// Build draws a document.
//
// Every level is laid out, routed, and then judged by the composition rules. A
// route that breaks one is dropped and recorded against the box it left, which
// is the same bargain stage 3 makes: a relationship may go undrawn, but never
// untraced.
func Build(doc *diagram.Document) (*Page, error) {
	switch doc.Family {
	case diagram.FamilyComponent, diagram.FamilyState, diagram.FamilyUsecase:
		// These three are boxes joined by lines. What differs between them is
		// which shape a box takes and what a line means, and neither changes
		// where anything goes.
	case diagram.FamilySequence:
		return buildLadder(doc)
	default:
		return nil, fmt.Errorf("rendering the %q family is not implemented yet", doc.Family)
	}

	page := &Page{Title: doc.Meta.Title, Subtitle: doc.Meta.Subtitle, Family: doc.Family, Revision: doc.Provenance.Revision}
	for _, level := range doc.Levels {
		scene, drops, silent := buildScene(doc.Family, level)
		page.Scenes = append(page.Scenes, scene)
		page.Dropped = append(page.Dropped, drops...)
		page.Unwritten = append(page.Unwritten, silent...)

		page.Accounting.Proven += level.Accounting.Drawn
		page.Accounting.Drawn += len(scene.Routes)
		page.Accounting.Dropped += len(drops)
	}
	sort.Slice(page.Scenes, func(i, j int) bool { return page.Scenes[i].Level < page.Scenes[j].Level })
	return page, nil
}

// buildScene draws one level.
func buildScene(family diagram.Family, level diagram.Level) (artifact.Scene, []Drop, []Unwritten) {
	boxes, regions, g := layOut(family, level)
	paths, refused := routeAll(level, g, boxes)

	scene := artifact.Scene{
		Family: string(family),
		Level:  level.ID,
		Title:  level.Title,
		Width:  quantize(g.width),
		Height: quantize(g.height),
	}
	for _, b := range boxes {
		scene.Boxes = append(scene.Boxes, artifact.Box{
			ID: b.ID, Label: b.ShortLabel, Opens: b.Opens, Stereotype: b.Stereotype,
			X: quantize(b.X), Y: quantize(b.Y), W: quantize(b.W), H: quantize(b.H),
		})
	}
	for _, r := range regions {
		scene.Regions = append(scene.Regions, artifact.Region{
			ID: r.ID, Label: r.Label,
			X: quantize(r.X), Y: quantize(r.Y), W: quantize(r.W), H: quantize(r.H),
		})
	}
	for _, p := range paths {
		scene.Routes = append(scene.Routes, toArtifactRoute(p))
	}

	// Labels are placed before the rules are consulted, so a relationship is
	// only refused when there is nowhere on its route to write on, rather than
	// when the middle happens to be taken.
	//
	// A label with nowhere to go loses its text and keeps its line. The two are
	// not the same loss: a reader of a line without a name can still see that
	// the two boxes are joined, and a reader of neither cannot. What the page
	// must not do is lose the text quietly, so each one is recorded.
	unwritten := placeLabels(&scene)

	var drops []Drop
	var silent []Unwritten
	for _, u := range unwritten {
		u.Level = level.ID
		u.Box = sourceOf(level, u.Route)
		silent = append(silent, u)
	}
	// A route with no lane left never became a line at all, so it is recorded
	// before the rules are consulted: there is nothing for them to judge.
	for _, id := range sortedKeys(refused) {
		drops = append(drops, Drop{
			Level: level.ID, Box: sourceOf(level, id), Route: id,
			Rule: "channel-capacity", Why: refused[id].Error(),
		})
	}

	// The rules are applied to the scene as built, and anything they refuse is
	// taken out of it. Removing one route can only make the remaining drawing
	// sounder, never less so, which is why one pass is enough.
	//
	// Every rule but one condemns a route on its own, so those are settled
	// first and there is nothing to decide. Crossings are settled afterwards,
	// over what is left: a crossing condemns two routes jointly, and a pair
	// whose other half has already gone for some other reason no longer needs
	// settling at all.
	condemned := map[string]invariant.RouteProblem{}
	for _, p := range invariant.Composition(&scene) {
		if p.Rule == invariant.RuleCrossing {
			continue
		}
		if _, seen := condemned[p.Route]; !seen {
			condemned[p.Route] = p
		}
	}
	scene.Routes, drops = withoutRoutes(scene.Routes, drops, level.ID, func(id string) (string, string, bool) {
		p, bad := condemned[id]
		return p.Rule, p.Detail, bad
	})

	for _, c := range fewestCrossingRoutes(invariant.Crossings(&scene)) {
		condemned[c.route] = invariant.RouteProblem{
			Route: c.route, Rule: invariant.RuleCrossing,
			Detail: "crosses " + c.crosses + ", which it shares no end with",
		}
	}
	scene.Routes, drops = withoutRoutes(scene.Routes, drops, level.ID, func(id string) (string, string, bool) {
		p, bad := condemned[id]
		return p.Rule, p.Detail, bad && p.Rule == invariant.RuleCrossing
	})

	sort.Slice(drops, func(i, j int) bool { return drops[i].Route < drops[j].Route })
	// A route that went in the end takes its unwritten text with it: there is
	// no line left for the reader to wonder about the name of.
	gone := map[string]bool{}
	for _, d := range drops {
		gone[d.Route] = true
	}
	kept := silent[:0]
	for _, u := range silent {
		if !gone[u.Route] {
			kept = append(kept, u)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Route < kept[j].Route })
	return scene, drops, kept
}

// withoutRoutes takes the condemned routes out of the scene and records each
// one, so that what is drawn and what is recorded are decided in one place.
func withoutRoutes(routes []artifact.Route, drops []Drop, level string,
	condemned func(id string) (rule, why string, bad bool),
) ([]artifact.Route, []Drop) {
	kept := routes[:0]
	for _, r := range routes {
		rule, why, bad := condemned(r.ID)
		if !bad {
			kept = append(kept, r)
			continue
		}
		drops = append(drops, Drop{Level: level, Box: r.From, Route: r.ID, Rule: rule, Why: why})
	}
	return kept, drops
}

// crossingDrop is one route left out to settle a crossing, and one of the
// routes it crossed, so the record says what the conflict was.
type crossingDrop struct{ route, crosses string }

// fewestCrossingRoutes chooses which routes to leave out so that no crossing
// remains on the page.
//
// This is a vertex cover of the conflict graph, and the smallest one is a hard
// problem in general. These graphs hold tens of routes, and the greedy answer —
// take out whichever route crosses the most others, look again, repeat — is
// what a person would do by eye. What it replaced was not a smaller answer but
// an arbitrary one: the checker reported the earlier route of each pair and the
// renderer removed every route so reported, which is a cover only by accident
// of how the pairs sort. Measured over the fixtures and two real projects it
// leaves out 43 routes where blaming whichever sorted first left out 61.
//
// Ties go to the lower id, so the same page always leaves out the same routes.
func fewestCrossingRoutes(pairs [][2]string) []crossingDrop {
	live := make([][2]string, len(pairs))
	copy(live, pairs)

	var out []crossingDrop
	for len(live) > 0 {
		degree := map[string]int{}
		partner := map[string]string{}
		for _, p := range live {
			degree[p[0]]++
			degree[p[1]]++
			if _, ok := partner[p[0]]; !ok {
				partner[p[0]] = p[1]
			}
			if _, ok := partner[p[1]]; !ok {
				partner[p[1]] = p[0]
			}
		}
		worst := ""
		for id, n := range degree {
			if worst == "" || n > degree[worst] || (n == degree[worst] && id < worst) {
				worst = id
			}
		}
		out = append(out, crossingDrop{route: worst, crosses: partner[worst]})

		rest := live[:0]
		for _, p := range live {
			if p[0] != worst && p[1] != worst {
				rest = append(rest, p)
			}
		}
		live = rest
	}
	sort.Slice(out, func(i, j int) bool { return out[i].route < out[j].route })
	return out
}

func toArtifactRoute(p routed) artifact.Route {
	points := make([]artifact.Point, len(p.Points))
	for i, pt := range p.Points {
		points[i] = artifact.Point{X: quantize(pt.X), Y: quantize(pt.Y)}
	}
	return artifact.Route{
		ID:        p.ID,
		From:      p.From,
		To:        p.To,
		FromSide:  string(p.FromSide),
		ToSide:    string(p.ToSide),
		Points:    points,
		LabelAt:   artifact.Point{X: quantize(p.LabelAt.X), Y: quantize(p.LabelAt.Y)},
		LabelText: p.Label,
	}
}

// sourceOf finds the box a connection leaves, so a drop is recorded somewhere a
// reader will look.
func sourceOf(level diagram.Level, connection string) string {
	for _, c := range level.Connections {
		if c.ID == connection {
			return c.From
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
