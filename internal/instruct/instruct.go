package instruct

import (
	"fmt"
	"strings"

	"github.com/0xmhha/diagrammer/internal/schema"
)

// Numbers stage 2 has to know because stage 3 acts on them.
//
// They are duplicated from internal/compose deliberately: a model is told them
// as prose and the composer reads them as constants, and the two cannot be the
// same declaration without the instruction importing the composer. A test in
// internal/docscheck reads both and fails when they part company, which is the
// same guard the documents are held by.
const (
	minBoxesPerLevel = 6
	expandedMaxBoxes = 24
)

// Stage2 returns the instruction a skill performs stage 2 from.
//
// The schemas are carried verbatim rather than described. A description is a
// second statement of the contract that can disagree with the first, and the
// gate checks the document against the schema, not against the prose.
func Stage2() (string, error) {
	graph, err := schema.Raw(schema.Graph)
	if err != nil {
		return "", err
	}
	model, err := schema.Raw(schema.Codegraph)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(opening)
	b.WriteString("\n## What you are given\n\n")
	b.WriteString(inputProse)
	b.WriteString("\nThis is the schema it validates against, as committed:\n\n")
	fenced(&b, schema.Filename(schema.Graph), graph)
	b.WriteString("\n## What you must return\n\n")
	b.WriteString(outputProse)
	b.WriteString("\n")
	fenced(&b, schema.Filename(schema.Codegraph), model)
	b.WriteString("\n## What the gate checks that the schema cannot\n\n")
	b.WriteString(gateProse)
	b.WriteString("\n## Choosing what to say\n\n")
	fmt.Fprintf(&b, judgementProse, minBoxesPerLevel, expandedMaxBoxes)
	b.WriteString("\n## What happens to what you return\n\n")
	b.WriteString(closingProse)
	return b.String(), nil
}

// fenced writes one schema in a fenced block, labelled with its filename so a
// reader can tell the two apart at a glance.
func fenced(b *strings.Builder, name string, content []byte) {
	fmt.Fprintf(b, "`%s`:\n\n```json\n%s\n```\n", name, strings.TrimRight(string(content), "\n"))
}

const opening = `# Stage 2: read a code graph, return a UML model

You are the second of four stages. The first parsed a source tree into a code
graph. You read that graph and return a UML model. The third turns your model
into diagram sources and the fourth draws them.

The program that runs the other three never calls a model, which is why this
instruction exists: what you return is checked against a schema it embeds, and
this is that schema.

Return one JSON document and nothing else. No prose around it, no fenced block,
no explanation of your reasoning. The next thing that touches your answer is a
parser.
`

const inputProse = `A stage-1 code graph. It is a flat list of nodes with parent pointers and a
flat list of edges, plus what the parse could not do.

Read three things from it in particular. ` + "`nodes`" + ` carries a kind on every entry,
and the kinds form the containment you will be deciding what to do with.
` + "`edges`" + ` carries the relationships that were proven rather than guessed: an
import is a fact, a resolved call is a fact. ` + "`diagnostics`" + ` says what could not be
read, and a graph that failed to parse half its files is not a graph you should
describe with confidence.

Nothing in the graph is an opinion. Every count, name and path in it was
observed. Anything you say that the graph does not support is something you
invented, and a reader cannot tell the two apart once it is drawn.
`

const outputProse = `A UML model. Not a drawing: no coordinates, no sizes, no colours. Those are the
next stage's business and it will refuse a document that tries to make them its
own.

Declare in ` + "`families`" + ` only what you actually carry. A model claiming ` + "`sequence`" + `
without lifelines is refused at the boundary rather than three stages later.
`

const gateProse = `The schema settles shape: which fields exist, their types, which are required,
and that nothing unexpected is present. It cannot settle meaning, so the gate
checks that too, and refuses the document if any of it fails.

- **families** — every family declared has its section, and every section
  present is declared.
- **component** — ids are unique; a port names an interface that exists; a
  parent names a component that exists and is not the component itself;
  containment has no cycle; a dependency's two ends name a component or an
  interface.
- **sequence** — ids are unique; a message names lifelines that exist; an
  activation names a lifeline and two messages that exist, and does not end
  before it starts; a fragment names messages that exist; a lifeline that
  represents a component names one that exists.
- **state** — ids are unique; a parent names a state that exists and is of kind
  composite; containment has no cycle; a transition's two ends name states that
  exist.
- **usecase** — ids are unique; an association names an actor and a use case
  that exist; an include or an extend names use cases that exist.

Every failure is reported with the JSON pointer of the value that caused it, so
a refusal tells you where to look rather than that something is wrong.
`

const judgementProse = `Everything above is mechanical. This part is not, and it is the reason a model
does this stage rather than a converter.

**Four families, and you are asked for every one the graph can carry.** The
rule above is about not over-claiming; this one is about not stopping early. A
model that returns only a component diagram has answered a quarter of the
question when the graph would have carried more, and nothing downstream can tell
that from a graph that really had only that much in it.

- **component** is always available. Packages, files and imports are structure
  the graph proves rather than structure you infer.
- **sequence** is available wherever there are call edges, and there are usually
  far more of those than imports. Do not transcribe them: choose one path a
  reader would want to follow and show that, with the lifelines it touches.
- **state** is available wherever something moves through phases. It does not
  have to be a state machine in the code. A request, a document or a record that
  is created, checked, used and finished is one, and the doc comments usually
  say so in words.
- **use case** is available wherever the graph shows a surface something reaches
  the system through: commands, handlers, an exported API. The actors are
  whoever is on the other side of that surface, which includes other programs.

**And do not invent one.** If nothing in the tree moves through phases, leave
` + "`state`" + ` out. A family made up to fill the set is worse than a missing one,
because a reader cannot tell the difference and the drawing will look just as
confident either way.

**A top level is a map, not a census.** Somebody opening a diagram of a project
they do not know wants to see what the parts are and which way they lean, and
then to open the one they came for. Fifteen packages with every relationship
between them answers a question nobody asked first.

**Nest with ` + "`parent`" + `.** Nesting is what stage 3 turns into levels and
drill-down pages. Without it a component diagram is one flat page whatever its
size. Group by what a reader would go looking for, which is usually not the
directory tree: stages of a pipeline, or the parts of a subsystem.

**Watch the two numbers stage 3 acts on.** A level with fewer than %[1]d boxes is
unfolded: its members are replaced by their children and each unfolded component
is drawn as a band around them instead of a box. That is how a band appears, and
it is worth aiming for. An unfold that would push a page past %[2]d boxes does not
happen, and the page stays thin instead.

**A thin top level costs you every page below it.** That unfold is good news
deep in the tree and bad news on the first page. The top level is a level like
any other: put fewer than %[1]d components there and it unfolds, they become
bands, and their children take their place as boxes. Whatever those children
open still opens. If they open nothing, every level you built has collapsed into
one page with nothing to click, and the reader gets a list after all.

So the top level is the one place to count. Two components with ten beneath them
is one page of ten boxes and nowhere to go. Put at least %[1]d there, and give
them enough beneath to be worth opening.

**A page with too much on it loses relationships.** Stage 4 records every one it
cannot draw, so nothing goes missing silently, but a page that records a third
of its lines is a page that failed to say what it was for. Splitting the work
across levels is how you avoid that, and it is your decision, not the drawing's.

**Say where it came from.** ` + "`provenance`" + ` is part of the document. Put the graph you
read in ` + "`sourceGraph`" + ` and a sentence in ` + "`note`" + ` saying what you did and what you
judged, so a reader of the diagram can tell what was observed from what was
decided.

**Copy ` + "`revision`" + ` exactly, or leave it out.** If the graph you read carries a
` + "`revision`" + `, copy that object into ` + "`sourceGraph`" + ` character for character. It
reaches the finished page, where it is the only thing telling a reader which
version of the code they are looking at. Do not abbreviate it, do not tidy it,
and do not supply one from anywhere else: nothing downstream can tell a wrong
commit from a right one, so a wrong one is worse than none. A graph with no
` + "`revision`" + ` is a tree that was not in a checkout, and the honest model of it
carries none.
`

const closingProse = `` + "`validate`" + ` accepts or refuses it. ` + "`compose`" + ` turns it into one diagram source
per family, assigning every box a cell. ` + "`render`" + ` draws each of those into a
self-contained page.

Those two stages keep an account you are the start of. **drawn + dropped == proven**
holds on every level, on every document, and across the boundary between them. Every relationship your model states is either drawn or recorded on the
page with the reason it could not be. Nothing you return is discarded in
silence, and nothing appears that you did not state.
`
