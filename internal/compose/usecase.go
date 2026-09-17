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

	// A use case sits in the row of the actor that reaches it.
	//
	// The actors stand in one column, one per row, so a line to something in
	// the same row runs straight across and a line to another row has to climb
	// past whatever is between. Aligning the rows is what removes most of the
	// crossings; ordering by id alone, or clustering use cases by actor without
	// regard to which row that actor is on, both measured worse.
	byID := make(map[string]uml.Usecase, len(m.Usecases))
	for _, u := range m.Usecases {
		byID[u.ID] = u
	}
	actorRow := make(map[string]int, len(actors))
	for i, a := range actors {
		actorRow[a.ID] = i
	}

	// The first actor to reach a use case claims its row. Ties break on the
	// actor's own order, which is by id, so the result does not depend on the
	// order associations happen to be written in.
	rowOf := map[string]int{}
	associations := append([]uml.Association(nil), m.Associations...)
	sort.Slice(associations, func(i, j int) bool { return associations[i].ID < associations[j].ID })
	for _, a := range associations {
		row, ok := actorRow[a.Actor]
		if !ok {
			continue
		}
		if existing, claimed := rowOf[a.Usecase]; !claimed || row < existing {
			rowOf[a.Usecase] = row
		}
	}

	// A use case no actor reaches follows whatever includes or extends it, so
	// it lands beside the thing it belongs to rather than at the end.
	related := map[string][]string{}
	for _, inc := range m.Includes {
		related[inc.From] = append(related[inc.From], inc.To)
		related[inc.To] = append(related[inc.To], inc.From)
	}
	for _, ext := range m.Extends {
		related[ext.From] = append(related[ext.From], ext.To)
		related[ext.To] = append(related[ext.To], ext.From)
	}
	unplaced := []string{}
	for _, u := range m.Usecases {
		if _, ok := rowOf[u.ID]; !ok {
			unplaced = append(unplaced, u.ID)
		}
	}
	sort.Strings(unplaced)
	for changed := true; changed; {
		changed = false
		for _, id := range unplaced {
			if _, ok := rowOf[id]; ok {
				continue
			}
			best, found := 0, false
			for _, other := range related[id] {
				if row, ok := rowOf[other]; ok && (!found || row < best) {
					best, found = row, true
				}
			}
			if found {
				rowOf[id] = best
				changed = true
			}
		}
	}

	// Anything still unplaced goes on the shortest row, so the page stays as
	// square as the associations allow.
	rows := map[int][]string{}
	for id, row := range rowOf {
		rows[row] = append(rows[row], id)
	}
	for _, u := range m.Usecases {
		if _, ok := rowOf[u.ID]; ok {
			continue
		}
		shortest, size := 0, -1
		for r := range max(len(actors), 1) {
			if size < 0 || len(rows[r]) < size {
				shortest, size = r, len(rows[r])
			}
		}
		rowOf[u.ID] = shortest
		rows[shortest] = append(rows[shortest], u.ID)
	}
	for r := range rows {
		sort.Strings(rows[r])
	}

	// Two bands: the actors own the first column, which is what puts them
	// outside the system rather than merely near it, and the use cases own the
	// rest.
	//
	// Order within a row was alphabetical, which is an order the page cannot
	// use. A use case sat where its name put it, so a line from an actor to it
	// ran however far the alphabet decided, and a long run in a shared channel
	// is what crosses. The rows are already meaningful — a use case sits on the
	// row of the actor that reaches it — so only the columns needed arranging,
	// by the same barycentre sweep the other families use.
	const actorBand = ""
	rank := map[string]int{}
	band := map[string]string{}
	ordered := make([]string, 0, len(actors)+len(m.Usecases))
	for i, a := range actors {
		rank[a.ID], band[a.ID] = i, actorBand
		ordered = append(ordered, a.ID)
	}
	for row := range max(len(rows), max(len(actors), 1)) {
		for _, id := range rows[row] {
			rank[id], band[id] = row, m.System.ID
			ordered = append(ordered, id)
		}
	}
	bandOf := func(id string) string { return band[id] }

	adjacency := make([][2]string, 0, len(m.Associations)+len(m.Includes)+len(m.Extends))
	for _, a := range m.Associations {
		adjacency = append(adjacency, [2]string{a.Actor, a.Usecase})
	}
	for _, inc := range m.Includes {
		adjacency = append(adjacency, [2]string{inc.From, inc.To})
	}
	for _, ext := range m.Extends {
		adjacency = append(adjacency, [2]string{ext.From, ext.To})
	}
	ordered = orderWithin(ordered, rank, bandOf, adjacency)
	placed, grid := cellsByRank(ordered, rank, bandOf)

	boxes := make([]diagram.Box, 0, len(actors)+len(m.Usecases))
	for _, a := range actors {
		kind := a.Kind
		if kind == "" {
			kind = uml.ActorPrimary
		}
		cell := placed[a.ID]
		boxes = append(boxes, diagram.Box{
			ID:         a.ID,
			Label:      a.Name,
			Stereotype: string(kind),
			Row:        cell[0],
			Col:        cell[1],
		})
	}
	for _, u := range m.Usecases {
		cell, ok := placed[u.ID]
		if !ok {
			continue
		}
		boxes = append(boxes, diagram.Box{
			ID:     u.ID,
			Label:  u.Name,
			Row:    cell[0],
			Col:    cell[1],
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
