package render

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
)

// Page is a whole document, drawn.
type Page struct {
	Title    string
	Subtitle string
	Family   diagram.Family
	Scenes   []artifact.Scene
	// Accounting carries stage 3's drawn count forward as this stage's proven:
	// what the pages handed us is what we were responsible for showing.
	Accounting diagram.Accounting
	// Dropped records, per level and per box, the relationships the geometry
	// could not hold, each with the rule that refused it.
	Dropped []Drop
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

	page := &Page{Title: doc.Meta.Title, Subtitle: doc.Meta.Subtitle, Family: doc.Family}
	for _, level := range doc.Levels {
		scene, drops := buildScene(doc.Family, level)
		page.Scenes = append(page.Scenes, scene)
		page.Dropped = append(page.Dropped, drops...)

		page.Accounting.Proven += level.Accounting.Drawn
		page.Accounting.Drawn += len(scene.Routes)
		page.Accounting.Dropped += len(drops)
	}
	sort.Slice(page.Scenes, func(i, j int) bool { return page.Scenes[i].Level < page.Scenes[j].Level })
	return page, nil
}

// buildScene draws one level.
func buildScene(family diagram.Family, level diagram.Level) (artifact.Scene, []Drop) {
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
	placeLabels(&scene)

	var drops []Drop
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
	problems := invariant.Composition(&scene)
	refusedByRule := map[string]invariant.RouteProblem{}
	for _, p := range problems {
		if _, seen := refusedByRule[p.Route]; !seen {
			refusedByRule[p.Route] = p
		}
	}
	if len(refusedByRule) > 0 {
		kept := scene.Routes[:0]
		for _, r := range scene.Routes {
			p, bad := refusedByRule[r.ID]
			if !bad {
				kept = append(kept, r)
				continue
			}
			drops = append(drops, Drop{
				Level: level.ID, Box: r.From, Route: r.ID, Rule: p.Rule, Why: p.Detail,
			})
		}
		scene.Routes = kept
	}

	sort.Slice(drops, func(i, j int) bool { return drops[i].Route < drops[j].Route })
	return scene, drops
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
