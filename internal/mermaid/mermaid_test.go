package mermaid_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/mermaid"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// The text is a second way out of stage 3, and the thing that would go wrong
// with it goes wrong silently: a relationship left out, an id that happens to
// be a keyword, a label with a quote in it. Each of those produces a file that
// looks fine until somebody feeds it to Mermaid. So the tests here read the
// text back rather than trusting the writer.

func families() []struct {
	Family  diagram.Family
	Fixture string
} {
	return []struct {
		Family  diagram.Family
		Fixture string
	}{
		{diagram.FamilyComponent, "hub"},
		{diagram.FamilyComponent, "hub-overflow"},
		{diagram.FamilyComponent, "nested-platform"},
		{diagram.FamilyComponent, "diagrammer-layered"},
		{diagram.FamilySequence, "diagrammer-layered"},
		{diagram.FamilyState, "diagrammer-layered"},
		{diagram.FamilyUsecase, "diagrammer-layered"},
		{diagram.FamilyComponent, "order-service"},
		{diagram.FamilySequence, "order-service"},
		{diagram.FamilyState, "order-service"},
		{diagram.FamilyUsecase, "order-service"},
		{diagram.FamilyState, "skipping"},
	}
}

func composeFor(t *testing.T, family diagram.Family, fixture string) *diagram.Document {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "codegraph", fixture+".codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("%s does not validate: %v", fixture, err)
	}
	build := map[diagram.Family]func(string, *uml.Model) (*diagram.Document, error){
		diagram.FamilyComponent: compose.Component,
		diagram.FamilySequence:  compose.Sequence,
		diagram.FamilyState:     compose.State,
		diagram.FamilyUsecase:   compose.Usecase,
	}[family]
	doc, err := build(path, model)
	if err != nil {
		t.Fatalf("compose %s: %v", family, err)
	}
	return doc
}

func write(t *testing.T, doc *diagram.Document) (*mermaid.Output, string) {
	t.Helper()
	out, err := mermaid.Document(doc)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	return out, string(out.Markdown)
}

func TestOneBlockPerLevel(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.Family)+"/"+f.Fixture, func(t *testing.T) {
			doc := composeFor(t, f.Family, f.Fixture)
			out, text := write(t, doc)
			if got := strings.Count(text, "```mermaid\n"); got != len(doc.Levels) {
				t.Errorf("%d level(s), %d block(s)", len(doc.Levels), got)
			}
			if out.Levels != len(doc.Levels) {
				t.Errorf("the output says %d level(s), the document has %d", out.Levels, len(doc.Levels))
			}
			if strings.Count(text, "```mermaid\n") != strings.Count(text, "```\n\n") {
				t.Error("a block was opened and not closed")
			}
		})
	}
}

// TestEveryProvenRelationshipIsCarriedOrNamed is the accounting the text keeps,
// in the same shape stage 3 and stage 4 keep theirs. Nothing the document
// proved may vanish between it and the text without being counted.
func TestEveryProvenRelationshipIsCarriedOrNamed(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.Family)+"/"+f.Fixture, func(t *testing.T) {
			doc := composeFor(t, f.Family, f.Fixture)
			out, _ := write(t, doc)
			acc := out.Accounting

			proven := 0
			for _, level := range doc.Levels {
				proven += level.Accounting.Proven
			}
			if acc.Proven != proven {
				t.Errorf("the text counts %d proven, the document %d", acc.Proven, proven)
			}
			if acc.Carried+acc.Omitted != acc.Proven {
				t.Errorf("carried %d + omitted %d != proven %d", acc.Carried, acc.Omitted, acc.Proven)
			}
			// Only a sequence diagram has anything it cannot place, because
			// only there is order the meaning. Every other family lays itself
			// out and carries everything.
			if f.Family != diagram.FamilySequence && acc.Omitted != 0 {
				t.Errorf("a %s text omitted %d relationship(s) it could have carried", f.Family, acc.Omitted)
			}
		})
	}
}

// TestTheTextCarriesWhatThePageRecorded is the reason the export exists beside
// the page. Stage 3 records a relationship its grid cannot place; a flowchart
// lays itself out and can, so the text must.
//
// The fixture is the one that records at stage 3. hub-overflow records more,
// but at stage 4, and what stage 4 records is already in the document as a
// connection: the text carries it without noticing.
func TestTheTextCarriesWhatThePageRecorded(t *testing.T) {
	doc := composeFor(t, diagram.FamilyComponent, "diagrammer-layered")
	dropped := 0
	for _, level := range doc.Levels {
		dropped += level.Accounting.Dropped
	}
	if dropped == 0 {
		t.Fatal("diagrammer-layered no longer records anything at stage 3, so this test is holding nothing")
	}

	out, text := write(t, doc)
	if out.Accounting.Recorded != dropped {
		t.Errorf("the page recorded %d, the text carried %d of them", dropped, out.Accounting.Recorded)
	}
	if !strings.Contains(text, "the page recorded these") {
		t.Error("the recorded relationships are carried without being marked as such")
	}
}

// TestARecordedMessageIsNamedAndNotPlaced covers the one omission the text
// admits to. No fixture records a message, so one is made: order is meaning in
// a sequence diagram and a recorded message has none, so it is named where the
// reader will see it and counted as omitted rather than placed at random.
func TestARecordedMessageIsNamedAndNotPlaced(t *testing.T) {
	doc := composeFor(t, diagram.FamilySequence, "order-service")
	level := &doc.Levels[0]
	level.Boxes[0].Dropped = append(level.Boxes[0].Dropped, diagram.DroppedRelationship{
		To: level.Boxes[1].ID, Kind: diagram.KindMessage, Reason: diagram.ReasonSelfReference,
	})
	level.Accounting.Proven++
	level.Accounting.Dropped++

	out, text := write(t, doc)
	if out.Accounting.Omitted != 1 {
		t.Errorf("want 1 omitted, got %d", out.Accounting.Omitted)
	}
	if !strings.Contains(text, "recorded on the page and not placed here") {
		t.Error("the omitted message is not named in the text")
	}
	if out.Accounting.Carried+out.Accounting.Omitted != out.Accounting.Proven {
		t.Error("the account does not add up")
	}
}

var safeID = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// TestEveryIdIsOneMermaidAccepts reads the ids back out of the text. Stage-3
// ids carry dots and colons, 107 of the 198 in the fixtures at last count, so a
// writer that passed them through would produce a text that fails on the first
// real document.
func TestEveryIdIsOneMermaidAccepts(t *testing.T) {
	// The forms an id takes in each grammar, with the id as the first group.
	forms := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+(\S+?)(?:\["|\(\["|\("|\[\()`),       // flowchart node
		regexp.MustCompile(`(?m)^\s+subgraph (\S+) \[`),                  // subgraph
		regexp.MustCompile(`(?m)^\s+(\S+) (?:-->|---|-\.->)`),            // link source
		regexp.MustCompile(`(?m)(?:-->|---|-\.->)(?:\|[^|]*\|)? (\S+)$`), // link target
		regexp.MustCompile(`(?m)^\s+(?:participant|actor) (\S+) as `),    // lifeline
		regexp.MustCompile(`(?m)^\s+(?:activate|deactivate) (\S+)$`),     // activation
		regexp.MustCompile(`(?m)^\s+state "[^"]*" as (\S+)`),             // state
		regexp.MustCompile(`(?m)^\s+state (\S+) <<`),                     // pseudostate
		regexp.MustCompile(`(?m)^\s+(\S+) --> (\S+)(?: :|$)`),            // transition
	}
	for _, f := range families() {
		t.Run(string(f.Family)+"/"+f.Fixture, func(t *testing.T) {
			_, text := write(t, composeFor(t, f.Family, f.Fixture))
			seen := 0
			for _, form := range forms {
				for _, m := range form.FindAllStringSubmatch(text, -1) {
					for _, id := range m[1:] {
						if id == "" || id == "[*]" {
							continue
						}
						seen++
						if !safeID.MatchString(id) {
							t.Errorf("id %q is not one Mermaid accepts", id)
						}
					}
				}
			}
			if seen == 0 {
				t.Fatal("no ids were read back, so this test is checking nothing")
			}
		})
	}
}

func TestSanitise(t *testing.T) {
	cases := map[string]string{
		"a.ci":         "a_ci",
		"level:pkg0":   "level_pkg0",
		"rel:d.a":      "rel_d_a",
		"bulk.fn01":    "bulk_fn01",
		"already_fine": "already_fine",
		// A keyword would be read as syntax. `end` closes a subgraph.
		"end":  "end_",
		"End":  "End_",
		"loop": "loop_",
		// An id cannot open with a digit.
		"3d": "n_3d",
		"":   "n_",
	}
	for source, want := range cases {
		out, err := mermaid.Document(&diagram.Document{
			Family: diagram.FamilyComponent,
			Levels: []diagram.Level{{ID: "l", Boxes: []diagram.Box{{ID: source, Label: "x"}}}},
		})
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if !strings.Contains(string(out.Markdown), "  "+want+`["x"]`) {
			t.Errorf("%q: want id %q in\n%s", source, want, out.Markdown)
		}
	}
}

// TestTwoIdsThatSanitiseAlikeStayApart holds the rewrite to being injective on
// any one level. `a.b` and `a_b` both want `a_b`; giving both of them to it
// would join two boxes into one and every line to either into a line to both.
func TestTwoIdsThatSanitiseAlikeStayApart(t *testing.T) {
	doc := &diagram.Document{
		Family: diagram.FamilyComponent,
		Levels: []diagram.Level{{ID: "l", Boxes: []diagram.Box{
			{ID: "a_b", Label: "one"}, {ID: "a.b", Label: "two"}, {ID: "a:b", Label: "three"},
		}}},
	}
	_, text := write(t, doc)
	for _, want := range []string{`a_b["one"]`, `a_b_2["two"]`, `a_b_3["three"]`} {
		if !strings.Contains(text, want) {
			t.Errorf("want %s in\n%s", want, text)
		}
	}

	// And the same document with its boxes in another order gives the same
	// ids, because they are taken in sorted order rather than as they come.
	doc.Levels[0].Boxes[0], doc.Levels[0].Boxes[2] = doc.Levels[0].Boxes[2], doc.Levels[0].Boxes[0]
	_, again := write(t, doc)
	for _, want := range []string{`a_b["one"]`, `a_b_2["two"]`, `a_b_3["three"]`} {
		if !strings.Contains(again, want) {
			t.Errorf("reordered: want %s in\n%s", want, again)
		}
	}
}

// TestALabelCannotEndItsOwnStatement covers the characters each grammar reads
// as the end of something: a quote in a node label, a pipe in a link label, a
// semicolon after a colon. Each has to be neutralised rather than escaped,
// because Mermaid's escaping is not something every reader of the text shares.
func TestALabelCannotEndItsOwnStatement(t *testing.T) {
	doc := &diagram.Document{
		Family: diagram.FamilyComponent,
		Levels: []diagram.Level{{ID: "l",
			Boxes:       []diagram.Box{{ID: "a", Label: `say "hi"` + "\nthere"}, {ID: "b", Label: "b"}},
			Connections: []diagram.Connection{{From: "a", To: "b", Kind: diagram.KindDependency, Label: "x | y"}},
		}},
	}
	_, text := write(t, doc)
	if !strings.Contains(text, `a["say 'hi' there"]`) {
		t.Errorf("the quote or the newline survived into a node label:\n%s", text)
	}
	if !strings.Contains(text, `a -->|x / y| b`) {
		t.Errorf("the pipe survived into a link label:\n%s", text)
	}

	seq := &diagram.Document{
		Family: diagram.FamilySequence,
		Levels: []diagram.Level{{ID: "l",
			Boxes:       []diagram.Box{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
			Connections: []diagram.Connection{{From: "a", To: "b", Kind: diagram.KindMessage, Variant: diagram.VariantSync, Label: "one; two", Order: new(int)}},
		}},
	}
	_, text = write(t, seq)
	if !strings.Contains(text, "a->>b: one, two") {
		t.Errorf("the semicolon survived into a message:\n%s", text)
	}
}

// TestSequenceBracketsBalance reads the fragments and activations back. An
// interval opened and not closed is a text that parses up to the last message
// and then does not.
func TestSequenceBracketsBalance(t *testing.T) {
	opener := regexp.MustCompile(`(?m)^\s+(alt|opt|loop|par|critical|break)\b`)
	closer := regexp.MustCompile(`(?m)^\s+end$`)
	activate := regexp.MustCompile(`(?m)^\s+activate `)
	deactivate := regexp.MustCompile(`(?m)^\s+deactivate `)

	for _, fixture := range []string{"order-service", "diagrammer-layered", "diagrammer"} {
		t.Run(fixture, func(t *testing.T) {
			doc := composeFor(t, diagram.FamilySequence, fixture)
			_, text := write(t, doc)
			if o, c := len(opener.FindAllString(text, -1)), len(closer.FindAllString(text, -1)); o != c {
				t.Errorf("%d fragment(s) opened, %d closed", o, c)
			}
			if a, d := len(activate.FindAllString(text, -1)), len(deactivate.FindAllString(text, -1)); a != d {
				t.Errorf("%d activation(s) opened, %d closed", a, d)
			}
			frags := 0
			for _, level := range doc.Levels {
				frags += len(level.Fragments)
			}
			if got := len(opener.FindAllString(text, -1)); got != frags {
				t.Errorf("the document has %d fragment(s), the text opens %d", frags, got)
			}
		})
	}
}

// TestPseudostatesAreSpelledTheWayMermaidSpellsThem holds the one place the
// state grammar differs from the document. An initial or final state is a box
// in the document and a symbol in Mermaid, and declaring it as a state would
// draw a box called "start" beside the dot that already means it.
func TestPseudostatesAreSpelledTheWayMermaidSpellsThem(t *testing.T) {
	doc := composeFor(t, diagram.FamilyState, "order-service")
	_, text := write(t, doc)

	initials, finals := 0, 0
	for _, level := range doc.Levels {
		for _, box := range level.Boxes {
			switch box.Stereotype {
			case "initial":
				initials++
			case "final":
				finals++
			}
			if box.Stereotype == "initial" || box.Stereotype == "final" {
				if strings.Contains(text, `as `+box.ID) || strings.Contains(text, `"`+box.Label+`"`) {
					t.Errorf("pseudostate %s was declared as a state", box.ID)
				}
			}
		}
	}
	if initials == 0 || finals == 0 {
		t.Fatal("order-service has lost its initial or final state, so this test holds nothing")
	}
	if !strings.Contains(text, "[*] --> ") {
		t.Error("no transition leaves the initial pseudostate")
	}
	if !strings.Contains(text, " --> [*]") {
		t.Error("no transition reaches the final pseudostate")
	}
}

// TestACompositeIsABlockAndNotAlsoAState covers the one shape a composite has
// in the document and the one it must not have in the text: it is both a box
// and a region on the page, and declaring both would put an empty state beside
// the block that already is it.
func TestACompositeIsABlockAndNotAlsoAState(t *testing.T) {
	doc := &diagram.Document{
		Family: diagram.FamilyState,
		Levels: []diagram.Level{{ID: "l",
			Boxes: []diagram.Box{
				{ID: "outer", Label: "Outer", Stereotype: "composite"},
				{ID: "inner", Label: "Inner", Stereotype: "simple", Region: "outer"},
				{ID: "go", Label: "start", Stereotype: "initial", Region: "outer"},
			},
			Regions:     []diagram.Region{{ID: "outer", Label: "Outer", Stereotype: "composite"}},
			Connections: []diagram.Connection{{From: "go", To: "inner", Kind: diagram.KindTransition}},
		}},
	}
	_, text := write(t, doc)
	if strings.Count(text, "outer") != 1 {
		t.Errorf("the composite appears %d time(s), want once as a block:\n%s", strings.Count(text, "outer"), text)
	}
	if !strings.Contains(text, "state \"Outer\" as outer {\n    state \"Inner\" as inner\n    [*] --> inner\n  }") {
		t.Errorf("the composite's start is not inside its block:\n%s", text)
	}
}

func TestTheSameDocumentWritesTheSameText(t *testing.T) {
	for _, f := range families() {
		doc := composeFor(t, f.Family, f.Fixture)
		_, one := write(t, doc)
		_, two := write(t, doc)
		if one != two {
			t.Errorf("%s/%s: two writes of one document differ", f.Family, f.Fixture)
		}
	}
}

func TestTheHeaderNamesTheCommit(t *testing.T) {
	doc := composeFor(t, diagram.FamilyComponent, "order-service")
	if doc.Provenance.Revision == nil {
		t.Fatal("order-service no longer carries a revision")
	}
	_, text := write(t, doc)
	if !strings.Contains(text, doc.Provenance.Revision.Commit) {
		t.Error("the text does not say which commit it describes")
	}
}
