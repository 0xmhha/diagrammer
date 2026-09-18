package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/artifact"
	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/instruct"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/render"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
	"github.com/0xmhha/diagrammer/internal/vcs"
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

func (r *GraphRequest) Paths() []string { return []string{r.Source, r.Out} }

func (r *GraphRequest) Run(ctx context.Context) (*Result, error) {
	if r.Source == "" {
		return nil, fmt.Errorf("graph needs a source directory")
	}
	registry := analyzers()
	opts := analyze.Options{
		IncludeTests: r.IncludeTests,
		MaxDepth:     r.MaxDepth,
		Exclude:      r.Exclude,
	}

	// Every language this build reads is run over the tree and the results are
	// merged. A tree with none of a language in it costs a walk and contributes
	// nothing, which is cheaper than asking the caller to say what is in there.
	var graphs []*graph.Graph
	for _, language := range registry.Languages() {
		reader, ok := registry.For(language)
		if !ok {
			continue
		}
		one, err := reader.Analyze(ctx, r.Source, opts)
		if err != nil {
			return nil, fmt.Errorf("analyze %s as %s: %w", r.Source, language, err)
		}
		graphs = append(graphs, one)
	}
	g, err := analyze.Merge(graphs...)
	if err != nil {
		return nil, fmt.Errorf("merge the analyzers' graphs: %w", err)
	}

	// Which commit the tree was at is written down here because this is the
	// only stage that reads the tree. Every stage after it repeats what is
	// recorded now, and none of them can go back and look.
	//
	// A tree that is not in a checkout gets no revision and that is the end of
	// it. Failing would make an ordinary directory an error; guessing would put
	// a commit into a document that never described one.
	revision, known, err := vcs.Of(r.Source)
	if err != nil {
		return nil, fmt.Errorf("read the revision of %s: %w", r.Source, err)
	}
	if known {
		g.Revision = &revision
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
	sayRevision(out, g.Revision)
	summariseGraph(out, r.Source, g)
	if err := deliver(out, r.Out, encoded); err != nil {
		return nil, err
	}
	return out, nil
}

// sayRevision reports the commit, or reports that there was none to read.
//
// Silence would be the wrong answer to a tree that is not a checkout. Somebody
// who expected a revision in the document needs to learn here that there is not
// going to be one, rather than by opening the finished page and finding nothing
// where they expected a commit.
func sayRevision(out *Result, revision *vcs.Revision) {
	if revision == nil {
		out.say("revision: none; this tree is not in a git checkout, so the documents will not name a commit")
		return
	}
	if revision.Ref == "" {
		out.say("revision: %s (detached)", revision.Commit)
		return
	}
	out.say("revision: %s (%s)", revision.Commit, revision.Ref)
}

// summariseGraph names the files that would not parse rather than counting
// them. A count says something is wrong; the paths say what to go and open.
//
// How much of a refused file is in the graph depends on which parser refused it,
// and for a long time this said neither. go/ast refuses a file outright and
// nothing of it arrives. tree-sitter recovers: it reports an error and carries
// on, and most of the file usually still comes through. Two files in the
// reference tree this project reads are reported here and both contributed
// every declaration they have; what the grammar could not read was one
// expression inside one function body, and nothing was lost.
//
// A sentence claiming either behaviour for both would be wrong half the time,
// and wrong in the direction that matters: telling somebody their code is
// absent when it is there. So it is no longer claimed. It is counted, per file,
// out of the graph itself, which is the one place that actually knows.
func summariseGraph(out *Result, source string, g *graph.Graph) {
	d := g.Diagnostics
	out.say("%s: %d nodes, %d edges, %d files read", source, len(g.Nodes), len(g.Edges), d.FilesParsed)
	if len(d.ParseFailures) == 0 {
		return
	}
	survived := declarationsByFile(g)
	n := len(d.ParseFailures)
	out.say("%d %s something the parser could not read:", n, plural(n, "file has", "files have"))
	for _, f := range d.ParseFailures {
		where := f.Path
		if f.Line > 0 {
			where = fmt.Sprintf("%s:%d", f.Path, f.Line)
		}
		out.say("  %s: %s", where, f.Message)
		out.say("      %s", survivalOf(survived[f.Path]))
	}
}

// survivalOf says how much of a file reached the graph, which is what separates
// a file that is gone from a file with a hole in it.
func survivalOf(declarations int) string {
	switch declarations {
	case 0:
		return "nothing from this file is in the graph"
	case 1:
		return "1 declaration from this file is in the graph; the rest of it is not"
	default:
		return fmt.Sprintf("%d declarations from this file are in the graph, so what is missing is "+
			"what the message names and not the file", declarations)
	}
}

// declarationsByFile counts the declarations each file contributed.
func declarationsByFile(g *graph.Graph) map[string]int {
	out := map[string]int{}
	for _, n := range g.Nodes {
		if n.Kind != graph.KindFunc && n.Kind != graph.KindType {
			continue
		}
		if n.Source != nil && n.Source.Path != "" {
			out[n.Source.Path]++
		}
	}
	return out
}

// --- the stage-2 boundary -----------------------------------------------------

// ValidateRequest accepts or refuses a returned UML model.
type ValidateRequest struct {
	// Model is the codegraph.json to check.
	Model string `json:"model" jsonschema:"the codegraph.json to check"`
	// Graph is the stage-1 graph the model says it came from. Named, it turns
	// the model's accountsFor claims from prose into arithmetic.
	Graph string `json:"graph,omitempty" jsonschema:"the stage-1 graph the model came from; checks what the model claims to stand for"`
}

func (r *ValidateRequest) Op() Op { return OpValidate }

func (r *ValidateRequest) Paths() []string { return []string{r.Model, r.Graph} }

func (r *ValidateRequest) Run(context.Context) (*Result, error) {
	model, _, err := readModel(r.Model)
	if err != nil {
		return nil, err
	}
	out := &Result{}
	out.say("%s: valid, %d %s (%s)", r.Model, len(model.Families),
		plural(len(model.Families), "family", "families"), joinFamilies(model.Families))

	if r.Graph == "" {
		return out, nil
	}
	g, err := readGraph(r.Graph)
	if err != nil {
		return nil, err
	}
	// An id the graph does not have is a defect and stops the command; areas
	// nobody stood for are reported and do not. A model is a map rather than a
	// census and is allowed to leave things out, but not to leave them out
	// without saying so.
	coverage, err := validate.Against(model, g)
	if err != nil {
		return nil, err
	}
	out.say("%s", coverage)
	// Naming the gaps is only worth anything once something was claimed. A
	// model that stated nothing has every area unaccounted for, and listing
	// them all would read as a long list of failures rather than as one
	// sentence saying the question was never answered.
	if coverage.Stated > 0 {
		for _, area := range coverage.Unclaimed {
			out.say("  no component stands for %s", area)
		}
	}
	return out, nil
}

// readGraph loads a stage-1 graph and holds it to its own schema first, so a
// graph that is not one fails as itself rather than as a coverage result
// nobody can explain.
func readGraph(path string) (*graph.Graph, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the caller named this file
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := schema.Validate(schema.Graph, raw); err != nil {
		return nil, fmt.Errorf("%s does not satisfy the graph schema: %w", path, err)
	}
	var g graph.Graph
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &g, nil
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

func (r *ComposeRequest) Paths() []string { return []string{r.Model, r.Out} }

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
		if err := os.MkdirAll(r.Out, outputDirMode); err != nil {
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

func (r *RenderRequest) Paths() []string { return []string{r.Document, r.Out} }

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
	// Drawn but unnamed. It is not a drop and is not counted as one: the
	// relationship is on the page and its text is not, and a reader deciding
	// whether to trust the drawing needs the difference.
	if n := len(page.Unwritten); n > 0 {
		out.say("%d line(s) are drawn without their text:", n)
		for _, u := range page.Unwritten {
			out.say("  %s on %s: %s", u.Route, u.Level, u.Text)
		}
	}
	if err := deliver(out, r.Out, html); err != nil {
		return nil, err
	}
	return out, nil
}

// --- the stage-2 boundary, from the other side ---------------------------------

// InstructRequest hands out the instruction stage 2 is performed from.
//
// Every other capability reads a document and writes one. This one reads
// nothing: the instruction is built from the schemas the binary already
// carries, so it can be asked for anywhere the binary runs and cannot disagree
// with the gate that will judge what comes back.
type InstructRequest struct {
	// Out names a file to write the instruction to. Left empty, it is handed
	// back instead, which is what a plugin asking over MCP wants.
	Out string `json:"out,omitempty" jsonschema:"a file to write the instruction to; omit it and it is returned instead"`
}

func (r *InstructRequest) Op() Op { return OpInstruct }

func (r *InstructRequest) Paths() []string { return []string{r.Out} }

func (r *InstructRequest) Run(context.Context) (*Result, error) {
	text, err := instruct.Stage2()
	if err != nil {
		return nil, err
	}
	out := &Result{}
	if r.Out == "" {
		out.Files = append(out.Files, OutputFile{Content: []byte(text)})
		return out, nil
	}
	out.say("%s: the stage-2 instruction, %d bytes", r.Out, len(text))
	if err := deliver(out, r.Out, []byte(text)); err != nil {
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
//
// The mode is set on every write, not only when the file is created. That is
// what os.WriteFile does on its own, and it leaves a file somebody made
// world-readable world-readable while this program fills it with the doc
// comments of a private repository. The run that produced the content is the
// one that decides who may read it; widening it is still one chmod, taken after
// that run rather than once and forgotten.
//
// Directories are left alone. An existing output directory keeps whatever mode
// it has, because what it holds is already the owner's alone and re-tightening
// somebody's directory is a larger surprise than it is worth.
func deliver(out *Result, path string, content []byte) error {
	if path != "" {
		if err := os.WriteFile(path, content, outputFileMode); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		if err := os.Chmod(path, outputFileMode); err != nil {
			return fmt.Errorf("set the mode on %s: %w", path, err)
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
