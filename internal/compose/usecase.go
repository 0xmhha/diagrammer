package compose

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// Usecase composes the use case family.
//
// The layout says what the diagram means: actors stand in the first column,
// outside the system, and the use cases fill the columns to their right, inside
// the band that frames the system. Where a thing sits is the claim the model
// made about which side of the boundary it is on, so the placement is not a
// convenience here.
//
// Nothing is dropped. Every actor, use case, association, include and extend
// has a place.
func Usecase(source string, model *uml.Model) (*diagram.Document, error) {
	m := model.Usecase
	if m == nil {
		return nil, fmt.Errorf("the model carries no usecase section")
	}

	actors := make([]uml.Actor, len(m.Actors))
	copy(actors, m.Actors)
	sort.Slice(actors, func(i, j int) bool { return actors[i].ID < actors[j].ID })

	usecases := make([]uml.Usecase, len(m.Usecases))
	copy(usecases, m.Usecases)
	sort.Slice(usecases, func(i, j int) bool { return usecases[i].ID < usecases[j].ID })

	// The first column is reserved for the actors, which is what puts them
	// outside the system band rather than merely near it.
	inner := gridFor(len(usecases))
	grid := diagram.Grid{
		Rows: max(inner.Rows, len(actors)),
		Cols: inner.Cols + 1,
	}

	boxes := make([]diagram.Box, 0, len(actors)+len(usecases))
	for i, a := range actors {
		kind := a.Kind
		if kind == "" {
			kind = uml.ActorPrimary
		}
		boxes = append(boxes, diagram.Box{
			ID:         a.ID,
			Label:      a.Name,
			Stereotype: string(kind),
			Row:        i,
			Col:        0,
		})
	}
	for i, u := range usecases {
		boxes = append(boxes, diagram.Box{
			ID:     u.ID,
			Label:  u.Name,
			Row:    i / inner.Cols,
			Col:    1 + i%inner.Cols,
			Region: m.System.ID,
		})
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].ID < boxes[j].ID })

	connections := make([]diagram.Connection, 0, len(m.Associations)+len(m.Includes)+len(m.Extends))
	for _, a := range m.Associations {
		connections = append(connections, diagram.Connection{
			ID: a.ID, From: a.Actor, To: a.Usecase, Kind: diagram.KindAssociation,
		})
	}
	for _, inc := range m.Includes {
		connections = append(connections, diagram.Connection{
			ID: inc.ID, From: inc.From, To: inc.To, Kind: diagram.KindInclude,
		})
	}
	for _, ext := range m.Extends {
		// The label carries the condition and the point it attaches at, because
		// an extend with neither is an arrow that says only "sometimes".
		label := ext.Condition
		if ext.ExtensionPoint != "" {
			if label != "" {
				label += " "
			}
			label += "at " + ext.ExtensionPoint
		}
		connections = append(connections, diagram.Connection{
			ID: ext.ID, From: ext.From, To: ext.To, Kind: diagram.KindExtend, Label: label,
		})
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })

	level := diagram.Level{
		ID:          overviewLevelID,
		Title:       model.Meta.Title,
		Grid:        grid,
		Boxes:       boxes,
		Regions:     []diagram.Region{{ID: m.System.ID, Label: m.System.Name, Stereotype: "system"}},
		Connections: connections,
		Accounting: diagram.Accounting{
			Proven: len(connections),
			Drawn:  len(connections),
		},
	}

	return &diagram.Document{
		SchemaVersion: 1,
		Family:        diagram.FamilyUsecase,
		Meta:          diagram.Meta{Title: model.Meta.Title, Subtitle: model.Meta.Subtitle},
		Provenance:    diagram.Provenance{Model: source, GeneratedBy: generatedBy},
		Levels:        []diagram.Level{level},
		Accounting:    level.Accounting,
	}, nil
}
