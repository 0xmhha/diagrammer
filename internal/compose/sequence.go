package compose

import (
	"fmt"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// Sequence composes the sequence family.
//
// A sequence diagram is a ladder rather than a grid: lifelines stand in
// columns and messages sit on rows, in the order the model wrote them. That
// order is the model's array order and nothing else, because a second way to
// express order is a second thing that can disagree.
//
// Nothing is ever dropped here. A grid cannot route every relationship, which
// is why the component family needs a record; a ladder has a row for every
// message and no reason to refuse one. The accounting is still carried, and the
// rule still asserts that dropped is zero, so the day something does start
// dropping messages it is a failure rather than a silence.
func Sequence(source string, model *uml.Model) (*diagram.Document, error) {
	m := model.Sequence
	if m == nil {
		return nil, fmt.Errorf("the model carries no sequence section")
	}

	column := make(map[string]int, len(m.Lifelines))
	boxes := make([]diagram.Box, 0, len(m.Lifelines))
	for i, l := range m.Lifelines {
		column[l.ID] = i
		stereotype := ""
		if l.Actor {
			stereotype = "actor"
		}
		boxes = append(boxes, diagram.Box{
			ID:         l.ID,
			Label:      l.Name,
			Stereotype: stereotype,
			Row:        0,
			Col:        i,
		})
	}

	// Rows are the ladder's rungs. Lifelines all sit on row zero; the rest of
	// the rows belong to the messages.
	row := make(map[string]int, len(m.Messages))
	// The row numbers are held in a slice so each connection can point at its
	// own, rather than at a loop variable every one of them would share.
	messageRows := make([]int, len(m.Messages))
	connections := make([]diagram.Connection, 0, len(m.Messages))
	for i, msg := range m.Messages {
		row[msg.ID] = i
		messageRows[i] = i
		connections = append(connections, diagram.Connection{
			ID:      msg.ID,
			From:    msg.From,
			To:      msg.To,
			Kind:    diagram.KindMessage,
			Label:   msg.Name,
			Order:   &messageRows[i],
			Variant: diagram.MessageVariant(msg.Kind),
		})
	}

	activations := make([]diagram.Activation, 0, len(m.Activations))
	for _, a := range m.Activations {
		activations = append(activations, diagram.Activation{
			ID:      a.ID,
			Box:     a.Lifeline,
			FromRow: row[a.Start],
			ToRow:   row[a.End],
		})
	}

	fragments := make([]diagram.Fragment, 0, len(m.Fragments))
	for _, f := range m.Fragments {
		fragments = append(fragments, composeFragment(f, row, column, m))
	}

	rows := len(m.Messages)
	if rows == 0 {
		rows = 1
	}
	level := diagram.Level{
		ID:          overviewLevelID,
		Title:       model.Meta.Title,
		Grid:        diagram.Grid{Rows: rows, Cols: len(m.Lifelines)},
		Boxes:       boxes,
		Connections: connections,
		Activations: activations,
		Fragments:   fragments,
		Accounting: diagram.Accounting{
			// Both messages and activations have to appear, so both are what
			// this page was responsible for.
			Proven: len(connections) + len(activations),
			Drawn:  len(connections) + len(activations),
		},
	}

	return &diagram.Document{
		SchemaVersion: 1,
		Family:        diagram.FamilySequence,
		Meta:          diagram.Meta{Title: model.Meta.Title, Subtitle: model.Meta.Subtitle},
		Provenance:    provenanceOf(source, model),
		Levels:        []diagram.Level{level},
		Accounting:    level.Accounting,
	}, nil
}

// composeFragment frames the rows a fragment covers and the columns it spans.
//
// The span is taken from the messages inside it: a fragment reaches as wide as
// the lifelines its messages touch, because a frame narrower than that would
// cut through a line it is supposed to contain.
func composeFragment(f uml.Fragment, row, column map[string]int, m *uml.SequenceModel) diagram.Fragment {
	messageByID := make(map[string]uml.Message, len(m.Messages))
	for _, msg := range m.Messages {
		messageByID[msg.ID] = msg
	}

	out := diagram.Fragment{
		ID:      f.ID,
		Kind:    diagram.FragmentKind(f.Kind),
		FromRow: -1,
		FromCol: -1,
	}
	for _, op := range f.Operands {
		operand := diagram.FragmentOperand{Guard: op.Guard, FromRow: -1}
		for _, id := range op.Messages {
			r := row[id]
			if operand.FromRow < 0 || r < operand.FromRow {
				operand.FromRow = r
			}
			if out.FromRow < 0 || r < out.FromRow {
				out.FromRow = r
			}
			if r > out.ToRow {
				out.ToRow = r
			}
			msg := messageByID[id]
			for _, c := range []int{column[msg.From], column[msg.To]} {
				if out.FromCol < 0 || c < out.FromCol {
					out.FromCol = c
				}
				if c > out.ToCol {
					out.ToCol = c
				}
			}
		}
		if operand.FromRow < 0 {
			operand.FromRow = 0
		}
		out.Operands = append(out.Operands, operand)
	}
	if out.FromRow < 0 {
		out.FromRow = 0
	}
	if out.FromCol < 0 {
		out.FromCol = 0
	}
	return out
}
