package mermaid

import (
	"sort"
	"strconv"
	"strings"
)

// reserved are words Mermaid reads as syntax when they stand alone as an id.
//
// `end` is the one that bites: it closes a subgraph or a fragment, so a box
// called `end` closes something it never opened. The rest are keywords in one
// grammar or another, and a set that covers all three grammars at once is
// simpler to hold than three sets, at the cost of a few ids gaining an
// underscore they did not strictly need.
var reserved = map[string]bool{
	"end": true, "graph": true, "flowchart": true, "subgraph": true, "direction": true,
	"style": true, "classdef": true, "class": true, "click": true, "linkstyle": true,
	"participant": true, "actor": true, "activate": true, "deactivate": true,
	"note": true, "loop": true, "alt": true, "else": true, "opt": true, "par": true,
	"and": true, "critical": true, "option": true, "break": true, "rect": true,
	"create": true, "destroy": true, "box": true, "state": true, "default": true,
}

// ids maps stage-3 ids to ids Mermaid accepts.
//
// Mermaid wants an identifier: letters, digits and underscores, opening with a
// letter or an underscore. Stage-3 ids are authored to be readable and carry
// dots, colons and hyphens, all of which Mermaid reads as syntax somewhere. The
// rewrite has to be total, because a text that is half-quoted works until the
// one id that is not, and it has to be deterministic, because two runs over one
// document must produce one text.
type ids struct {
	safe map[string]string
}

// newIDs assigns every source id a safe one. Sources are taken in sorted order
// so that a collision is resolved the same way every time.
func newIDs(sources []string) ids {
	sorted := append([]string(nil), sources...)
	sort.Strings(sorted)

	out := ids{safe: make(map[string]string, len(sorted))}
	taken := make(map[string]bool, len(sorted))
	claim := func(source string) {
		if _, done := out.safe[source]; done {
			return
		}
		candidate := sanitise(source)
		for n := 2; taken[candidate]; n++ {
			candidate = sanitise(source) + "_" + strconv.Itoa(n)
		}
		taken[candidate] = true
		out.safe[source] = candidate
	}
	// An id that needs no rewriting is taken first, so that it keeps its own
	// name rather than losing it to a neighbour that sorts earlier and happens
	// to sanitise to the same thing. Two passes are what make that so.
	for _, source := range sorted {
		if sanitise(source) == source {
			claim(source)
		}
	}
	for _, source := range sorted {
		claim(source)
	}
	return out
}

// of returns the safe id for source. A source never registered is sanitised on
// the spot rather than refused, so a dangling reference in a document becomes a
// dangling reference in the text, where a reader can see it, rather than a
// panic.
func (i ids) of(source string) string {
	if safe, ok := i.safe[source]; ok {
		return safe
	}
	return sanitise(source)
}

// sanitise is the rewrite itself: every character Mermaid would read as syntax
// becomes an underscore, an opening digit gains a prefix, and a keyword gains a
// suffix. It is not injective, which is why newIDs resolves collisions.
func sanitise(source string) string {
	var b strings.Builder
	for _, r := range source {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "n_" + out
	}
	if reserved[strings.ToLower(out)] {
		out += "_"
	}
	return out
}

// label makes text safe inside a quoted node label, `["..."]`.
//
// A double quote would end the label early. Newlines are folded because a
// label is one line of Mermaid, and the extractors on the other end treat a
// line as a statement.
func label(text string) string {
	text = strings.ReplaceAll(text, `"`, `'`)
	return oneLine(text)
}

// edgeLabel makes text safe between pipes, `-->|...|`. A pipe would end it.
func edgeLabel(text string) string {
	text = strings.ReplaceAll(text, "|", "/")
	return oneLine(text)
}

// statement makes text safe after a colon in a sequence or state statement.
// A semicolon ends a Mermaid statement, so it becomes a comma.
func statement(text string) string {
	text = strings.ReplaceAll(text, ";", ",")
	return oneLine(text)
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
