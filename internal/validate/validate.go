package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// Problem is one defect in a document.
type Problem struct {
	// Path locates the offending value, as a JSON pointer into the document.
	Path string
	// Message says what is wrong, in lower case and without trailing
	// punctuation, so it reads correctly when wrapped.
	Message string
}

func (p Problem) String() string {
	if p.Path == "" {
		return p.Message
	}
	return p.Path + ": " + p.Message
}

// Report lists every defect found in one document.
//
// It is an error, so a caller may simply propagate it, and it carries the whole
// list because a person fixing a model wants all of it at once rather than one
// defect per run.
type Report struct {
	// Source names the document, for the message. It is usually a file path.
	Source   string
	Problems []Problem
}

func (r *Report) Error() string {
	var b strings.Builder
	source := r.Source
	if source == "" {
		source = "document"
	}
	fmt.Fprintf(&b, "%s: %d problem", source, len(r.Problems))
	if len(r.Problems) != 1 {
		b.WriteByte('s')
	}
	for _, p := range r.Problems {
		b.WriteString("\n  ")
		b.WriteString(p.String())
	}
	return b.String()
}

func (r *Report) add(path, format string, args ...any) {
	r.Problems = append(r.Problems, Problem{Path: path, Message: fmt.Sprintf(format, args...)})
}

// Codegraph checks a stage-2 UML model.
//
// It returns the parsed model when the document is acceptable. Otherwise it
// returns a *Report naming every defect; callers that need the list can reach
// it with errors.As. A document that is not JSON at all fails with a plain
// error, because there is nothing to enumerate.
func Codegraph(source string, doc []byte) (*uml.Model, error) {
	report := &Report{Source: source}

	if err := schema.Validate(schema.Codegraph, doc); err != nil {
		var invalid *jsonschema.ValidationError
		if !errors.As(err, &invalid) {
			return nil, err
		}
		collectSchemaProblems(invalid, report)
		// Shape is wrong, so the semantic checks below would report noise
		// derived from fields that may not be there. Stop at the first half.
		return nil, report
	}

	var model uml.Model
	if err := json.Unmarshal(doc, &model); err != nil {
		return nil, fmt.Errorf("parse document: %w", err)
	}

	checkFamilies(&model, report)
	if model.Component != nil {
		checkComponent(model.Component, report)
	}
	if model.Sequence != nil {
		checkSequence(&model, report)
	}
	if model.State != nil {
		checkState(model.State, report)
	}
	if model.Usecase != nil {
		checkUsecase(model.Usecase, report)
	}

	if len(report.Problems) > 0 {
		return nil, report
	}
	return &model, nil
}

// --- families ----------------------------------------------------------------

// checkFamilies holds the declaration and the contents to each other. Either
// direction being wrong is a defect: a family declared but absent would make
// compose fail late, and a family present but undeclared would let a diagram be
// produced that the model never promised.
func checkFamilies(m *uml.Model, r *Report) {
	present := map[uml.Family]bool{
		uml.FamilyComponent: m.Component != nil,
		uml.FamilySequence:  m.Sequence != nil,
		uml.FamilyState:     m.State != nil,
		uml.FamilyUsecase:   m.Usecase != nil,
	}
	// A repeated entry is already refused by the schema's uniqueItems, so there
	// is nothing to check for here. Duplicating the rule would give it two
	// owners and let them disagree.
	declared := make(map[uml.Family]bool, len(m.Families))
	for _, f := range m.Families {
		declared[f] = true
	}
	for _, f := range uml.Families() {
		switch {
		case declared[f] && !present[f]:
			r.add("/families", "family %q is declared but the %q section is absent", f, f)
		case present[f] && !declared[f]:
			r.add("/"+string(f), "section is present but %q is not declared in families", f)
		}
	}
}

// --- component ---------------------------------------------------------------

func checkComponent(m *uml.ComponentModel, r *Report) {
	ids := newIDSet(r)
	components := make(map[string]bool, len(m.Components))
	interfaces := make(map[string]bool, len(m.Interfaces))

	for i, c := range m.Components {
		at := fmt.Sprintf("/component/components/%d", i)
		ids.add(at, c.ID)
		components[c.ID] = true
		for j, p := range c.Ports {
			ids.add(fmt.Sprintf("%s/ports/%d", at, j), p.ID)
		}
	}
	for i, iface := range m.Interfaces {
		at := fmt.Sprintf("/component/interfaces/%d", i)
		ids.add(at, iface.ID)
		interfaces[iface.ID] = true
	}
	for i, d := range m.Dependencies {
		ids.add(fmt.Sprintf("/component/dependencies/%d", i), d.ID)
	}

	for i, c := range m.Components {
		for j, p := range c.Ports {
			at := fmt.Sprintf("/component/components/%d/ports/%d/interface", i, j)
			if !interfaces[p.Interface] {
				r.add(at, "port names interface %q, which is not declared", p.Interface)
			}
		}
	}
	// Nesting is what stage 3 turns into levels, so a parent that names nothing
	// would put a whole subtree on no page at all.
	parents := make(map[string]string, len(m.Components))
	for i, c := range m.Components {
		if c.Parent == "" {
			continue
		}
		at := fmt.Sprintf("/component/components/%d/parent", i)
		switch {
		case !components[c.Parent]:
			r.add(at, "names component %q, which is not declared", c.Parent)
		case c.Parent == c.ID:
			r.add(at, "names the component itself")
		default:
			parents[c.ID] = c.Parent
		}
	}
	for _, id := range sortedKeys(parents) {
		if path, looped := parentCycle(parents, id); looped {
			r.add("/component/components", "components %s form a containment cycle", strings.Join(path, " -> "))
			break // one report is enough; every member would produce the same cycle
		}
	}

	// A dependency may run between components, between interfaces, or across
	// the two, so both ends are resolved against the union rather than against
	// one kind.
	for i, d := range m.Dependencies {
		at := fmt.Sprintf("/component/dependencies/%d", i)
		if !components[d.From] && !interfaces[d.From] {
			r.add(at+"/from", "names %q, which is neither a component nor an interface", d.From)
		}
		if !components[d.To] && !interfaces[d.To] {
			r.add(at+"/to", "names %q, which is neither a component nor an interface", d.To)
		}
	}
}

// --- sequence ----------------------------------------------------------------

func checkSequence(model *uml.Model, r *Report) {
	m := model.Sequence
	ids := newIDSet(r)
	lifelines := make(map[string]bool, len(m.Lifelines))
	// order records each message's position, which is what makes "start comes
	// before end" a question with an answer. Message order is the order of the
	// array; there is no separate field to consult.
	order := make(map[string]int, len(m.Messages))

	for i, l := range m.Lifelines {
		ids.add(fmt.Sprintf("/sequence/lifelines/%d", i), l.ID)
		lifelines[l.ID] = true
	}
	for i, msg := range m.Messages {
		ids.add(fmt.Sprintf("/sequence/messages/%d", i), msg.ID)
		order[msg.ID] = i
	}
	for i, a := range m.Activations {
		ids.add(fmt.Sprintf("/sequence/activations/%d", i), a.ID)
	}
	for i, f := range m.Fragments {
		ids.add(fmt.Sprintf("/sequence/fragments/%d", i), f.ID)
	}

	for i, msg := range m.Messages {
		at := fmt.Sprintf("/sequence/messages/%d", i)
		if !lifelines[msg.From] {
			r.add(at+"/from", "names lifeline %q, which is not declared", msg.From)
		}
		if !lifelines[msg.To] {
			r.add(at+"/to", "names lifeline %q, which is not declared", msg.To)
		}
	}
	for i, a := range m.Activations {
		at := fmt.Sprintf("/sequence/activations/%d", i)
		if !lifelines[a.Lifeline] {
			r.add(at+"/lifeline", "names lifeline %q, which is not declared", a.Lifeline)
		}
		start, startOK := order[a.Start]
		end, endOK := order[a.End]
		if !startOK {
			r.add(at+"/start", "names message %q, which is not declared", a.Start)
		}
		if !endOK {
			r.add(at+"/end", "names message %q, which is not declared", a.End)
		}
		if startOK && endOK && start > end {
			r.add(at, "starts at message %q and ends at %q, which comes earlier", a.Start, a.End)
		}
	}
	for i, f := range m.Fragments {
		for j, op := range f.Operands {
			for k, id := range op.Messages {
				if _, ok := order[id]; !ok {
					at := fmt.Sprintf("/sequence/fragments/%d/operands/%d/messages/%d", i, j, k)
					r.add(at, "names message %q, which is not declared", id)
				}
			}
		}
	}
	// represents is only checkable when the component family is there to check
	// against; absent it, the field is a note rather than a reference.
	if model.Component != nil {
		components := make(map[string]bool, len(model.Component.Components))
		for _, c := range model.Component.Components {
			components[c.ID] = true
		}
		for i, l := range m.Lifelines {
			if l.Represents != "" && !components[l.Represents] {
				at := fmt.Sprintf("/sequence/lifelines/%d/represents", i)
				r.add(at, "names component %q, which is not declared", l.Represents)
			}
		}
	}
}

// --- state -------------------------------------------------------------------

func checkState(m *uml.StateModel, r *Report) {
	ids := newIDSet(r)
	kinds := make(map[string]uml.StateKind, len(m.States))

	for i, s := range m.States {
		ids.add(fmt.Sprintf("/state/states/%d", i), s.ID)
		kinds[s.ID] = s.Kind
	}
	for i, t := range m.Transitions {
		ids.add(fmt.Sprintf("/state/transitions/%d", i), t.ID)
	}

	parents := make(map[string]string, len(m.States))
	for i, s := range m.States {
		if s.Parent == "" {
			continue
		}
		at := fmt.Sprintf("/state/states/%d/parent", i)
		kind, ok := kinds[s.Parent]
		switch {
		case !ok:
			r.add(at, "names state %q, which is not declared", s.Parent)
		case kind != uml.StateComposite:
			r.add(at, "names state %q, which is of kind %q rather than composite", s.Parent, kind)
		default:
			parents[s.ID] = s.Parent
		}
	}
	for _, id := range sortedKeys(parents) {
		if path, looped := parentCycle(parents, id); looped {
			r.add("/state/states", "states %s form a containment cycle", strings.Join(path, " -> "))
			break // one report is enough; every member would produce the same cycle
		}
	}

	for i, t := range m.Transitions {
		at := fmt.Sprintf("/state/transitions/%d", i)
		if _, ok := kinds[t.From]; !ok {
			r.add(at+"/from", "names state %q, which is not declared", t.From)
		}
		if _, ok := kinds[t.To]; !ok {
			r.add(at+"/to", "names state %q, which is not declared", t.To)
		}
	}
}

// parentCycle walks up from start and reports the loop it lands in, if any.
func parentCycle(parents map[string]string, start string) ([]string, bool) {
	seen := map[string]bool{start: true}
	path := []string{start}
	for cur := start; ; {
		next, ok := parents[cur]
		if !ok {
			return nil, false
		}
		path = append(path, next)
		if seen[next] {
			return path, true
		}
		seen[next] = true
		cur = next
	}
}

// --- use case ----------------------------------------------------------------

func checkUsecase(m *uml.UsecaseModel, r *Report) {
	ids := newIDSet(r)
	actors := make(map[string]bool, len(m.Actors))
	usecases := make(map[string]bool, len(m.Usecases))

	ids.add("/usecase/system", m.System.ID)
	for i, a := range m.Actors {
		ids.add(fmt.Sprintf("/usecase/actors/%d", i), a.ID)
		actors[a.ID] = true
	}
	for i, u := range m.Usecases {
		ids.add(fmt.Sprintf("/usecase/usecases/%d", i), u.ID)
		usecases[u.ID] = true
	}
	for i, a := range m.Associations {
		ids.add(fmt.Sprintf("/usecase/associations/%d", i), a.ID)
	}
	for i, inc := range m.Includes {
		ids.add(fmt.Sprintf("/usecase/includes/%d", i), inc.ID)
	}
	for i, ext := range m.Extends {
		ids.add(fmt.Sprintf("/usecase/extends/%d", i), ext.ID)
	}

	for i, a := range m.Associations {
		at := fmt.Sprintf("/usecase/associations/%d", i)
		if !actors[a.Actor] {
			r.add(at+"/actor", "names actor %q, which is not declared", a.Actor)
		}
		if !usecases[a.Usecase] {
			r.add(at+"/usecase", "names use case %q, which is not declared", a.Usecase)
		}
	}
	for i, inc := range m.Includes {
		checkUsecaseEnds(r, fmt.Sprintf("/usecase/includes/%d", i), usecases, inc.From, inc.To)
	}
	for i, ext := range m.Extends {
		checkUsecaseEnds(r, fmt.Sprintf("/usecase/extends/%d", i), usecases, ext.From, ext.To)
	}
}

func checkUsecaseEnds(r *Report, at string, usecases map[string]bool, from, to string) {
	if !usecases[from] {
		r.add(at+"/from", "names use case %q, which is not declared", from)
	}
	if !usecases[to] {
		r.add(at+"/to", "names use case %q, which is not declared", to)
	}
}

// --- shared ------------------------------------------------------------------

// idSet reports an id used twice. Ids are unique within a family section rather
// than within a kind, so a dependency naming either a component or an interface
// has exactly one thing it can mean.
type idSet struct {
	report *Report
	seen   map[string]string // id -> path where it was first used
}

func newIDSet(r *Report) *idSet {
	return &idSet{report: r, seen: make(map[string]string)}
}

func (s *idSet) add(path, id string) {
	if first, ok := s.seen[id]; ok {
		s.report.add(path, "id %q is already used at %s", id, first)
		return
	}
	s.seen[id] = path
}

// collectSchemaProblems flattens a validation failure into one problem per
// offending value.
//
// The library nests its causes and only the leaves name a value a person can go
// and fix, so the walk goes all the way down. The flat BasicOutput rendering is
// not used: it reports an intermediate unit as "validation failed" and loses the
// leaf's message, which is the only part worth reading.
func collectSchemaProblems(err *jsonschema.ValidationError, r *Report) {
	before := len(r.Problems)
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			r.add(pointer(e.InstanceLocation), "%s", e.ErrorKind.LocalizedString(englishPrinter()))
			return
		}
		for _, cause := range e.Causes {
			walk(cause)
		}
	}
	walk(err)
	if len(r.Problems) == before {
		// Never let a rejected document produce an empty report: a refusal with
		// no reason is worse than a clumsy one.
		r.add("", "%s", err.Error())
	}
}

// englishPrinter renders the library's diagnostics. The locale is fixed rather
// than taken from the environment, so a report reads the same wherever it is
// produced and a fixture expectation does not depend on the machine.
var englishPrinter = sync.OnceValue(func() *message.Printer {
	return message.NewPrinter(language.English)
})

func pointer(location []string) string {
	if len(location) == 0 {
		return ""
	}
	return "/" + strings.Join(location, "/")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
