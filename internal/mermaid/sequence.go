package mermaid

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// stereotypeActor is what stage 3 puts on a lifeline drawn as a stick figure.
const stereotypeActor = "actor"

// arrows maps a message variant to the Mermaid operator that means the same.
//
// create has no operator of its own: Mermaid says it with a directive before
// the message, and the participant is declared up front here anyway, so a
// creating message is drawn as the synchronous call it also is. destroy is a
// message ending in a cross, which is what Mermaid draws for it too.
var arrows = map[diagram.MessageVariant]string{
	diagram.VariantSync:    "->>",
	diagram.VariantAsync:   "-)",
	diagram.VariantReply:   "-->>",
	diagram.VariantCreate:  "->>",
	diagram.VariantDestroy: "-x",
}

// fragmentKeywords maps a fragment kind to the keyword that opens it and the
// one that separates its operands.
//
// strict and seq have no Mermaid form. They are opened as opt, with the kind
// written into the guard so the reader sees what it was, and a comment says so
// beside it. Approximated and marked beats dropped.
var fragmentKeywords = map[diagram.FragmentKind]struct{ open, next string }{
	diagram.FragmentAlt:      {"alt", "else"},
	diagram.FragmentOpt:      {"opt", "else"},
	diagram.FragmentLoop:     {"loop", "else"},
	diagram.FragmentPar:      {"par", "and"},
	diagram.FragmentCritical: {"critical", "option"},
	diagram.FragmentBreak:    {"break", "else"},
	diagram.FragmentStrict:   {"opt", "else"},
	diagram.FragmentSeq:      {"opt", "else"},
}

// sequence writes one level in the sequence grammar.
//
// Lifelines are declared in column order. Messages follow in the order the
// document gave them, which is their meaning. Activations and fragments are
// intervals over message rows, and each is opened before the first message in
// it and closed after the last, outermost first on the way in and innermost
// first on the way out.
func sequence(level *diagram.Level) (string, Accounting) {
	ids := newIDs(sourcesOf(level))
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")

	lifelines := append([]diagram.Box(nil), level.Boxes...)
	sort.Slice(lifelines, func(i, j int) bool { return lifelines[i].Col < lifelines[j].Col })
	for i := range lifelines {
		l := &lifelines[i]
		kind := "participant"
		if l.Stereotype == stereotypeActor {
			kind = "actor"
		}
		fmt.Fprintf(&b, "  %s %s as %s\n", kind, ids.of(l.ID), statement(l.Label))
	}

	messages := make([]*diagram.Connection, 0, len(level.Connections))
	for i := range level.Connections {
		messages = append(messages, &level.Connections[i])
	}
	sort.SliceStable(messages, func(i, j int) bool { return rowOf(messages[i]) < rowOf(messages[j]) })

	brackets := bracketsOf(level)
	depth := 0
	indent := func() string { return strings.Repeat("  ", depth+1) }

	// An activation is a relationship the document proved, the same as a
	// message: the completeness rule for this family counts both. Each one is
	// carried as an activate and a deactivate below, so it is counted here.
	acc := Accounting{Proven: level.Accounting.Proven, Carried: len(level.Activations)}
	for _, m := range messages {
		row := rowOf(m)
		for _, f := range brackets.opens[row] {
			kw := fragmentKeywords[f.Kind]
			guard := ""
			if len(f.Operands) > 0 {
				guard = f.Operands[0].Guard
			}
			if kw.open == "opt" && f.Kind != diagram.FragmentOpt {
				fmt.Fprintf(&b, "%s%%%% %s fragment, which Mermaid has no keyword for\n", indent(), f.Kind)
				guard = strings.TrimSpace(string(f.Kind) + " " + guard)
			}
			fmt.Fprintf(&b, "%s%s %s\n", indent(), kw.open, statement(guard))
			depth++
		}
		for _, op := range brackets.operands[row] {
			fmt.Fprintf(&b, "%s%s %s\n", strings.Repeat("  ", depth), op.keyword, statement(op.guard))
		}
		for _, box := range brackets.activate[row] {
			fmt.Fprintf(&b, "%sactivate %s\n", indent(), ids.of(box))
		}

		arrow, ok := arrows[m.Variant]
		if !ok {
			arrow = arrows[diagram.VariantSync]
		}
		fmt.Fprintf(&b, "%s%s%s%s: %s\n", indent(), ids.of(m.From), arrow, ids.of(m.To), statement(m.Label))
		acc.Carried++

		for _, box := range brackets.deactivate[row] {
			fmt.Fprintf(&b, "%sdeactivate %s\n", indent(), ids.of(box))
		}
		for range brackets.closes[row] {
			depth--
			fmt.Fprintf(&b, "%send\n", indent())
		}
	}

	// A recorded message has no row, and a message with no row has no place
	// in a diagram whose order is its meaning. It is named rather than placed.
	for _, r := range recordedOn(level) {
		fmt.Fprintf(&b, "  %%%% recorded on the page and not placed here: %s to %s\n", ids.of(r.From), ids.of(r.To))
		acc.Omitted++
	}
	return b.String(), acc
}

// rowOf is the row a message sits on, which is its order. A message with no
// order is put first, which a valid document never produces.
func rowOf(m *diagram.Connection) int {
	if m.Order == nil {
		return -1
	}
	return *m.Order
}

// brackets indexes every interval on a level by the rows it opens and closes
// on, so writing the messages in order is one pass.
type brackets struct {
	opens      map[int][]*diagram.Fragment
	closes     map[int][]*diagram.Fragment
	operands   map[int][]operand
	activate   map[int][]string
	deactivate map[int][]string
}

type operand struct {
	keyword string
	guard   string
}

func bracketsOf(level *diagram.Level) brackets {
	out := brackets{
		opens:      map[int][]*diagram.Fragment{},
		closes:     map[int][]*diagram.Fragment{},
		operands:   map[int][]operand{},
		activate:   map[int][]string{},
		deactivate: map[int][]string{},
	}
	for i := range level.Fragments {
		f := &level.Fragments[i]
		out.opens[f.FromRow] = append(out.opens[f.FromRow], f)
		out.closes[f.ToRow] = append(out.closes[f.ToRow], f)
		kw := fragmentKeywords[f.Kind]
		for _, op := range f.Operands[min(1, len(f.Operands)):] {
			out.operands[op.FromRow] = append(out.operands[op.FromRow], operand{kw.next, op.Guard})
		}
	}
	// Outermost opens first and closes last: a wider fragment opens before a
	// narrower one on the same row, and the reverse on the way out. Ties fall
	// to id so the order is total.
	span := func(f *diagram.Fragment) int { return f.ToRow - f.FromRow }
	for row := range out.opens {
		fs := out.opens[row]
		sort.Slice(fs, func(i, j int) bool {
			if span(fs[i]) != span(fs[j]) {
				return span(fs[i]) > span(fs[j])
			}
			return fs[i].ID < fs[j].ID
		})
	}
	for row := range out.closes {
		fs := out.closes[row]
		sort.Slice(fs, func(i, j int) bool {
			if span(fs[i]) != span(fs[j]) {
				return span(fs[i]) < span(fs[j])
			}
			return fs[i].ID < fs[j].ID
		})
	}
	for i := range level.Activations {
		a := &level.Activations[i]
		out.activate[a.FromRow] = append(out.activate[a.FromRow], a.Box)
		out.deactivate[a.ToRow] = append(out.deactivate[a.ToRow], a.Box)
	}
	for row := range out.activate {
		sort.Strings(out.activate[row])
	}
	for row := range out.deactivate {
		sort.Sort(sort.Reverse(sort.StringSlice(out.deactivate[row])))
	}
	return out
}
