package compose

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// State composes the state family.
//
// Every state becomes a box, composite ones included. A composite state is also
// listed as a band, because that is what it is: in UML the composite state's
// own box is the frame its substates sit inside. Being both is not a
// duplication, it is the one shape saying both things it has to say, and it is
// what lets a transition land on a composite state instead of having nowhere to
// point.
//
// Nothing is dropped. Every state and every transition has somewhere to go.
func State(source string, model *uml.Model) (*diagram.Document, error) {
	m := model.State
	if m == nil {
		return nil, fmt.Errorf("the model carries no state section")
	}

	// Ordering by containment first puts a composite state's children together,
	// so the frame around them covers a run rather than a scatter.
	ordered := make([]uml.State, len(m.States))
	copy(ordered, m.States)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Parent != ordered[j].Parent {
			return ordered[i].Parent < ordered[j].Parent
		}
		return ordered[i].ID < ordered[j].ID
	})

	grid := gridFor(len(ordered))
	boxes := make([]diagram.Box, 0, len(ordered))
	hasChildren := make(map[string]bool, len(ordered))
	for _, s := range ordered {
		if s.Parent != "" {
			hasChildren[s.Parent] = true
		}
	}
	for i, s := range ordered {
		boxes = append(boxes, diagram.Box{
			ID:         s.ID,
			Label:      s.Name,
			Stereotype: string(s.Kind),
			Row:        i / grid.Cols,
			Col:        i % grid.Cols,
			Region:     s.Parent,
		})
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].ID < boxes[j].ID })

	var regions []diagram.Region
	for _, s := range ordered {
		if !hasChildren[s.ID] {
			continue
		}
		regions = append(regions, diagram.Region{ID: s.ID, Label: s.Name, Stereotype: string(s.Kind)})
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].ID < regions[j].ID })

	connections := make([]diagram.Connection, 0, len(m.Transitions))
	for _, t := range m.Transitions {
		connections = append(connections, diagram.Connection{
			ID:    t.ID,
			From:  t.From,
			To:    t.To,
			Kind:  diagram.KindTransition,
			Label: transitionLabel(t),
		})
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })

	level := diagram.Level{
		ID:          overviewLevelID,
		Title:       model.Meta.Title,
		Grid:        grid,
		Boxes:       boxes,
		Regions:     regions,
		Connections: connections,
		Accounting: diagram.Accounting{
			Proven: len(connections),
			Drawn:  len(connections),
		},
	}

	return &diagram.Document{
		SchemaVersion: 1,
		Family:        diagram.FamilyState,
		Meta:          diagram.Meta{Title: model.Meta.Title, Subtitle: model.Meta.Subtitle},
		Provenance:    diagram.Provenance{Model: source, GeneratedBy: generatedBy},
		Levels:        []diagram.Level{level},
		Accounting:    level.Accounting,
	}, nil
}

// transitionLabel renders the UML form: trigger [guard] / effect.
//
// All three parts are optional, and a transition with none of them is taken as
// soon as its source completes, so it is labelled with nothing rather than with
// empty brackets.
func transitionLabel(t uml.Transition) string {
	var b strings.Builder
	b.WriteString(t.Trigger)
	if t.Guard != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("[" + t.Guard + "]")
	}
	if t.Effect != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("/ " + t.Effect)
	}
	return b.String()
}
