package validate_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// Every rule below needs a case that makes it fail, not only cases that pass.
// A check that never fires cannot be told apart from one that was never
// written, and this gate is the only thing standing between an external stage
// and everything downstream of it.

// doc wraps family sections in the envelope every model needs, so each case
// below shows only the part it is about.
func doc(families string, sections ...string) []byte {
	var b strings.Builder
	b.WriteString(`{"schemaVersion":1,"meta":{"title":"t"},`)
	b.WriteString(`"provenance":{"origin":"handwritten"},"families":[` + families + `]`)
	for _, s := range sections {
		b.WriteString("," + s)
	}
	b.WriteString("}")
	return []byte(b.String())
}

const (
	// minimal sections, valid on their own, for cases about something else.
	componentSection = `"component":{"components":[{"id":"a","name":"A"}]}`
	sequenceSection  = `"sequence":{"lifelines":[{"id":"x","name":"X"},{"id":"y","name":"Y"}],` +
		`"messages":[{"id":"m1","from":"x","to":"y","name":"go","kind":"sync"}]}`
	stateSection = `"state":{"states":[{"id":"s","name":"S","kind":"simple"}],` +
		`"transitions":[{"id":"t","from":"s","to":"s"}]}`
	usecaseSection = `"usecase":{"system":{"id":"sys","name":"S"},` +
		`"actors":[{"id":"ac","name":"A"}],"usecases":[{"id":"uc","name":"U"}],` +
		`"associations":[{"id":"as","actor":"ac","usecase":"uc"}]}`
)

func TestCodegraphAccepts(t *testing.T) {
	cases := []struct {
		name string
		doc  []byte
	}{
		{"one family", doc(`"component"`, componentSection)},
		{"every family", doc(`"component","sequence","state","usecase"`,
			componentSection, sequenceSection, stateSection, usecaseSection)},
		{"a dependency may end on an interface", doc(`"component"`,
			`"component":{"components":[{"id":"a","name":"A"}],`+
				`"interfaces":[{"id":"I","name":"I"}],`+
				`"dependencies":[{"id":"d","from":"a","to":"I","kind":"realize"}]}`)},
		{"an activation may start and end on one message", doc(`"sequence"`,
			`"sequence":{"lifelines":[{"id":"x","name":"X"},{"id":"y","name":"Y"}],`+
				`"messages":[{"id":"m1","from":"x","to":"y","name":"go","kind":"sync"}],`+
				`"activations":[{"id":"a","lifeline":"y","start":"m1","end":"m1"}]}`)},
		{"a state may nest in a composite", doc(`"state"`,
			`"state":{"states":[{"id":"c","name":"C","kind":"composite"},`+
				`{"id":"s","name":"S","kind":"simple","parent":"c"}],`+
				`"transitions":[{"id":"t","from":"s","to":"c"}]}`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			model, err := validate.Codegraph("test", c.doc)
			if err != nil {
				t.Fatalf("want accepted, got %v", err)
			}
			if model == nil {
				t.Fatal("accepted but returned no model")
			}
		})
	}
}

func TestCodegraphRefuses(t *testing.T) {
	cases := []struct {
		name string
		doc  []byte
		// want is a substring of the report. It names the rule rather than the
		// whole message, so rewording a diagnostic does not break the test.
		want string
	}{
		// the declaration must agree with the contents
		{
			"family declared with no section",
			doc(`"component","sequence"`, componentSection),
			`"sequence" is declared but the "sequence" section is absent`,
		}, {
			"section present but not declared",
			doc(`"component"`, componentSection, sequenceSection),
			`section is present but "sequence" is not declared`,
		}, {
			"family declared twice",
			doc(`"component","component"`, componentSection),
			"items at 0 and 1 are equal",
		},

		// ids are unique within a family section
		{
			"two components share an id",
			doc(`"component"`, `"component":{"components":[{"id":"a","name":"A"},{"id":"a","name":"B"}]}`),
			`id "a" is already used`,
		}, {
			"a port reuses a component id",
			doc(`"component"`, `"component":{"components":[{"id":"a","name":"A",`+
				`"ports":[{"id":"a","kind":"provided","interface":"I"}]}],`+
				`"interfaces":[{"id":"I","name":"I"}]}`),
			`id "a" is already used`,
		},

		// component references resolve
		{
			"port names an undeclared interface",
			doc(`"component"`, `"component":{"components":[{"id":"a","name":"A",`+
				`"ports":[{"id":"p","kind":"required","interface":"Missing"}]}]}`),
			`port names interface "Missing", which is not declared`,
		}, {
			"dependency end names nothing",
			doc(`"component"`, `"component":{"components":[{"id":"a","name":"A"}],`+
				`"dependencies":[{"id":"d","from":"a","to":"ghost"}]}`),
			`names "ghost", which is neither a component nor an interface`,
		},

		// sequence references resolve, and order means something
		{
			"message names an undeclared lifeline",
			doc(`"sequence"`, `"sequence":{"lifelines":[{"id":"x","name":"X"},{"id":"y","name":"Y"}],`+
				`"messages":[{"id":"m1","from":"x","to":"ghost","name":"go","kind":"sync"}]}`),
			`names lifeline "ghost", which is not declared`,
		}, {
			"activation ends before it starts",
			doc(`"sequence"`, `"sequence":{"lifelines":[{"id":"x","name":"X"},{"id":"y","name":"Y"}],`+
				`"messages":[{"id":"m1","from":"x","to":"y","name":"a","kind":"sync"},`+
				`{"id":"m2","from":"y","to":"x","name":"b","kind":"reply"}],`+
				`"activations":[{"id":"a","lifeline":"y","start":"m2","end":"m1"}]}`),
			"which comes earlier",
		}, {
			"fragment covers an undeclared message",
			doc(`"sequence"`, `"sequence":{"lifelines":[{"id":"x","name":"X"},{"id":"y","name":"Y"}],`+
				`"messages":[{"id":"m1","from":"x","to":"y","name":"go","kind":"sync"}],`+
				`"fragments":[{"id":"f","kind":"opt","operands":[{"messages":["ghost"]}]}]}`),
			`names message "ghost", which is not declared`,
		}, {
			"lifeline represents an undeclared component",
			doc(`"component","sequence"`, componentSection,
				`"sequence":{"lifelines":[{"id":"x","name":"X","represents":"ghost"},{"id":"y","name":"Y"}],`+
					`"messages":[{"id":"m1","from":"x","to":"y","name":"go","kind":"sync"}]}`),
			`names component "ghost", which is not declared`,
		},

		// state containment and transitions resolve
		{
			"transition names an undeclared state",
			doc(`"state"`, `"state":{"states":[{"id":"s","name":"S","kind":"simple"}],`+
				`"transitions":[{"id":"t","from":"s","to":"ghost"}]}`),
			`names state "ghost", which is not declared`,
		}, {
			"parent is not a composite state",
			doc(`"state"`, `"state":{"states":[{"id":"p","name":"P","kind":"simple"},`+
				`{"id":"s","name":"S","kind":"simple","parent":"p"}],`+
				`"transitions":[{"id":"t","from":"s","to":"p"}]}`),
			`which is of kind "simple" rather than composite`,
		}, {
			"states contain each other",
			doc(`"state"`, `"state":{"states":[{"id":"a","name":"A","kind":"composite","parent":"b"},`+
				`{"id":"b","name":"B","kind":"composite","parent":"a"}],`+
				`"transitions":[{"id":"t","from":"a","to":"b"}]}`),
			"containment cycle",
		},

		// use case references resolve
		{
			"association names an undeclared actor",
			doc(`"usecase"`, `"usecase":{"system":{"id":"sys","name":"S"},`+
				`"actors":[{"id":"ac","name":"A"}],"usecases":[{"id":"uc","name":"U"}],`+
				`"associations":[{"id":"as","actor":"ghost","usecase":"uc"}]}`),
			`names actor "ghost", which is not declared`,
		}, {
			"extend names an undeclared use case",
			doc(`"usecase"`, `"usecase":{"system":{"id":"sys","name":"S"},`+
				`"actors":[{"id":"ac","name":"A"}],"usecases":[{"id":"uc","name":"U"}],`+
				`"associations":[{"id":"as","actor":"ac","usecase":"uc"}],`+
				`"extends":[{"id":"ex","from":"uc","to":"ghost"}]}`),
			`names use case "ghost", which is not declared`,
		},

		// shape, settled by the schema rather than here
		{
			"required field missing",
			[]byte(`{"schemaVersion":1,"provenance":{"origin":"handwritten"},"families":["component"]}`),
			"meta",
		}, {
			"unknown field present",
			doc(`"component"`, componentSection, `"surprise":1`),
			"surprise",
		}, {
			"enum value not allowed",
			doc(`"component"`, `"component":{"components":[{"id":"a","name":"A",`+
				`"ports":[{"id":"p","kind":"sideways","interface":"I"}]}],`+
				`"interfaces":[{"id":"I","name":"I"}]}`),
			"must be one of 'provided', 'required'",
		}, {
			"no families declared",
			[]byte(`{"schemaVersion":1,"meta":{"title":"t"},` +
				`"provenance":{"origin":"handwritten"},"families":[]}`),
			"families",
		}, {
			"schema version not recognised",
			[]byte(`{"schemaVersion":99,"meta":{"title":"t"},` +
				`"provenance":{"origin":"handwritten"},"families":["component"],` + componentSection + `}`),
			"schemaVersion",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			model, err := validate.Codegraph("test", c.doc)
			if err == nil {
				t.Fatal("want refused, got accepted")
			}
			if model != nil {
				t.Error("refused but still returned a model")
			}
			var report *validate.Report
			if !errors.As(err, &report) {
				t.Fatalf("want a *Report, got %T: %v", err, err)
			}
			if len(report.Problems) == 0 {
				t.Fatal("refused with an empty report")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("report does not mention %q\ngot:\n%s", c.want, err)
			}
		})
	}
}

// TestCodegraphRefusesNonJSON keeps the non-JSON path distinct: there is nothing
// to enumerate, so it is a plain error rather than a report.
func TestCodegraphRefusesNonJSON(t *testing.T) {
	_, err := validate.Codegraph("test", []byte("not json at all"))
	if err == nil {
		t.Fatal("want refused, got accepted")
	}
	var report *validate.Report
	if errors.As(err, &report) {
		t.Errorf("want a plain error for unparsable input, got a report: %v", err)
	}
}

// TestCommittedFixture holds the repository's own fixture to the contract, and
// asserts it covers every family. A fixture that quietly stopped exercising a
// family would leave that family's rules untested while the suite stayed green.
func TestCommittedFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "codegraph", "order-service.codegraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	model, err := validate.Codegraph(path, raw)
	if err != nil {
		t.Fatalf("fixture does not validate: %v", err)
	}

	declared := make(map[uml.Family]bool, len(model.Families))
	for _, f := range model.Families {
		declared[f] = true
	}
	for _, f := range uml.Families() {
		if !declared[f] {
			t.Errorf("fixture no longer covers the %q family", f)
		}
	}

	// Round 11 of the interview names the vocabulary each family must exercise.
	// These assertions are what stop the fixture from shrinking to a shape that
	// validates without proving anything.
	if got := countPorts(model, uml.PortProvided); got == 0 {
		t.Error("fixture declares no provided port")
	}
	if got := countPorts(model, uml.PortRequired); got == 0 {
		t.Error("fixture declares no required port")
	}
	if len(model.Component.Dependencies) == 0 {
		t.Error("fixture declares no dependency")
	}
	if len(model.Sequence.Activations) == 0 {
		t.Error("fixture declares no activation")
	}
	if len(model.Sequence.Fragments) == 0 {
		t.Error("fixture declares no combined fragment")
	}
	if !hasMessageKind(model, uml.MessageAsync) {
		t.Error("fixture declares no asynchronous message")
	}
	if !hasStateKind(model, uml.StateInitial) || !hasStateKind(model, uml.StateFinal) {
		t.Error("fixture is missing an initial or final state")
	}
	if !hasFullTransition(model) {
		t.Error("fixture declares no transition carrying trigger, guard and effect at once")
	}
	if len(model.Usecase.Includes) == 0 || len(model.Usecase.Extends) == 0 {
		t.Error("fixture is missing an include or an extend")
	}
}

func countPorts(m *uml.Model, kind uml.PortKind) int {
	n := 0
	for _, c := range m.Component.Components {
		for _, p := range c.Ports {
			if p.Kind == kind {
				n++
			}
		}
	}
	return n
}

func hasMessageKind(m *uml.Model, kind uml.MessageKind) bool {
	for _, msg := range m.Sequence.Messages {
		if msg.Kind == kind {
			return true
		}
	}
	return false
}

func hasStateKind(m *uml.Model, kind uml.StateKind) bool {
	for _, s := range m.State.States {
		if s.Kind == kind {
			return true
		}
	}
	return false
}

func hasFullTransition(m *uml.Model) bool {
	for _, t := range m.State.Transitions {
		if t.Trigger != "" && t.Guard != "" && t.Effect != "" {
			return true
		}
	}
	return false
}
