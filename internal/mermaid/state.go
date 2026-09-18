package mermaid

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// The stereotype stage 3 puts on a state box is the UML state kind, and the
// pseudostates are what the grammar treats specially.
const (
	stereotypeInitial   = "initial"
	stereotypeFinal     = "final"
	stereotypeChoice    = "choice"
	stereotypeJunction  = "junction"
	stereotypeHistory   = "history"
	stereotypeComposite = "composite"
)

// pseudo is Mermaid's spelling of the start and the end.
const pseudo = "[*]"

// state writes one level in the state grammar.
//
// Every state is declared with its label, inside its composite where it has
// one. An initial or a final pseudostate is not declared at all: Mermaid has one
// spelling for both, and a transition touching one is written with it. Those
// transitions go inside the composite the pseudostate belongs to, because [*]
// inside a block means that block's start, and everything else goes at the top
// level, where the grammar lets it name a nested state.
func state(level *diagram.Level) (string, Accounting) {
	ids := newIDs(sourcesOf(level))
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n  direction TB\n")

	boxes := map[string]*diagram.Box{}
	byRegion := map[string][]*diagram.Box{}
	for i := range level.Boxes {
		box := &level.Boxes[i]
		boxes[box.ID] = box
		byRegion[box.Region] = append(byRegion[box.Region], box)
	}

	connections := append([]diagram.Connection(nil), level.Connections...)
	acc := Accounting{Proven: level.Accounting.Proven}
	recorded := recordedOn(level)
	connections = append(connections, recorded...)

	// A transition touching a pseudostate is written where that pseudostate
	// lives. Everything else is written once, at the top.
	inside := map[string][]diagram.Connection{}
	var top []diagram.Connection
	for _, c := range connections {
		region, local := pseudoRegion(c, boxes)
		if local {
			inside[region] = append(inside[region], c)
		} else {
			top = append(top, c)
		}
	}

	// A composite is both a box, so it has a cell on the page, and a region,
	// so its children have somewhere to be. In text it is a block and nothing
	// else, so its box is not declared a second time as a state.
	isRegion := map[string]bool{}
	for _, r := range level.Regions {
		isRegion[r.ID] = true
	}

	var declare func(region string, indent string)
	declare = func(region string, indent string) {
		for _, box := range byRegion[region] {
			if isRegion[box.ID] {
				continue
			}
			switch box.Stereotype {
			case stereotypeInitial, stereotypeFinal:
				continue
			case stereotypeChoice, stereotypeJunction:
				fmt.Fprintf(&b, "%sstate %s <<choice>>\n", indent, ids.of(box.ID))
			default:
				fmt.Fprintf(&b, "%sstate \"%s\" as %s\n", indent, label(box.Label), ids.of(box.ID))
			}
		}
		for _, r := range regionsUnder(level, region) {
			fmt.Fprintf(&b, "%sstate \"%s\" as %s {\n", indent, label(r.Label), ids.of(r.ID))
			declare(r.ID, indent+"  ")
			fmt.Fprintf(&b, "%s}\n", indent)
		}
		// After the blocks, so that a transition from a nested state to this
		// region's end reads below the block that declares that state. The
		// grammar takes either order; a reader takes this one.
		for _, c := range inside[region] {
			writeTransition(&b, indent, ids, &c, boxes)
		}
	}
	declare("", "  ")

	for i := range top {
		writeTransition(&b, "  ", ids, &top[i], boxes)
	}
	acc.Carried = len(connections)
	acc.Recorded = len(recorded)
	return b.String(), acc
}

// pseudoRegion reports whether c touches an initial or final pseudostate, and
// if so which region that pseudostate sits in.
func pseudoRegion(c diagram.Connection, boxes map[string]*diagram.Box) (string, bool) {
	if from, ok := boxes[c.From]; ok && from.Stereotype == stereotypeInitial {
		return from.Region, true
	}
	if to, ok := boxes[c.To]; ok && to.Stereotype == stereotypeFinal {
		return to.Region, true
	}
	return "", false
}

// regionsUnder lists the composites directly inside region, sorted, so nesting
// is written depth first and in one order.
//
// A composite is also a box, and that box's region is the composite's parent.
// That is where the nesting is read from; a region with no box of its own has
// no parent anyone can find and is written at the top.
func regionsUnder(level *diagram.Level, region string) []diagram.Region {
	parentOf := map[string]string{}
	for i := range level.Boxes {
		parentOf[level.Boxes[i].ID] = level.Boxes[i].Region
	}
	var out []diagram.Region
	for _, r := range level.Regions {
		if parentOf[r.ID] == region {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func writeTransition(b *strings.Builder, indent string, ids ids, c *diagram.Connection, boxes map[string]*diagram.Box) {
	from, to := ids.of(c.From), ids.of(c.To)
	if box, ok := boxes[c.From]; ok && box.Stereotype == stereotypeInitial {
		from = pseudo
	}
	if box, ok := boxes[c.To]; ok && box.Stereotype == stereotypeFinal {
		to = pseudo
	}
	if c.Label == "" {
		fmt.Fprintf(b, "%s%s --> %s\n", indent, from, to)
		return
	}
	fmt.Fprintf(b, "%s%s --> %s : %s\n", indent, from, to, statement(c.Label))
}
