package compose

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// Thresholds deciding how many boxes a page may hold.
//
// They come from the reference implementation, where they were arrived at by
// measuring real repositories rather than chosen. They have not been re-measured
// here, and docs/thresholds.md says so; a change to them should be made the same
// way they were made, by measuring, rather than by taste.
const (
	// minBoxesPerLevel is the point below which a page is too thin to be worth
	// opening, and unfolds a generation instead.
	minBoxesPerLevel = 6
	// expandedMaxBoxes is the ceiling an unfold may not push a page past. A
	// page with more boxes than this cannot be read, so the thin page is the
	// lesser problem and the unfold is abandoned.
	expandedMaxBoxes = 24
)

const (
	overviewLevelID = "overview"
	levelIDPrefix   = "level:"
	generatedBy     = "diagrammer compose"
)

// Component composes the component family.
//
// model must already have passed validate: compose assumes a validated model
// rather than re-checking one, so that there is exactly one place a malformed
// document is refused.
func Component(source string, model *uml.Model) (*diagram.Document, error) {
	if model.Component == nil {
		return nil, fmt.Errorf("the model carries no component section")
	}
	tree := newComponentTree(model.Component)
	relationships := componentRelationships(model.Component)

	levels := buildLevels(tree, endpointsOf(relationships))
	out := make([]diagram.Level, 0, len(levels))
	total := diagram.Accounting{}
	for _, l := range levels {
		composed := composeLevel(l, tree, relationships)
		total.Proven += composed.Accounting.Proven
		total.Drawn += composed.Accounting.Drawn
		total.Dropped += composed.Accounting.Dropped
		out = append(out, composed)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	return &diagram.Document{
		SchemaVersion: 1,
		Family:        diagram.FamilyComponent,
		Meta: diagram.Meta{
			Title:    model.Meta.Title,
			Subtitle: model.Meta.Subtitle,
		},
		Provenance: diagram.Provenance{Model: source, GeneratedBy: generatedBy},
		Levels:     out,
		Accounting: total,
	}, nil
}

// --- the nesting tree ---------------------------------------------------------

// componentTree indexes a model's components by id and by parent.
type componentTree struct {
	byID     map[string]uml.Component
	children map[string][]string // parent id -> child ids, sorted
	roots    []string            // sorted
}

func newComponentTree(m *uml.ComponentModel) *componentTree {
	t := &componentTree{
		byID:     make(map[string]uml.Component, len(m.Components)),
		children: make(map[string][]string),
	}
	for _, c := range m.Components {
		t.byID[c.ID] = c
	}
	for _, c := range m.Components {
		if c.Parent == "" {
			t.roots = append(t.roots, c.ID)
			continue
		}
		t.children[c.Parent] = append(t.children[c.Parent], c.ID)
	}
	sort.Strings(t.roots)
	for parent := range t.children {
		sort.Strings(t.children[parent])
	}
	return t
}

// ancestors walks from a component up to its root, itself first.
func (t *componentTree) ancestors(id string) []string {
	var out []string
	for cur, steps := id, 0; cur != ""; steps++ {
		if steps > len(t.byID) {
			// validate rejects containment cycles, so this cannot happen on a
			// validated model. The guard is here because a silent infinite loop
			// is a far worse failure than a truncated answer.
			break
		}
		out = append(out, cur)
		cur = t.byID[cur].Parent
	}
	return out
}

// --- relationships ------------------------------------------------------------

// relationship is one statement the model makes about two components.
type relationship struct {
	from  string
	to    string
	kind  diagram.RelationshipKind
	label string
	iface string
	id    string
}

// componentRelationships collects everything the model says about which
// components reach which, in a deterministic order.
//
// Two sources feed it. A dependency is stated outright. An assembly is derived:
// one component requires an interface another provides, which UML draws as the
// two halves of a connector meeting, and which is a relationship the model
// implies without writing down.
func componentRelationships(m *uml.ComponentModel) []relationship {
	components := make(map[string]bool, len(m.Components))
	for _, c := range m.Components {
		components[c.ID] = true
	}

	var out []relationship
	for _, d := range m.Dependencies {
		// A dependency may end on an interface rather than a component. Those
		// are drawn on the box as a port, not as a line between boxes, so they
		// are not relationships this diagram has to route.
		if !components[d.From] || !components[d.To] {
			continue
		}
		out = append(out, relationship{
			from:  d.From,
			to:    d.To,
			kind:  diagram.KindDependency,
			label: d.Name,
			id:    "rel:" + d.ID,
		})
	}

	// providers maps an interface to every component offering it.
	providers := make(map[string][]string)
	for _, c := range m.Components {
		for _, p := range c.Ports {
			if p.Kind == uml.PortProvided {
				providers[p.Interface] = append(providers[p.Interface], c.ID)
			}
		}
	}
	for iface := range providers {
		sort.Strings(providers[iface])
	}
	for _, c := range m.Components {
		for _, p := range c.Ports {
			if p.Kind != uml.PortRequired {
				continue
			}
			for _, provider := range providers[p.Interface] {
				if provider == c.ID {
					continue // a component satisfying its own requirement draws nothing
				}
				out = append(out, relationship{
					from:  c.ID,
					to:    provider,
					kind:  diagram.KindAssembly,
					iface: p.Interface,
					id:    "rel:" + p.ID + ":" + provider,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// --- levels -------------------------------------------------------------------

// plannedLevel is a page before its relationships have been worked out.
type plannedLevel struct {
	id        string
	title     string
	parent    string
	opensFrom string
	// members are the components drawn as boxes here, sorted.
	members []string
	// regions are components that were unfolded away: their children are boxes
	// on this page, so they are drawn as a band instead of a box.
	regions []string
}

// buildLevels lays the nesting tree out as pages.
//
// The overview holds the roots, and every box with children opens a page of its
// own. A page too thin to be worth opening unfolds a generation instead, and
// the component it unfolded becomes a band framing the children it gave up its
// box for. That band is what stops an unfolded page from reading as a flat
// list of things with no relation to each other.
func buildLevels(t *componentTree, engaged map[string]bool) []plannedLevel {
	var out []plannedLevel
	var walk func(owner string, members []string, parent, opensFrom, title string)

	walk = func(owner string, members []string, parent, opensFrom, title string) {
		level := plannedLevel{
			id:        levelID(owner),
			title:     title,
			parent:    parent,
			opensFrom: opensFrom,
			members:   append([]string(nil), members...),
		}

		// A thin page unfolds one generation, provided the result still fits.
		if len(level.members) < minBoxesPerLevel {
			if unfolded, regions, ok := unfold(t, level.members, engaged); ok {
				level.members = unfolded
				level.regions = regions
			}
		}
		sort.Strings(level.members)
		sort.Strings(level.regions)

		// Whatever is still a box on this page and has children of its own
		// opens a page below. A component that was unfolded does not: its
		// children are already here.
		var descend []string
		for _, id := range level.members {
			if len(t.children[id]) > 0 {
				descend = append(descend, id)
			}
		}
		out = append(out, level)
		for _, id := range descend {
			walk(id, t.children[id], level.id, id, t.byID[id].Name)
		}
	}

	walk("", t.roots, "", "", "Overview")
	return out
}

// endpointsOf lists every component that is one end of some relationship.
func endpointsOf(relationships []relationship) map[string]bool {
	engaged := make(map[string]bool)
	for _, r := range relationships {
		engaged[r.from] = true
		engaged[r.to] = true
	}
	return engaged
}

// unfold replaces a member that has children with those children, turning the
// member into a band.
//
// A member that is one end of a relationship is left alone, however thin the
// page. A band is not a box and has no edge for a line to meet, so unfolding
// such a component would leave its relationships with nothing to attach to, and
// they would have to be dropped from a page that could perfectly well have
// shown them. A thin page is a smaller loss than a page that hides an edge.
//
// It reports false when the result would pass the ceiling a page can be read
// at, because a page nobody can read is worse than a thin one, and when nothing
// would change, so a page of leaves is not pointlessly rebuilt.
func unfold(t *componentTree, members []string, engaged map[string]bool) (unfolded, regions []string, ok bool) {
	changed := false
	for _, id := range members {
		kids := t.children[id]
		if len(kids) == 0 || engaged[id] {
			unfolded = append(unfolded, id)
			continue
		}
		changed = true
		regions = append(regions, id)
		unfolded = append(unfolded, kids...)
	}
	if !changed || len(unfolded) > expandedMaxBoxes {
		return nil, nil, false
	}
	return unfolded, regions, true
}

func levelID(owner string) string {
	if owner == "" {
		return overviewLevelID
	}
	return levelIDPrefix + owner
}

// --- composing one level ------------------------------------------------------

// composeLevel works out what one page shows and what it cannot.
//
// Every relationship whose two ends are both represented here is this page's
// responsibility. It is drawn when the two ends land on different boxes, and
// recorded on the box when they land on the same one, which happens where both
// ends live inside one component that this page only shows the outside of. The
// relationship is not lost: it is drawn on the page below, where the two ends
// are separate boxes. Counting it here anyway is what makes the record
// honest — a reader looking at this page can see that something inside that box
// was not shown.
func composeLevel(l plannedLevel, t *componentTree, relationships []relationship) diagram.Level {
	// represents maps any component to the box standing for it on this page.
	represents := make(map[string]string)
	onLevel := make(map[string]bool, len(l.members))
	for _, id := range l.members {
		onLevel[id] = true
	}
	// Every component either lands on a box here or has an ancestor that does,
	// or it belongs to a different page entirely. Nothing can land on a band
	// instead: unfold refuses to turn a component with relationships into one,
	// so a relationship always has two box ends or is none of this page's
	// business.
	for id := range t.byID {
		for _, ancestor := range t.ancestors(id) {
			if onLevel[ancestor] {
				represents[id] = ancestor
				break
			}
		}
	}

	regionOf := make(map[string]string, len(l.members))
	for _, region := range l.regions {
		for _, child := range t.children[region] {
			regionOf[child] = region
		}
	}

	dropped := make(map[string][]diagram.DroppedRelationship)
	// Built empty rather than nil. A level that draws nothing is a perfectly
	// ordinary page — a container whose children have no relationships among
	// them — and a nil slice encodes as null, which the schema refuses. The
	// difference never shows up until a model has such a level.
	connections := []diagram.Connection{}
	accounting := diagram.Accounting{}

	for _, r := range relationships {
		from, fromOK := represents[r.from]
		to, toOK := represents[r.to]
		if !fromOK || !toOK {
			continue // neither this page's business
		}
		accounting.Proven++
		if from == to {
			accounting.Dropped++
			dropped[from] = append(dropped[from], diagram.DroppedRelationship{
				To:     to,
				Kind:   r.kind,
				Reason: diagram.ReasonSelfReference,
			})
			continue
		}
		accounting.Drawn++
		connections = append(connections, diagram.Connection{
			ID:        r.id,
			From:      from,
			To:        to,
			Kind:      r.kind,
			Label:     r.label,
			Interface: r.iface,
		})
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })

	adjacency := make([][2]string, 0, len(connections))
	for _, c := range connections {
		adjacency = append(adjacency, [2]string{c.From, c.To})
	}
	boxes, grid := placeBoxes(l, t, regionOf, dropped, adjacency)

	regions := make([]diagram.Region, 0, len(l.regions))
	for _, id := range l.regions {
		c := t.byID[id]
		regions = append(regions, diagram.Region{ID: id, Label: c.Name, Stereotype: c.Stereotype})
	}

	return diagram.Level{
		ID:          l.id,
		Title:       l.title,
		Parent:      l.parent,
		OpensFrom:   l.opensFrom,
		Grid:        grid,
		Boxes:       boxes,
		Regions:     regions,
		Connections: connections,
		Accounting:  accounting,
	}
}

// placeBoxes lays the members out by dependency depth, in bands.
//
// The row is the depth: whatever nothing on this page depends on is at the top
// and what it rests on is beneath it, which is the direction a reader of a
// component diagram expects and the direction the router draws best. The
// columns belong to the bands, so a region frames a block rather than a scatter.
//
// It used to fill a square grid in walk order. Nothing in the picture then said
// which way anything depended on anything, and a relationship between two boxes
// the walk happened to separate became a detour across the page.
func placeBoxes(l plannedLevel, t *componentTree, regionOf map[string]string, dropped map[string][]diagram.DroppedRelationship, adjacency [][2]string) ([]diagram.Box, diagram.Grid) {
	sorted := append([]string(nil), l.members...)
	sort.Strings(sorted)
	ordered := orderByAdjacency(sorted, adjacency, func(id string) string { return regionOf[id] })

	rank := ranksOf(ordered, adjacency)
	placed, grid := cellsByRank(ordered, rank, func(id string) string { return regionOf[id] })

	boxes := make([]diagram.Box, 0, len(ordered))
	for _, id := range ordered {
		c := t.byID[id]
		cell := placed[id]
		box := diagram.Box{
			ID:         id,
			Label:      c.Name,
			Stereotype: c.Stereotype,
			Row:        cell[0],
			Col:        cell[1],
			Region:     regionOf[id],
			Dropped:    dropped[id],
		}
		// A component whose children are already on this page opens nothing:
		// there would be nothing new behind the door.
		if len(t.children[id]) > 0 {
			box.Opens = levelID(id)
		}
		for _, p := range c.Ports {
			box.Ports = append(box.Ports, diagram.Port{
				ID:        p.ID,
				Name:      p.Name,
				Kind:      diagram.PortKind(p.Kind),
				Interface: p.Interface,
			})
		}
		boxes = append(boxes, box)
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].ID < boxes[j].ID })
	return boxes, grid
}
