package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/analyze/goast"
	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/render"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// --- stage 1 ------------------------------------------------------------------

// GraphRequest parses a source tree into a code graph.
type GraphRequest struct {
	// Source is the directory to read.
	Source string `json:"source" jsonschema:"the directory to read"`
	// Out names a file to write the graph to. Left empty, the graph is handed
	// back instead.
	Out string `json:"out,omitempty" jsonschema:"a file to write the graph to; omit it and the graph is returned instead"`
	// IncludeTests reads _test.go files too, which are skipped by default: test
	// code describes how a package is exercised rather than what it is.
	IncludeTests bool `json:"includeTests,omitempty" jsonschema:"read _test.go files too; they are skipped by default"`
	// MaxDepth limits how far below the source to descend. Zero is unlimited.
	MaxDepth int `json:"maxDepth,omitempty" jsonschema:"how far below the source to descend; zero is unlimited"`
	// Exclude lists source-relative directories to skip, such as docs.
	Exclude []string `json:"exclude,omitempty" jsonschema:"source-relative directories to skip"`
}

func (r *GraphRequest) Op() Op { return OpGraph }

// analyzers is what this build can read.
//
// One language today. It is assembled rather than registered globally, so the
// set depends on this line rather than on which packages happened to be linked,
// and a caller can be told what it is instead of discovering it by pointing the
// program at a repository it returns almost nothing for.
func analyzers() *analyze.Registry {
	return analyze.NewRegistry(goast.Analyzer{})
}

func (r *GraphRequest) Run(ctx context.Context) (*Result, error) {
	if r.Source == "" {
		return nil, fmt.Errorf("graph needs a source directory")
	}
	registry := analyzers()
	reader, ok := registry.For(graph.Go)
	if !ok {
		return nil, fmt.Errorf("this build has no analyzer for Go")
	}
	g, err := reader.Analyze(ctx, r.Source, analyze.Options{
		IncludeTests: r.IncludeTests,
		MaxDepth:     r.MaxDepth,
		Exclude:      r.Exclude,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze %s: %w", r.Source, err)
	}

	encoded, err := encode(g)
	if err != nil {
		return nil, err
	}
	// The graph is checked against its own schema before it leaves. An analyzer
	// that drifts from the contract should fail here, where the bug is, rather
	// than at the plugin that reads the file.
	if err := schema.Validate(schema.Graph, encoded); err != nil {
		return nil, fmt.Errorf("the graph does not satisfy the graph schema, which is a defect in the analyzer rather than in the source: %w", err)
	}

	out := &Result{}
	// What was read is said rather than left to be inferred. A graph of a
	// mixed repository that mentions only Go is otherwise indistinguishable
	// from a graph of a repository that only had Go in it.
	out.say("languages read: %s", joinLanguages(registry.Languages()))
	summariseGraph(out, r.Source, g)
	if err := deliver(out, r.Out, encoded); err != nil {
		return nil, err
	}
	return out, nil
}

// summariseGraph names the files that went missing rather than counting them. A
// count says something is gone; the paths say what to go and open.
func summariseGraph(out *Result, source string, g *graph.Graph) {
	d := g.Diagnostics
	out.say("%s: %d nodes, %d edges, %d files read", source, len(g.Nodes), len(g.Edges), d.FilesParsed)
	if len(d.ParseFailures) == 0 {
		return
	}
	out.say("%d file(s) did not parse and are missing from the graph:", len(d.ParseFailures))
	for _, f := range d.ParseFailures {
		if f.Line > 0 {
			out.say("  %s:%d: %s", f.Path, f.Line, f.Message)
			continue
		}
		out.say("  %s: %s", f.Path, f.Message)
	}
}

// --- the stage-2 boundary -----------------------------------------------------

// ValidateRequest accepts or refuses a returned UML model.
type ValidateRequest struct {
	// Model is the codegraph.json to check.
	Model string `json:"model" jsonschema:"the codegraph.json to check"`
}

func (r *ValidateRequest) Op() Op { return OpValidate }

func (r *ValidateRequest) Run(context.Context) (*Result, error) {
	model, _, err := readModel(r.Model)
	if err != nil {
		return nil, err
	}
	out := &Result{}
	out.say("%s: valid, %d %s (%s)", r.Model, len(model.Families),
		plural(len(model.Families), "family", "families"), joinFamilies(model.Families))
	return out, nil
}

// --- stage 3 ------------------------------------------------------------------

// ComposeRequest turns a UML model into diagram sources.
type ComposeRequest struct {
	// Model is the codegraph.json to compose.
	Model string `json:"model" jsonschema:"the codegraph.json to compose"`
	// Out names a directory to write the documents into. Left empty, they are
	// handed back instead.
	Out string `json:"out,omitempty" jsonschema:"a directory to write the documents into; omit it and they are returned instead"`
	// Family narrows the output to one of the families the model declares.
	// Left empty, every declared family is composed.
	Family string `json:"family,omitempty" jsonschema:"one of the families the model declares; omit it to compose every one"`
}

func (r *ComposeRequest) Op() Op { return OpCompose }

func (r *ComposeRequest) Run(context.Context) (*Result, error) {
	model, _, err := readModel(r.Model)
	if err != nil {
		return nil, err
	}
	out := &Result{}
	// The stage-2 boundary has exactly one gate, and reading the model ran it.
	// Saying so beats running it quietly: nobody should be left thinking the
	// model reached here unchecked.
	out.say("%s: validated, %d declared families", r.Model, len(model.Families))

	wanted, err := familiesToCompose(model, r.Family)
	if err != nil {
		return nil, err
	}
	if r.Out != "" {
		if err := os.MkdirAll(r.Out, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", r.Out, err)
		}
	}

	for _, f := range wanted {
		build, ok := composers()[f]
		if !ok {
			return nil, fmt.Errorf("the model declares the %q family, but its composer is not built yet; name a family that is", f)
		}
		check, ok := checkers()[f]
		if !ok {
			return nil, fmt.Errorf("the %q family has a composer but no completeness rule, so its output would go unchecked", f)
		}

		doc, err := build(r.Model, model)
		if err != nil {
			return nil, fmt.Errorf("compose %s: %w", f, err)
		}
		// The family's rule is checked over what was actually built. A rule
		// enforced only inside the composer proves nothing about the document
		// that leaves it.
		if problems := check(model, doc); len(problems) > 0 {
			return nil, fmt.Errorf("the %s document breaks its own completeness rule, which is a defect in the composer:\n  %s",
				f, joinProblems(problems))
		}
		encoded, err := encode(doc)
		if err != nil {
			return nil, err
		}
		if err := schema.Validate(schema.Diagram, encoded); err != nil {
			return nil, fmt.Errorf("the %s document does not satisfy the diagram schema, which is a defect in the composer rather than in the model: %w", f, err)
		}

		target := ""
		if r.Out != "" {
			target = filepath.Join(r.Out, string(f)+".diagram.json")
		}
		a := doc.Accounting
		name := target
		if name == "" {
			name = string(f)
		}
		out.say("%s: %d level(s), %d proven, %d drawn, %d recorded",
			name, len(doc.Levels), a.Proven, a.Drawn, a.Dropped)
		if err := deliver(out, target, encoded); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func composers() map[uml.Family]func(string, *uml.Model) (*diagram.Document, error) {
	return map[uml.Family]func(string, *uml.Model) (*diagram.Document, error){
		uml.FamilyComponent: compose.Component,
		uml.FamilySequence:  compose.Sequence,
		uml.FamilyState:     compose.State,
		uml.FamilyUsecase:   compose.Usecase,
	}
}

// checkers is kept beside composers deliberately: a family gains an entry in
// both at once, so a composer cannot be added without the rule that holds it to
// something.
func checkers() map[uml.Family]func(*uml.Model, *diagram.Document) []invariant.Problem {
	return map[uml.Family]func(*uml.Model, *diagram.Document) []invariant.Problem{
		uml.FamilyComponent: invariant.Component,
		uml.FamilySequence:  invariant.Sequence,
		uml.FamilyState:     invariant.State,
		uml.FamilyUsecase:   invariant.Usecase,
	}
}

// familiesToCompose resolves what to emit, honouring what the model declares.
//
// What a UML model can express is a property of its contents rather than of the
// caller's request, so asking for a family it does not declare is refused.
// Drawing one anyway would produce a diagram nobody could trust.
func familiesToCompose(model *uml.Model, requested string) ([]uml.Family, error) {
	if requested == "" {
		return model.Families, nil
	}
	for _, f := range model.Families {
		if string(f) == requested {
			return []uml.Family{f}, nil
		}
	}
	return nil, fmt.Errorf("the model does not declare the %q family; it declares %s", requested, joinFamilies(model.Families))
}

// --- stage 4 ------------------------------------------------------------------

// RenderRequest turns a diagram source into a self-contained page.
type RenderRequest struct {
	// Document is the diagram source to render.
	Document string `json:"document" jsonschema:"the diagram source to render"`
	// Out names the HTML file to write.
	Out string `json:"out,omitempty" jsonschema:"the HTML file to write"`
}

func (r *RenderRequest) Op() Op { return OpRender }

func (r *RenderRequest) Run(context.Context) (*Result, error) {
	if r.Document == "" {
		return nil, fmt.Errorf("render needs a diagram source path")
	}
	raw, err := os.ReadFile(r.Document)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", r.Document, err)
	}
	// The document is checked before it is drawn. compose emits documents that
	// satisfy the schema, but this command may be handed one from anywhere, and
	// the geometry would fail in stranger ways than a refusal.
	if err := schema.Validate(schema.Diagram, raw); err != nil {
		return nil, fmt.Errorf("%s does not satisfy the diagram schema: %w", r.Document, err)
	}
	var doc diagram.Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", r.Document, err)
	}

	page, err := render.Build(&doc)
	if err != nil {
		return nil, err
	}
	html := []byte(page.HTML())

	// The composition rules are re-run over the emitted document rather than
	// over what the renderer held in memory. Drawing and then checking the
	// drawing is what makes the rules mean something; a renderer that only
	// checked its own working state would prove that it agrees with itself.
	scenes, err := artifact.Parse(html)
	if err != nil {
		return nil, fmt.Errorf("read back the artifact: %w", err)
	}
	for i := range scenes {
		if problems := invariant.CompositionFor(&scenes[i]); len(problems) > 0 {
			return nil, fmt.Errorf("the drawing of %s breaks composition rules that the renderer thought it had satisfied, which is a defect in the renderer:\n  %s",
				scenes[i].Level, joinRouteProblems(problems))
		}
	}

	out := &Result{}
	a := page.Accounting
	out.say("%s: %d page(s), %d proven, %d drawn, %d recorded",
		r.Document, len(page.Scenes), a.Proven, a.Drawn, a.Dropped)
	for _, d := range page.Dropped {
		out.say("  %s on %s: %s", d.Route, d.Level, d.Why)
	}
	if err := deliver(out, r.Out, html); err != nil {
		return nil, err
	}
	return out, nil
}

func joinRouteProblems(problems []invariant.RouteProblem) string {
	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = p.String()
	}
	return joinLines(lines, "\n  ")
}

// --- shared -------------------------------------------------------------------

// readModel reads a codegraph and runs the one gate at the stage-2 boundary.
func readModel(path string) (*uml.Model, []byte, error) {
	if path == "" {
		return nil, nil, fmt.Errorf("a codegraph.json path is required")
	}
	doc, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	model, err := validate.Codegraph(path, doc)
	if err != nil {
		return nil, nil, err
	}
	return model, doc, nil
}

// encode renders a document deterministically.
//
// Byte-identical output across runs is part of the contract, so nothing here
// may depend on map ordering: the types that cross a stage boundary hold no
// maps, and every slice arrives sorted.
func encode(v any) ([]byte, error) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return append(encoded, '\n'), nil
}

// deliver writes content to path, or hands it back when no path was named.
func deliver(out *Result, path string, content []byte) error {
	if path != "" {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	out.Files = append(out.Files, OutputFile{Path: path, Content: content})
	return nil
}

func joinProblems(problems []invariant.Problem) string {
	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = p.String()
	}
	return joinLines(lines, "\n  ")
}

func joinLines(lines []string, sep string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += sep
		}
		out += l
	}
	return out
}

func joinLanguages(languages []graph.Language) string {
	names := make([]string, len(languages))
	for i, l := range languages {
		names[i] = string(l)
	}
	return joinLines(names, ", ")
}

func joinFamilies(families []uml.Family) string {
	names := make([]string, len(families))
	for i, f := range families {
		names[i] = string(f)
	}
	return joinLines(names, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
