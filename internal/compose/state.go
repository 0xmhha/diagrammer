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

	// States joined by a transition go near each other, and containment wins
	// where the two disagree: a frame drawn around a scatter is not a frame.
	// Ordering by id alone put states that talk to each other at opposite
	// corners, and the long routes between them crossed everything in between.
	byID := make(map[string]uml.State, len(m.States))
	ids := make([]string, 0, len(m.States))
	parentOf := make(map[string]string, len(m.States))
	for _, st := range m.States {
		byID[st.ID] = st
		ids = append(ids, st.ID)
		parentOf[st.ID] = st.Parent
	}
	sort.Strings(ids)

	adjacency := make([][2]string, 0, len(m.Transitions))
	for _, t := range m.Transitions {
		adjacency = append(adjacency, [2]string{t.From, t.To})
	}

	ordered := make([]uml.State, 0, len(m.States))
	for _, id := range orderByAdjacency(ids, adjacency, func(id string) string { return parentOf[id] }) {
		ordered = append(ordered, byID[id])
	}

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
