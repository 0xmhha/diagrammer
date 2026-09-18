package mermaid

import (
	"fmt"
	"strings"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

// Accounting says what the text carries against what the document proved.
//
// It is the same shape stage 3 and stage 4 keep, and it is here for the same
// reason: a text that quietly carried less than the document would look
// identical to one that carried all of it. Recorded counts the relationships
// the page could not draw and this text carries anyway, which is the one place
// this output is fuller than the page. Omitted counts the ones this text cannot
// place either, which today is a recorded message in a sequence diagram, where
// order is meaning and a recorded message has none.
type Accounting struct {
	Proven   int
	Carried  int
	Recorded int
	Omitted  int
}

// Output is the text and the account of it.
type Output struct {
	Markdown   []byte
	Levels     int
	Accounting Accounting
}

// Document writes doc as Markdown with one Mermaid block per level.
//
// Every level is written, in the document's order, under a heading that names
// it and what opens it, so a reader moving between blocks has the same map a
// reader of the page has between pages.
func Document(doc *diagram.Document) (*Output, error) {
	var b strings.Builder
	header(&b, doc)

	out := &Output{Levels: len(doc.Levels)}
	for i := range doc.Levels {
		level := &doc.Levels[i]
		acc, err := writeLevel(&b, doc.Family, level)
		if err != nil {
			return nil, fmt.Errorf("level %s: %w", level.ID, err)
		}
		out.Accounting.Proven += acc.Proven
		out.Accounting.Carried += acc.Carried
		out.Accounting.Recorded += acc.Recorded
		out.Accounting.Omitted += acc.Omitted
	}
	out.Markdown = []byte(b.String())
	return out, nil
}

func header(b *strings.Builder, doc *diagram.Document) {
	fmt.Fprintf(b, "# %s\n\n", oneLine(doc.Meta.Title))
	if doc.Meta.Subtitle != "" {
		fmt.Fprintf(b, "*%s*\n\n", oneLine(doc.Meta.Subtitle))
	}
	fmt.Fprintf(b, "%s diagram, %d level(s).", doc.Family, len(doc.Levels))
	if r := doc.Provenance.Revision; r != nil {
		fmt.Fprintf(b, " Drawn from `%s`", r.Commit)
		if r.Ref != "" {
			fmt.Fprintf(b, " on %s", r.Ref)
		}
		b.WriteString(".")
	}
	b.WriteString("\n\n")
}

// writeLevel writes one level as a heading, a line saying where it sits, and
// one fenced block in the grammar its family maps to.
func writeLevel(b *strings.Builder, family diagram.Family, level *diagram.Level) (Accounting, error) {
	fmt.Fprintf(b, "## %s\n\n", oneLine(level.Title))
	where(b, level)

	var (
		body string
		acc  Accounting
		err  error
	)
	switch family {
	case diagram.FamilyComponent:
		body, acc = flowchart(level, "TD", componentShapes)
	case diagram.FamilyUsecase:
		body, acc = flowchart(level, "LR", usecaseShapes)
	case diagram.FamilySequence:
		body, acc = sequence(level)
	case diagram.FamilyState:
		body, acc = state(level)
	default:
		err = fmt.Errorf("the %q family has no Mermaid grammar", family)
	}
	if err != nil {
		return Accounting{}, err
	}

	b.WriteString("```mermaid\n")
	b.WriteString(body)
	b.WriteString("```\n\n")
	return acc, nil
}

// where says what a level is beneath and which of its boxes open a level of
// their own. On the page that is a button; in text it has to be a sentence.
func where(b *strings.Builder, level *diagram.Level) {
	var opens []string
	for _, box := range level.Boxes {
		if box.Opens != "" {
			opens = append(opens, fmt.Sprintf("%s opens `%s`", oneLine(box.Label), box.Opens))
		}
	}
	if level.Parent == "" && len(opens) == 0 {
		return
	}
	if level.Parent != "" {
		fmt.Fprintf(b, "Beneath `%s`. ", level.Parent)
	}
	if len(opens) > 0 {
		b.WriteString(strings.Join(opens, "; "))
		b.WriteString(".")
	}
	b.WriteString("\n\n")
}

// sourcesOf lists every id a level's text may refer to: its boxes and its
// regions, in one pool so a region and a box cannot be given one Mermaid id.
func sourcesOf(level *diagram.Level) []string {
	out := make([]string, 0, len(level.Boxes)+len(level.Regions))
	for _, box := range level.Boxes {
		out = append(out, box.ID)
	}
	for _, region := range level.Regions {
		out = append(out, region.ID)
	}
	return out
}

// recordedOn lists the relationships the page recorded rather than drew, as
// connections, so a grammar that can place them treats them like any other.
//
// A recorded relationship carries where it goes and what kind it is, and not
// the label it had; the page did not keep that either. The account marks them
// so a reader can tell a line the page drew from one only the text has.
func recordedOn(level *diagram.Level) []diagram.Connection {
	var out []diagram.Connection
	for _, box := range level.Boxes {
		for _, d := range box.Dropped {
			out = append(out, diagram.Connection{From: box.ID, To: d.To, Kind: d.Kind})
		}
	}
	return out
}
