package mermaid

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// A flowchart is what Mermaid has for boxes joined by lines with subgraphs
// around some of them, which is a component diagram, and with a stretch a use
// case diagram. The two families share the grammar and differ in what shape a
// box takes and what operator a line takes, which is all shapes carries.
type shapes struct {
	// box returns the node statement for one box: its safe id and label.
	box func(box *diagram.Box, id string) string
	// operator returns the link operator and label for one connection.
	operator func(c *diagram.Connection) (op string, label string)
}

// componentShapes draws every component as a plain box and a dependency or an
// assembly as an arrow, labelled with the interface where the model named one.
var componentShapes = shapes{
	box: func(box *diagram.Box, id string) string {
		return fmt.Sprintf(`%s["%s"]`, id, label(box.Label))
	},
	operator: func(c *diagram.Connection) (string, string) {
		text := c.Label
		if text == "" {
			text = c.Interface
		}
		return "-->", text
	},
}

// usecaseShapes draws an actor as a stadium and a use case as a rounded box,
// which is as close as the grammar comes to a stick figure and an ellipse. An
// association is a plain line, and include and extend are dashed arrows that
// carry the UML stereotype as their label, since nothing else in the grammar
// can say which of the two a dashed arrow is.
var usecaseShapes = shapes{
	box: func(box *diagram.Box, id string) string {
		if box.Region == "" {
			return fmt.Sprintf(`%s(["%s"])`, id, label(box.Label))
		}
		return fmt.Sprintf(`%s("%s")`, id, label(box.Label))
	},
	operator: func(c *diagram.Connection) (string, string) {
		switch c.Kind {
		case diagram.KindInclude:
			return "-.->", "«include»"
		case diagram.KindExtend:
			if c.Label != "" {
				return "-.->", "«extend» " + c.Label
			}
			return "-.->", "«extend»"
		case diagram.KindAssociation:
			return "---", c.Label
		default:
			return "-->", c.Label
		}
	},
}

// flowchart writes one level in the flowchart grammar.
//
// Boxes inside a region are declared inside a subgraph, which is what makes a
// band on the page a zone in the text. Connections follow, every one the level
// drew and then every one it recorded, because a flowchart lays itself out and
// has no reason to leave a line off.
func flowchart(level *diagram.Level, direction string, s shapes) (string, Accounting) {
	ids := newIDs(sourcesOf(level))
	var b strings.Builder
	fmt.Fprintf(&b, "flowchart %s\n", direction)

	byRegion := map[string][]*diagram.Box{}
	for i := range level.Boxes {
		box := &level.Boxes[i]
		byRegion[box.Region] = append(byRegion[box.Region], box)
	}
	for _, box := range byRegion[""] {
		fmt.Fprintf(&b, "  %s\n", s.box(box, ids.of(box.ID)))
	}
	regions := append([]diagram.Region(nil), level.Regions...)
	sort.Slice(regions, func(i, j int) bool { return regions[i].ID < regions[j].ID })
	for i := range regions {
		region := &regions[i]
		fmt.Fprintf(&b, "  subgraph %s [\"%s\"]\n", ids.of(region.ID), label(region.Label))
		for _, box := range byRegion[region.ID] {
			fmt.Fprintf(&b, "    %s\n", s.box(box, ids.of(box.ID)))
		}
		b.WriteString("  end\n")
	}

	acc := Accounting{Proven: level.Accounting.Proven}
	for i := range level.Connections {
		writeLink(&b, ids, &level.Connections[i], s)
		acc.Carried++
	}
	recorded := recordedOn(level)
	if len(recorded) > 0 {
		b.WriteString("  %% the page recorded these rather than drawing them\n")
	}
	for i := range recorded {
		writeLink(&b, ids, &recorded[i], s)
		acc.Carried++
		acc.Recorded++
	}
	return b.String(), acc
}

func writeLink(b *strings.Builder, ids ids, c *diagram.Connection, s shapes) {
	op, text := s.operator(c)
	if text == "" {
		fmt.Fprintf(b, "  %s %s %s\n", ids.of(c.From), op, ids.of(c.To))
		return
	}
	fmt.Fprintf(b, "  %s %s|%s| %s\n", ids.of(c.From), op, edgeLabel(text), ids.of(c.To))
}
