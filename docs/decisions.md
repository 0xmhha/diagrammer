# Decisions

The design of 0.1.0 was settled in a 16-round interview before any code was
written. That transcript is the record, but it cannot be read from the top: the
design was revised three times as facts came in, and several early answers were
withdrawn later. Reading it in order produces an implementation of decisions
that were abandoned.

This page separates what holds from what was withdrawn, so the transcript can be
consulted for reasoning without being mistaken for a specification.

Source: Ouroboros interview `interview_20260915_124605`, 16 rounds, closed
2026-09-16 with an ambiguity score of 0.1305. Round numbers below cite it.

## How the design moved

Rounds 1 to 5 built a contract that measured diagrammer against Archify: a
pinned commit, a three-layer comparison, and a second artifact emitted purely to
feed Archify's checker.

Round 6 withdrew all of it, on two facts. The Archify branch carrying every rule
worth measuring against is 20 commits ahead of its origin and is never pushed,
so a pinned commit names something CI cannot fetch and a second person cannot
obtain. And every reference number came from a private repository that cannot
ship in an MIT project's testdata. Correctness moved to invariants checked over
synthetic fixtures held in this repository.

Round 10 rewrote the goal itself. The four-stage pipeline, the UML model, and
the second stage performed outside the binary all appear there for the first
time. That round reopened the language scope and reversed the decision to ship
one diagram family.

Rounds 11 to 16 settled the details on top of the rewritten goal and are the
specification.

**The trap is that withdrawn rounds carry live technical content.** Round 3's
scope statement is dead, but the viewer and renderer decisions inside the same
answer are current. Round 4's premise is dead, but its floating-point findings
are current. Neither round can be discarded or adopted whole.

## What holds

### Goal (round 10)

Parse a project's code into a graph with an AST parser. A plugin's skill
analyses that graph with an LLM and returns a `codegraph.json` expressed as a
UML model. Derive from that JSON the source data for UML diagrams (component,
sequence, state) and for use cases. Assemble the pages from an HTML template.

The four stages each run independently, because the second is performed by an
external plugin rather than by the binary. One compiled Go binary serves as both
a CLI and a local MCP server, so any plugin can drive it.

UML is load-bearing rather than decorative. A component diagram carries provided
and required interfaces, ports and dependencies; sequence carries lifelines,
messages, activations and fragments; state carries transitions with trigger,
guard and effect; use case carries actors, a system boundary, include and
extend. `codegraph.json` is a UML model, and that is the instruction the
stage-2 model works to.

### Correctness is proven by invariants, not by comparison (round 6)

Correctness is asserted against synthetic fixtures committed to this repository.
No external repository, no network, no private corpus, and no comparison with
another implementation.

Comparison with Archify stays available as a local development aid and is
documented as never being a gate.

### Schemas are the only contract (round 14)

Three JSON Schema files are committed and embedded with `go:embed`: the stage-1
graph schema, the stage-2 UML codegraph schema, and the stage-3 diagram-source
schema with a branch per family. `validate`, `compose` and `render` read only
those embedded schemas, and the stage-2 skill prompt is built from the same
files, so the instruction given to the model and the check applied to what it
returns come from one place.

Go structs are not the contract. They are either generated from the schemas or
checked against them by a test that fails the build on divergence. The drift
guard must exist; the decision is only worth taking if nothing can quietly
disagree with the schema.

This is forced by the external stage. A skill and a plugin can read a schema
file; neither can read a Go type. A hand-transcribed copy drifts from its
original without anyone noticing until the output is wrong.

Every document carries a schema version field, so a plugin detects a mismatch
and says so rather than half-consuming a document. Schemas are embedded rather
than read from disk, which is what keeps the binary self-contained and
`make verify` offline. Documentation under `docs/` describing a schema is marked
as derived: where the two disagree the schema wins and the documentation is what
gets corrected.

Once 0.1.0 ships the schemas are public contract. A change is a versioned
change, not an edit.

### Command surface (round 13)

    diagrammer graph    <src> -o graph.json       stage 1
    diagrammer validate <codegraph.json>          stage 2 boundary
    diagrammer compose  <codegraph.json> -o <dir> stage 3
    diagrammer render   <doc.json> -o out.html    stage 4
    diagrammer mermaid  <doc.json> -o out.md      stage 3, as text
    diagrammer instruct [-o prompt.md]            what stage 2 is performed from
    diagrammer serve    [-root dir]               the same, as a local MCP server
    diagrammer version

No single command infers its stage from the shape of the file it was handed.
Stage separation is a requirement, and a command that guesses makes the
boundaries invisible exactly where they matter most.

The `codegraph.json` declares which families the model supports, because what a
UML model can express is a property of its contents rather than of the command
line. `compose` emits every declared family by default and `--family` narrows it
to one. Asking for a family the model does not declare is refused with a clear
message rather than drawn empty. `validate` checks the declaration against the
contents, so a model claiming sequence support without lifelines is caught at the
boundary rather than three stages later.

`validate` is the only gate at the stage-2 boundary. `compose` and `render` may
assume a validated model and must say so rather than re-validating silently.

### What an exit code means (settled after 0.3.0, by writing down what the code already did)

Every subcommand reports on stderr and names the offending input path where one
exists. What the exit code says is narrower than "something went wrong", and the
distinction was decided in code long before anybody wrote it here.

**Exit 1: the command could not do what it was asked.** No artefact was
produced. An input that is missing or unreadable, a document that does not
satisfy the contract for its stage, a family the model does not declare, an
output path that cannot be written.

**Exit 0: the command did what it was asked, and says what it could not use.**
The artefact exists. A file that would not parse, a relationship the page could
not hold, a name there was nowhere to write: each is recorded in the output and
on stderr, and none of them is a failure of the command.

The line between the two is one question: **did the artefact the caller asked
for get produced?** A graph of a tree with one unreadable file is a graph. A
page that records three relationships it could not draw is a page, and the
record is the thing that makes it honest rather than the thing that makes it
broken.

**An output path that is already taken is overwritten, without asking.** Every
command here is one a person re-runs against the same paths while they work, and
a program that refused the second run would be a program you wrote a `rm` in
front of. What it does not do is inherit the file's old mode: the run that
produced the content decides who may read it, so the mode is set on every write
rather than only when the file is created. A code graph carries the doc comments
of everything it read, and leaving that at whatever the last owner chose is how
private prose becomes somebody else's to find.

Both halves are pinned by a test over the built command, because an exit code is
what a script branches on and nothing else here would notice it changing.

`instruct` was added after 0.1.1 and is the one capability that reads nothing.
Round 14 said the stage-2 skill prompt is built from the embedded schemas so
that the instruction given to a model and the check applied to what it returns
come from one file. Nothing built it: for two releases the only way to learn
what stage 2 must return was to find the schema in the source, which is a
requirement on whoever drives the pipeline that this program was supposed to
meet. It is a capability rather than a document in the repository for the same
reason the schemas are embedded rather than read from disk.

**It told a model what not to over-claim and never what to attempt** (after
0.4.0). The whole of "Choosing what to say" was about the component family:
nesting, levels, the unfold window. The only sentence about families said to
declare none you do not carry. A model reading that returned one family, or two,
and was right to. Asked for this repository, the answers held a component
diagram and sometimes a sequence one; the hand-written model of the same tree
carries four.

What the graph supports was never the constraint. This repository's graph holds
550 call edges against 37 imports, so the sequence family is better evidenced
than the component one, and the hand-written model reads a state machine out of
the relationship accounting and a use case diagram out of the command surface.
Both were there to be read the whole time.

The instruction now says which families a graph can carry and what each needs
from it, and says in the same breath not to invent one to fill the set, because
a family made up is worse than a family missing: a reader cannot tell, and the
drawing looks equally confident either way. Measured before and after on this
repository, one or two families became four, twice, with a state family
describing the real accounting lifecycle and a use case family naming the actual
capabilities.

**It described the unfold window and not what a thin top level costs.** The same
paragraph that says a level under `minBoxesPerLevel` is unfolded also says a band
is worth aiming for, and it is, on a page deep in the tree. On the first page it
is not: the overview is a level like any other, so too few components there
unfolds it, the children take their place, and if those children open nothing the
whole hierarchy has collapsed into one page.

That is not hypothetical. Pointed at a 29,000-line package of somebody else's
tree, a model returned twelve components with two at the top, and stage 3 drew
one page of ten boxes with nothing to open. The nesting was there and the unfold
undid it.

The instruction now says the top level is the one place to count, and why. Two
runs over the same package afterwards returned 23 components with five at the
top, which drew four pages, and 27 with six at the top, which drew seven and was
not unfolded at all. A model is not a function and neither run is a guarantee;
what changed is that the number it should be aiming at is now written down.

### The server is confined, the command line is not (after 0.2.0)

`serve` takes a root and refuses any path argument outside it. The default is
the directory it was started in, and `-root` widens it.

The two surfaces differ here although they share one set of capabilities, and
the reason is who chose the path. A path typed at a terminal was chosen by the
person who owns the terminal, and they can already write anywhere their shell
can; confining the command line would protect nobody from anyone. A path
arriving over MCP was chosen by a plugin, or by a model the plugin is driving,
and stage 1 has just handed that model the doc comments of a repository it did
not write. Text from a stranger's source tree reaching an argument that names a
file to overwrite is a short path, and it is worth closing.

**The default is the safe one, which refuses calls that worked in 0.2.0.** A
default that can be widened is worth more than one that can be narrowed: the
second is only ever set by somebody who already thought about the question, and
the people who need protecting are the ones who did not. The refusal names the
root and says which flag widens it, because the reader of that message is a
model and a message it cannot act on gets retried with a different path until
something works.

Enforcement sits on the MCP surface rather than in the operation, since the
operation serves both surfaces. What the operation owns is saying which of its
fields are paths, and that is on the `Request` interface so that a new
capability cannot be added without answering the question: the compiler asks.

CLI and MCP are two faces of one set of capabilities. `serve` exposes graph,
validate, compose, render and instruct with the same names, arguments and error
behaviour as the subcommands. Neither surface has a capability the other lacks. The MCP
tool and argument names are contract once 0.1.0 ships, because plugins bind to
them.

### Languages and parsers (rounds 12, 16)

Three languages, two parser paths. Go is parsed with `go/ast` from the standard
library. Python and JS/TS are parsed with a cgo-free pure-Go tree-sitter runtime
whose grammars are vendored into the repository.

tree-sitter is rejected for Go, on inspection rather than preference. Its
grammar's default branch has had no commit since 2025-09-15 and its
`method_declaration` rule carries no type parameters, so a Go 1.27 generic
method does not parse. tree-sitter fails soft by emitting an ERROR node and
continuing, so such a method disappears from the graph without an error, and no
LLM can restore what was never extracted. `go/ast` is free, faster, and current
with whichever compiler builds the binary.

The runtime must be cgo-free so `CGO_ENABLED=0` builds keep working. Grammars
are vendored rather than fetched, because `make verify` runs offline. The
runtime and every grammar get a `THIRD_PARTY_NOTICES` row naming project,
version and licence, added in the commit that brings them in. The runtime sits
behind an internal interface so it can be replaced without touching the
analyzers; it is a v0.x single-maintainer project and that risk is accepted
knowingly.

Two parser paths must not become two graph shapes. The graph schema is one
contract and every analyzer satisfies it.

### A drawing names the commit it was made from (after 0.3.0)

A diagram of a moving codebase is about a moment, and a page that does not say
which moment describes code that may no longer be there. So stage 1 records the
commit the tree was checked out at, and every stage after it repeats that
record unchanged until it reaches the page.

**Stage 1 is the only stage that can know it,** because it is the only one that
reads the source tree. `compose` reads a model and `render` reads a document;
neither has ever seen the code. That is why the value is copied rather than
looked up: a stage that went and asked git would be answering about whatever
tree it happened to be run in, which is not the tree the drawing describes.

**The .git files are read rather than git being run.** This program starts no
external process anywhere, and starting one here would make the answer depend on
git being installed and on what a person's configuration does to it. HEAD, a
ref file and `packed-refs` are three files with a simple format, and reading
them needs neither.

**It says what was checked out, and not that the files matched it.** Deciding
whether a working tree is clean means reading the object store, which is a
different order of work; a heuristic for it would be a number that is sometimes
wrong, and a commit that is sometimes wrong is worse than no commit. The schemas
say this in the field's own description, and so does the page.

**Two places take input from the tree being analysed rather than from the
caller,** and both are bounded rather than trusted. A HEAD naming a ref is
followed only within `refs/`, so a HEAD reading `ref: ../../../etc/passwd` is
refused instead of read and reported as a commit. A `.git` file naming a
directory is followed only if that directory holds a HEAD, which leaves a
hostile repository able to learn that some directory contains a file by that
name and nothing else. Both have fixtures that try it.

**Nothing verifies that the commit a model states is the one its graph
recorded,** and nothing can. Stage 2 runs outside this program, the instruction
tells it to copy the object unchanged or leave it out, and `validate` checks the
shape and not the truth. An invented commit would be believed. That is the same
class of trust the rest of stage 2 already runs on, and it is recorded here
rather than implied.

**What was asked for and not built:** evidence per box, so that clicking a
component opens the file it was drawn from. Stage 1 already carries a path and a
line for every node and stage 2 drops them, so the chain breaks at the schema
rather than for want of data. A commit is the part of that which is one field
and no new argument; the rest would make `compose` take a second input and ask a
model to transcribe numbers, and it was not worth that to answer a question
nobody had asked twice.

### What a language without a file generation actually loses (after 0.4.0)

This was left open as "how the unfold window behaves for a language without a
file generation, to be decided by measurement on the fixtures rather than
assumed". Measured, the answer is that the unfold window does not behave
differently at all, and something else does.

**Two models, the same ninety leaves, one generation apart.** One shaped like a
Go tree, package to file to function; one shaped like a tree with no file
generation, package straight to function.

| | pages | widest page | widest drawing |
|---|---|---|---|
| with a file generation | 19 | 6 boxes | 1888 px |
| without one | 4 | 30 boxes | 8608 px |

**The unfold window is never consulted for the wide page.** It runs only on a
level holding fewer than `minBoxesPerLevel`, and a page of thirty is not thin.
`expandedMaxBoxes` bounds what an unfold may produce and nothing else, so a page
that was never unfolded has no ceiling at all. The two ways a page fails to
split are opposite: a thin page is offered the unfold and has leaves to give it
nothing, and a wide page is never offered it.

**What the extra generation was doing was splitting, not unfolding.** A file
sits between a package and its functions, so a package page holds five files and
each opens a page of six. Remove it and the package page holds thirty functions,
because a column is spent per member and nothing anywhere folds one page into
two.

**Width is set by columns, measured at 287 to 315 px each across every fixture.**
Box count is the wrong proxy and `expandedMaxBoxes` could not have been reused:
a tall page of twenty-four boxes in one column is 488 px wide and perfectly
readable.

**The part that was a defect was not the width.** The stylesheet scaled every
drawing to the width of the page, so the eight-thousand-pixel drawing was shown
at about a sixth of its size, and every label in it went under `labelMinSize` at
once. That is the size the fitting refuses to go below, because below it a label
is not worth drawing. Nothing failed: the composition rules judge the drawing in
its own units, where the labels are still the size they were fitted to, so a
page nobody could read passed every check this project has.

**So the drawing now keeps the size it was laid out at and the page scrolls to
it.** `svgFor` writes that size, the stylesheet gives it something to overflow
into, and a test on each side holds the other to it. A drawing that fits is
unaffected and still stretches to the width.

**Folding was not built.** Splitting a wide page into levels is a decision about
what the diagram says, which is stage 2's, and `instruct` already tells a model
the two numbers. Stage 3 inventing levels the model did not ask for would be
stage 3 deciding what the drawing means. What changed is that the page is now
legible while it is wide, rather than illegible in a way nothing reported.

### A model is source, not build output (after 0.4.0)

`make diagram SRC=... AI=1` draws a tree in one command, and asking it twice
gives two different drawings. That is worth being precise about, because two
different things are going on and only one of them was a defect.

**Everything downstream of a model is already reproducible.** `make verify`
holds compose and render to byte-identity across runs, and has since round 11.
Hand the same model in twice and the same pages come out, on any machine.

**A model that differs only in order now composes identically, and did not
before.** Reordering every array whose order carries no meaning and composing
both, the levels, the boxes, where each box sits, the connections and the
accounting all came back the same. One thing did not: a box's `ports` were
passed through in whatever order the model listed them. Stage 4 does not read
ports yet, so nothing on a drawing moved, and a test that only looked at the
drawing would never have found it. They are sorted now, provided before
required and then by id.

A sequence diagram's messages are the boundary that makes the rest meaningful.
Their order is their meaning, the schema says so, and shuffling them is refused
by the validator rather than tolerated.

**Two answers from a model do not differ only in order.** Asked the same
question about this repository twice, it returned 23 components and 37
dependencies both times, which is a striking amount of agreement, and then named
them `analyze` once and `Analyze` the next, and emitted one family once and two
the next. Nothing about that is fixable here, and no amount of sorting reaches
it.

**So the model is kept, not regenerated.** Stage 2 is an authoring step and its
output is source: written once, read by a person, and committed, which is what
every file in `testdata/codegraph/` is. `make diagram MODEL=...` is that path,
and it is the one to use for anything whose drawing should not move under it.
`AI=1` is for the first draft and for a tree nobody has modelled yet.

This is the same reason `provenance.origin` distinguishes `model` from
`modelReviewed`. A model nobody has read is a draft, and the field says so.

### The one claim in a model that can be checked (after 0.4.0)

A graph too big to send is read by the model itself, which answered a question
nobody could answer before and raised one nobody had needed to ask: if the model
chooses what to read, how does anybody know it read all of it?

Measured by hand the first time, by looking for each area's name in the returned
model. That worked and it is the wrong instrument. It is optimistic in the one
direction that matters: a word that happens to appear reads as an area covered,
so the check says yes when it does not know.

**Everything a model returns is judgement except one thing.** A name is what it
chose to call something, a description is prose, and the nesting is a reading of
the code. None of that can be held to the tree. A graph node id can: it is in
the graph or it is not, and what sits beneath it is a fact the graph already
recorded.

So a component now says which graph nodes it stands for, in `accountsFor`, and
`validate --graph` does arithmetic instead of pattern matching. Naming a package
accounts for everything beneath it, which is what keeps this a handful of ids on
each component rather than a transcription of the tree.

**An invented id is refused and a missing area is reported,** and the split
matters. A claim about a node that is not there is wrong in a way a program can
see, so it stops the command. An area nobody stood for is not wrong at all: a
model is a map rather than a census and leaving things out is often the right
call. What it may not do is leave them out silently.

**Saying nothing and covering nothing are kept apart.** A model without the
field has not failed to cover the tree, it has not been asked, and every model
written before this is one of those. Reporting them as zero would be a number
that means the opposite of how it reads.

**The root is not counted.** A component claiming it would account for
everything by saying nothing.

Measured on the whole of another project, 15,660 nodes read rather than sent:
15,659 of 15,659 accounted for, every area claimed by some component. The worry
that motivated splitting the graph into pieces turns out to be answerable
without splitting it.

This is also the shape the deferred source-evidence question needs. Line numbers
were refused because a model transcribing them would produce numbers nothing
downstream could check. An id is not a transcription, and it is checked; what a
node's file and line are is then a lookup rather than a claim.

### A document has a second way out, as Mermaid (after 0.5.0)

`render` draws a page from a stage-3 document. `mermaid` writes the same
document as text, one fenced block per level, and is a peer of it rather than a
feature of it: both read the document, neither reads the other.

**Why Mermaid and not a format of this program's own.** Because the places a
diagram is taken to already read it. A README on GitHub, a Notion or Obsidian
page, and a redrawing tool such as diagram-design all take a fenced Mermaid
block and none of them would take anything invented here. diagram-design in
particular was the occasion: it redraws Mermaid in an editorial design system of
its own, at a chosen size and level of detail, and its `faithful` ceiling is 24
nodes, which is this program's own page ceiling. A level here is a diagram
there, and the two were built to the same number without knowing it.

**The text carries what the page could not.** A page is bound by a grid and
records a relationship it cannot route. Mermaid lays itself out and has no
reason to leave a line off, so the text carries every relationship the document
proved, and marks the ones the page recorded so a reader can tell them apart.
The accounting keeps the same shape as everywhere else: carried plus omitted
equals proven, on every level, and the command prints it.

The one omission is a recorded message in a sequence diagram. Its order is its
meaning and a recorded message has none, so placing it would be inventing an
order. It is named in a comment and counted as omitted. No fixture records a
message; a test constructs one so the path is not a rule nobody has seen fire.

**Ids are rewritten, totally and deterministically.** Stage-3 ids carry dots
and colons, 107 of the 198 in the fixtures, and Mermaid reads both as syntax.
Every id becomes an identifier; a keyword such as `end`, which closes a
subgraph, gains an underscore; and a collision is resolved in sorted order with
an id that was already acceptable keeping its own name. Two runs over one
document produce one text.

**What each family loses.** Nothing, for component and state: subgraphs are
bands, `[*]` is a pseudostate, a composite is a block. A sequence diagram
loses the `strict` and `seq` fragment kinds, which Mermaid does not have and
which are written as `opt` with the kind in the guard and a comment saying so.
A use case diagram has no grammar of its own anywhere in Mermaid and is
written as a flowchart with stadiums for actors and dashed arrows for include
and extend, carrying the stereotype as their label. That is an approximation,
the README says so, and diagram-design would route it to its architecture type
rather than a use case one, because it has none either.

**Measured, not assumed.** Every family of two fixtures was written and handed
to diagram-design's own extractor, `mermaid_extract.py`, with `--diagram all`.
All eight were accepted; the five-level component document came back as five
diagrams with the node and edge counts the document has. That is the importer
on the other side saying yes, which is the only claim about interoperability
worth making.

**Not built: an importer of our own.** diagram-design's other two inputs are
draw.io and Excalidraw, and this program could write either. It writes neither,
because nothing this program produces is improved by passing through a drawing
tool, and one text format the destinations already share is what was asked for.
### The page is drawn to a handful of rules (after 0.5.0)

The pages were compared with those of an editorial diagram tool, and the gap
was measured item by item before anything was changed. Half of it was already
closed: the token roles, the absence of shadows, the corner radius, the
splitting of a large model into levels and the bands that zone a page were all
there. What remained was strokes at 1.2 to 2.2 where a hairline is 1, one sans
face doing every job, the accent on every box that opened a page, and drawings
that said nothing to a screen reader.

**The rules, not the tool.** What that tool provides is a model drawing HTML by
hand to a design system, and the system is the part worth having. A model in
stage 4 would cost the determinism the whole pipeline rests on, and the
measurements of stage 2 say what that would mean: the same input drawn twice
differently. The rules cost nothing to keep and a test each to hold.

- **Hairlines.** Every stroke at rest is 1 or under; only what is under the
  pointer goes to 1.5.
- **One accent, and only where the reader is looking.** It appears under
  `.focusing` and in the token block and nowhere else. A box that opens a page
  used to take the accent, which on a page of six openers was six accents and
  no signal; it says so with a dotted underline on its name now.
- **Three font roles, by what a text is.** Serif for the title, sans for a
  name, mono for anything technical: a line's label, a zone's label, the
  eyebrow, the record. The families are system stacks and not the tool's own,
  because the page opens offline and a web font would put a network request in
  a file that has never needed one. A brand face is one token away.
- **No shadows, corners under 10.** Both already true, both now held.
- **A drawing announces itself.** `role="img"`, a title first, a description,
  wired with `aria-labelledby`. The tool's own self-check, run on a page before
  and after for information, failed all five of its accessibility checks before
  and passes them after; what it still fails is specific to its motion
  template.

**What was not taken.** The four-pixel grid, because the lane gap it would
move sets a channel's capacity and moving it drops relationships; that is a
threshold with a measurement behind it and a change to it starts with a new
one. And the nine-box density rule, because the levels already are that rule:
a page here is what that tool calls an overview plus detail.

**Held, not remembered.** `TestTheStylesheetKeepsTheEditorialRules` reads the
stylesheet with its comments stripped and holds every stroke, the accent's
placement, each role's face and every corner radius. `TestEveryDrawingAnnouncesItself`
holds the markup. A rule that erodes one commit at a time is exactly the kind a
test is for.

### The renderer is reimplemented; the viewer is embedded (round 3)

Go reimplements the renderer. The viewer ships as one frozen embedded asset.

The viewer is not Node code: it uses only `document` and `window`, and runs in
the browser inside the emitted HTML, so embedding it puts Node nowhere near the
shipped path. It is a generated build artifact of roughly 780 KB, and embedding
it grows the binary from about 1.5 MB to about 2.3 MB. It is copied material and
takes a `THIRD_PARTY_NOTICES` row in the commit that brings it in.

Layout cannot be deferred to the viewer under any arrangement. The document
format is a placed format: components carry position and size, connections carry
end sides and routes. Whoever owns layout owns the composition gates, because
those gates read routed polylines that only a renderer produces.

Three constraints from this round are easy to lose and expensive to rediscover:

- **The renderer-to-viewer DOM contract must be pinned by a generated test.**
  The viewer reads 195 distinct `data-*` attributes. A renderer that omits one
  still produces a page that draws, while interaction dies silently, and no
  composition rule would notice, because those rules check the drawing rather
  than the attribute vocabulary. The test extracts the attribute set from the
  pinned viewer and fails on mismatch.
- **The East Asian width table must be transcribed exactly.** Text is measured
  by arithmetic rather than font metrics: width is text units times font size
  times 0.6. The table deciding which characters count as two units is one
  regular expression of roughly 46 ranges with deliberate deviations from the
  Unicode standard, and `golang.org/x/text/width` will not reproduce it. This is
  the most transcription-error-prone part of the port, and it matters because
  labels here contain CJK.
- **Brand marks are catalog-only.** The live fetcher opens network connections.

Schema validation uses `santhosh-tekuri/jsonschema` v6.0.3: draft 2020-12,
cgo-free.

### Composition rules are ours, checked natively (round 7)

The composition checker is reimplemented in Go as an in-repo test. No Node step
in CI, and no vendored copy of the upstream script. Vendoring it would reinstate
the unfetchable dependency that round 6 removed, and would leave the gates as
somebody else's code that cannot be fixed without editing a copy.

Seven rules move into Go beside the renderer: endpoint side matches route
direction; no route through a non-endpoint box; no proper crossing between
unrelated relationships; no ambiguous corridor; no route following a boundary
border instead of crossing it; route rhythm, covering bend count, stretch ratio
and minimum segment lengths; and connection-label clearance from every route.
Alongside them sit the renderer's own local checks: unique ids, finite positions
and sizes, viewBox containment, label wider than its box, component separation,
boundary title containment and overlap, minimum connection length.

`data-composition-points` is a specified attribute of every emitted path,
written unrounded. The checker reads the emitted artifact rather than the
renderer's in-memory state, deliberately: that is what makes the check judge
what was produced rather than what was intended. The Go side needs its own
parser for that attribute, with unit tests, so a malformed or missing attribute
fails loudly rather than skipping a check in silence.

**Each rule needs a fixture that makes it fail.** A ported check that never
fires is indistinguishable from a check that was never ported, and that is the
main risk of reimplementing rather than invoking.

### The viewer is ours, not the reference's (supersedes part of round 3)

Round 3 decided to embed the reference implementation's viewer verbatim as a
frozen asset, and to pin its 195 `data-*` attributes with a generated test. That
was decided before round 10 rewrote the goal, and it does not survive the
rewrite.

Measured before deciding: the viewer is 8,781 lines across 13 files, and 47 of
its 195 attributes exist for features this project has no equivalent of — guided
views, story beats, route journeys, semantic radar and lens, reach sharing. The
rest assume that project's own document format, which round 10 replaced with a
UML model carrying four families. Embedding it would mean either emitting
attributes for features that do not exist, or pinning a contract most of which
we could never satisfy. Pinning a contract we cannot meet pins nothing.

So the viewer is written here, small, and covered what 0.1.0 named: level
navigation, which is the drill-down capability round 9 listed. It has since
grown one thing more, which is the argument for owning it: resting on a box
fades everything it is not joined to. That was three CSS rules and sixty lines
against a vocabulary this project already had, and it would have been a feature
request against somebody else's. The DOM contract requirement from round 15
stands unchanged in intent — a missing data attribute
leaves the page drawing while interaction dies in silence, and no composition
rule would notice — so our own, smaller vocabulary is pinned by a generated test
exactly as that round asked.

Two consequences, stated rather than discovered later:

- **Features are lost.** Search, minimap, the passport panel and guided views
  are real things the reference has and this does not. They are not in any
  acceptance list for 0.1.0, and they can be added later against a vocabulary we
  own.
- **Two attributions are no longer needed.** The 780 KB viewer asset is not
  copied, so it takes no notices row. Neither does the full-width character
  table: transcribing it exactly was only ever required to match that
  implementation's output byte for byte, and round 6 removed that comparison
  from the contract. Character width is derived from the Unicode standard
  instead, which produces different widths and is the right basis for a renderer
  that is not imitating another one.

### Routing failure is a drop, not a defeat (round 7, made concrete)

A router that satisfies every composition rule on every diagram is a hard
problem, and nothing requires one. The reference draws 800 of 1,355 proven
relationships and records the rest, which is what the accounting invariant is
for.

So stage 4 routes what it can and drops what it cannot, recording each drop on
the box it belonged to with the rule that refused it. The invariant carries
across the boundary: stage 4's proven is stage 3's drawn, and drawn plus dropped
equals proven at each stage.

That is also why stage 4 has drop reasons stage 3 does not. A grid that cannot
route a line is a geometric problem, and geometry lives here.

A drop being permitted is not the same as a drop being right, and one class of
them turned out to be avoidable. Channels used to be one size everywhere, which
is one size for the average, and a hub is where the average stops being a guide:
every component depending on one box sends its route through the same channel
while the neighbouring channels sit empty. Channels are now sized from the
routes that will use them, which is what `channelsWanted` in
`internal/render/route.go` is for. It came out of comparing two real projects,
and `docs/thresholds.md` has the measurements and the ceiling that widening
stops at.

The same comparison showed a second class. A crossing is the only rule that
condemns two routes at once, and the renderer used to take out every route the
checker named without noticing that naming one of each pair was already a
choice, and an arbitrary one. It now takes out the fewest routes that leave no
crossing behind. Two shapes changed with it: a detour leaves by the side facing
its target rather than always downward, and box-edge positions come from one
allocator rather than two, which had been handing two routes the same place on
the same box.

### Floating point (round 4)

The premise of this round was withdrawn; its measurements were not.

One numeric emission helper: quantize where the source already quantizes, format
with `strconv.FormatFloat(v, 'f', -1, 64)`, and normalise negative zero to `"0"`
because JS renders it that way and Go does not. Hard-fail on NaN and Inf rather
than serialising them.

`jsRound(x) = math.Floor(x + 0.5)` replaces every `Math.round` the port mirrors.
JS rounds negative halves toward positive infinity while Go rounds away from
zero, so `-2.5` gives `-2` there and `-3` here. Verified on eight values.

Float expressions keep the same evaluation order as the source. Rewriting one
for readability is a correctness change.

Formatting itself is not a risk: Go matched V8 on all 48 distinct long-float
values in a real artifact and on 20,003 of 20,007 random and boundary values,
the four misses being notation thresholds a diagram coordinate cannot reach.
The residual risk is `Math.hypot`, which is libm-defined rather than
bit-guaranteed, and which is the strongest single reason bytes are not gated.

### Acceptance (rounds 11, 15, 16)

`make verify` exiting zero is the only automated gate, and it is what authorises
a tag. It runs offline on a clean macOS machine with only Go and make installed.
Anything it needs that such a machine lacks is a defect.

What it runs, by stage:

- **Stage 1.** At least one source fixture per language emits a graph that
  validates against the graph schema and is byte-identical across runs. A
  fixture per language contains a deliberate parse failure, asserting the
  failure is counted and reported rather than silently dropped. This weighs more
  on the tree-sitter path, whose soft failure is exactly how missing code hides.
- **Stage 2.** Skipped. It is an external responsibility, and the acceptance
  ritual does not attempt it. Stage-2 inputs are `codegraph.json` fixtures
  committed to the repository, each recording its provenance so a reader can
  tell a hand-written fixture from one a model generated and a human reviewed.
- **Stage 3.** Each fixture produces a document that validates against its
  family's schema. The completeness rule for that family holds. Every element of
  the model appears somewhere in the output; nothing is silently discarded. The
  families the model declares and the families `compose` emits agree. Output is
  byte-identical across runs. At least one fixture per family is dense enough
  that the record path is exercised rather than merely present.
- **Stage 4.** The family's composition rules pass, checked by our own Go
  checker over the emitted artifact. Document to HTML is structurally lossless:
  every element in the document is present in the artifact and the artifact
  introduces none the document does not account for. The SVG comparison is exact
  on element order, tag names, classes, text and every non-numeric attribute,
  with a 1e-6 tolerance on numeric attributes only. Output is byte-identical
  across runs.

Golden HTML byte comparison runs and reports but never fails the build. Byte
equality has total sensitivity and almost no specificity: a good canary and a bad
specification. A difference is explained or fixed by a person, never silenced by
regenerating the golden file.

The 1e-6 tolerance is written down with its justification: far below one device
pixel, far above accumulated double error at diagram scale.

Recorded honestly rather than glossed: nothing in `make verify` demonstrates
that an LLM actually produces a conformant `codegraph.json`. The fixtures could
be more obliging than reality. That is accepted deliberately, because the
alternative is a network dependency inside the acceptance ritual.

### Completeness rules, per family (round 16)

All four families ship with their rule defined and their fixtures written.
Leaving three of them open would mean shipping with no way to tell a correct
diagram from a lossy one.

The general form is completeness: every element of the model is either rendered
or accounted for with a reason. The component family's version is specific to a
grid that cannot route everything, and does not transfer unchanged. A sequence
diagram does not drop messages the way a grid drops relationships.

- **component** — every proven relationship is drawn, or recorded on its box
  with a reason; `drawn + dropped == proven` holds exactly.
- **sequence** — every message and activation appears; ordering is preserved;
  no lifeline referenced by a message is missing.
- **state** — every state and transition appears; every transition's source and
  target resolve; initial and final states are present where declared.
- **use case** — every actor, use case and association appears; include and
  extend resolve to declared use cases; nothing sits outside the system boundary
  that the model placed inside it.

Each needs a fixture that violates it, so the rule is proven to fire. All four
are written into `docs/invariants.md`, so the contract is one document rather
than one rule and three verbal understandings.

### The four component-family capabilities (rounds 9, 11)

Multi-level free placement; drill-down pages that open and return; region frames
for the generation an unfolded level skipped; and the per-box record of every
proven relationship the drawing could not hold.

These are one argument rather than four features. Free placement without
drill-down is a picture that cannot go deeper. Drill-down without the record
silently loses what the grid could not draw. The region frames are what stop an
unfolded overview from reading as a flat list. Removing the fourth would also
remove the only way to check `drawn + dropped == proven`.

Round 11 attached them to the component family specifically; they are properties
of that layout rather than of the product.

### Shipped documents

Deliverables, carrying acceptance weight:

- `docs/invariants.md` — the correctness contract in full: composition rules,
  the four completeness rules, the accounting invariant, determinism.
- `docs/analyzer-interface.md` — the extension point for a new language.
- `docs/thresholds.md` — why 6, 14, 24 and 1.4, written as the measurements
  that produced them, so a future change is made the same way rather than by
  taste.
- `docs/install.md` — the notes a recipient of a packaged build needs, and the
  only document that ships inside the archive rather than beside the source.
- `docs/licensing.md`, `THIRD_PARTY_NOTICES.md`, `README.md`.

Anything else under `docs/` is a working note. This page is a working note.

### The human release checklist (round 16)

Beside `make verify`, and not automated, because each item is a judgement:

- Triage any difference from the golden HTML byte comparison.
- Confirm `THIRD_PARTY_NOTICES` lists every module compiled into the binary,
  each with its version and licence. `go list -deps ./cmd/diagrammer` names
  them, and it has to be run **twice**: the two builds link different modules,
  and the four grammars and the tree-sitter runtime appear only in the second.
  The viewer asset this item was also written for is gone; the viewer is ours.
- ~~Confirm the shipped documents match the code they describe.~~ Automated,
  as the rule below says such an item should be. `make verify` reads the
  thresholds and the rule names out of the source and fails when a document
  quotes a number the code no longer uses, or omits a rule the checkers have.
  What is left for a person is whether the prose still says something true,
  which no test can judge.
- Confirm the measured cost of the tree-sitter runtime has been recorded. This
  item was struck as moot while no build carried the runtime. One does now, so
  it stands again, and it is still owed: the figures in **Still open** are that
  project's published ones and have never been taken here.

- Decide whether this release is published as archives, and whether they are
  notarised. `make dist` builds them and proves them; nothing puts them
  anywhere, and ad-hoc signing leaves a recipient one `xattr` command to run.

Nothing on this list may silently substitute for a test. An item that can be
automated moves into `make verify` rather than staying a habit.

### Packaging for a mac that did not build it (after 0.3.0)

`make install` needs a Go toolchain on the machine that will run the program,
which is a fine answer for this machine and no answer at all for anyone else's.
`make dist` produces an archive per mac architecture, each holding both builds,
the licences and `docs/install.md`, with a `SHA256SUMS` beside them.

**Cross compiling is allowed here and still refused for linux.** That is one
rule rather than two: nothing ships until it has been run. Rosetta runs an
x86_64 mac binary on an arm64 one, so `make dist` unpacks each archive it makes
into a directory that is not this one, with an environment holding nothing but a
`PATH`, and runs the whole pipeline out of it with both binaries. Nothing here
runs a linux binary, so `make linux` still refuses, and when a linux runner
exists it becomes the same target pointed at that.

**The defect this found.** clang defaults the deployment target to the version
of macOS doing the building. So the four-language build was declaring it needed
the builder's own macOS, 26.0 on the machine that found it, while the Go-only
build beside it declared 12.0. Nobody would have noticed here: both binaries run
on the machine that produced them, which is the one place the fault cannot show.
`make dist` sets the floor for both builds and reads it back out of every packed
binary, and a mismatch fails the target.

**12.0 is a declaration, not a measurement,** and the document that ships says
so. It is the floor this Go toolchain already puts on a build of its own, so it
is the one number that is not invented. The oldest macOS anything here has been
run on is the one that packaged it.

**Ad-hoc signing is not notarisation.** The binaries are signed with an
identifier of their own, which lets a recipient ask `codesign` whether the file
still matches what was signed and answers nothing about who signed it.
Gatekeeper is unmoved: a build that arrives carrying a quarantine flag is killed
on sight, and the remedy is `xattr -d`. Notarisation would remove that step and
needs an Apple developer account, which is a decision for whoever publishes
rather than something the build can take.

Two things about quarantine were measured rather than assumed, because both are
widely asserted in both directions: unpacking with `tar` in a terminal does not
quarantine the contents even when the archive itself was quarantined, and
Gatekeeper kills the process rather than refusing to start it. `make dist`
quarantines a packed binary, watches it refuse to run, removes the flag and
watches it run, so the instruction in `docs/install.md` is one that has been
seen to work rather than one that was repeated.

Archives are deterministic: timestamps, ownership and member order are all
pinned, and `make dist` packs each staged directory twice and compares. That is
a claim about the archiving step only. Whether two builds of the same source
produce the same binary is a Go toolchain property that has not been tested
here, and is not claimed.

### What the linter is for, and is not

`golangci-lint` runs over both builds, because a run with `CGO_ENABLED=0` never
sees the four grammars: every file in that package but its doc is behind a build
tag, so the checks would pass by not looking.

It is deliberately not part of `make verify`. The release gate is a clean
machine with only Go and make, and requiring a linter would change that
definition for checks that find style rather than defects. It is part of
`make check`, which is what to run before a commit on a machine that has it.

The enabled set is the default plus the checks this project has actually been
bitten by, and nothing chosen for completeness. A linter reporting things nobody
intends to fix trains people to skim its output, which costs more than the
checks are worth.

Two exclusions are decisions rather than conveniences, and `.golangci.yml` says
so where they are made. `G304`, reading a file whose path came from a variable,
can never fire usefully here: every file this program reads is one the caller
named, and a security review found no trust boundary being crossed. `G115`,
integer conversion, is about grid indices and parser positions, both bounded by
the document they came from.

One finding was fixed rather than excluded. Output was being written 0644 into
0755 directories, and a code graph carries the doc comments of everything it
read: pointed at a private repository, the output holds private prose.
World-readable is the wrong default for that, so it is now 0600 and 0700, and a
test holds it there. Widening it is one chmod the person who wants it can run.

## What was withdrawn

| Withdrawn | Round | Replaced by | Why |
|---|---|---|---|
| Three-layer contract measured against a pinned Archify commit | 1 | 6 | The branch holding the rules is never pushed; the commit cannot be fetched by CI or obtained by a second person |
| The reference counts 3,029 / 3,247 / 431 / 3,039 / 1,355 / 800 | 1, 2 | 6 | Measured on one private repository at one commit, with thresholds that moved three times in a day; freezing them turns a tuning knob into an API |
| Byte-identical graph output as a cross-implementation contract | 2 | 6, 12 | Byte-identity now means identical across our own runs, not identical to another implementation |
| Sequence, lifecycle, dataflow and workflow renderers out of scope | 3 | 10 | The rewritten goal puts component, sequence, state and use case all in 0.1.0 |
| Upstream's golden test as the conformance oracle | 3 | 4 | It hardcodes a Node invocation and byte-compares checked-in HTML; it is that project's self-consistency test and knows nothing about Go |
| Embedding the reference viewer and pinning its 195 attributes | 3 | stage 4 | Decided before round 10 rewrote the goal; 47 of those attributes serve features this project does not have, and the rest assume a document format round 10 replaced |
| Transcribing the full-width character table exactly | 3 | 6, stage 4 | The only reason to match it was byte comparison with that implementation, which round 6 removed from the contract |
| Byte equality as a gate | 4 | 4, 15 | Total sensitivity, near-zero specificity; `Math.hypot` is not bit-guaranteed, so it cannot be promised |
| A second layout-JSON artifact emitted to feed the checker | 5 | 6 | False premise: the checker reads `data-composition-points` from the HTML, already unrounded, so no second artifact is needed |
| 0.1.0 reads Go only, Python and JS/TS deferred | 8 | 10, 12 | The rewritten goal states AST-based multi-language parsing, and stage 2 now supplies the meaning that weak call resolution used to owe |
| The four capabilities as the whole product's feature set | 9 | 11 | They are properties of the component layout; the other three families have their own rules |

Two corrections recorded inside the transcript itself, both worth keeping,
because each would otherwise send an implementer at the wrong problem:

- The per-language divergence in unfolding is **not** that Python and
  JavaScript graphs lack `file` nodes. The unfold logic never inspects node
  kind. A fix written against "JS lacks file nodes" fixes the wrong thing.

  The rest of this correction was itself wrong, and the section below has the
  measurement. It said a language without a file generation "jumps from package
  straight to functions and usually overshoots" the 24 an unfold may not pass.
  It does not overshoot: the unfold is never attempted, because a wide page is
  not a thin one and the window is only consulted for thin pages.
- The long decimal coordinates in Archify's output predate the local branch.
  They appear in checked-in examples on its `origin/main`. The region-boundary
  work multiplied them in large artifacts rather than introducing them.

## Unblocked: two builds from one source

The block below stood for as long as the question was "which cgo-free runtime",
and it has no answer. The question that does is **which binary**.

`CGO_ENABLED=0` compiles an analyzer for Go and nothing else. That is the build
the release gate runs, and it needs no C toolchain, so round 9's definition of
done is untouched. `CGO_ENABLED=1` compiles the same source with tree-sitter
analyzers for Python, Solidity and JS/TS as well.

The structure was already there. The analyzer registry is assembled by its
caller rather than registered into globally, precisely so that what a build can
read depends on the build; `graph` reports the languages it read on every run,
so nobody learns the scope by pointing the program at a repository and wondering
why the graph came back nearly empty.

**What it costs has been measured.** Round 12 said the runtime's cost had not
been taken here and that the result might reopen the choice. It has been taken
and it does not: 2.63 MB of binary, against a published figure of roughly 20 for
the whole grammar forest, and a parser between a third and half again slower
than `go/ast` rather than the cliff a comparison with native C suggests. The
numbers, how they were taken and the two make targets that take them again are
in `docs/thresholds.md`. The cold build is the one figure that grew sharply,
from 5.85 seconds to 12.63, and it is paid by whoever builds rather than by
whoever runs.

**The soft failure is answered rather than accepted.** tree-sitter does not
refuse a file it cannot parse: it emits an ERROR node and carries on, which is
why this project does not use it for Go, where the standard library gives a
parser that says no. It can be asked, though. Every tree is checked with
HasError and a file that parsed with errors is recorded with the position of the
first one, so a silent gap becomes a reported one.

**What that recovery is worth was not measured until later.** Pointed at the
reference tree, two files come back as failures, and both turned out to have
contributed every declaration they have. What the grammar could not read was a
NUL byte used as a separator inside a template literal, which Node accepts and
this grammar does not, sitting inside one function body. So recovery is not a
consolation prize here: it is the difference between losing an expression
nothing extracts anyway and losing forty-four declarations. The report now says
which of the two happened, per file, counted from the graph. A fixture with a deliberately
broken JavaScript file holds that to it: the part that parses is still read, and
the failure is still named.

**The cost, stated rather than glossed.** Two things.

One name now means two binaries. `go install` builds with cgo on by default and
gives four languages; a distributed binary built for a machine without a C
toolchain gives one. Every run says which, and `make build` and
`make build-polyglot` are named apart so nobody produces one while meaning the
other.

And a second gate. `make verify` cannot cover the cgo analyzers without
requiring a C toolchain, which is the thing it exists not to require. So
`make verify-cgo` exists, needs one, and a release claiming those languages has
to pass it. An analyzer no gate covers is worse than one that does not exist,
because its output looks the same as a tree with none of that language in it.

Vendoring the grammars takes `vendor/` from 4.7 MB to 22 MB. Both gates run
offline, which is what that buys.

## Blocked

### The cgo-free tree-sitter runtime does not exist (round 12, tested)

**Resolved by the section above**, which did not find one. It stopped needing
one. Kept because the finding stands and the four ways out are still the four
ways out.

Round 12 settled stage 1 as three languages behind two parser paths: `go/ast`
for Go, and **a cgo-free pure-Go tree-sitter runtime with vendored grammars**
for Python and JS/TS. That round also said the runtime's cost had not been
measured here and that the result might reopen the choice. It has, and earlier
than expected: the premise itself does not hold.

What was tested, on 2026-09-16:

- `github.com/alexaandru/go-sitter-forest` v1.9.163 carries grammars for Python,
  JavaScript and TypeScript, and depends on
  `github.com/alexaandru/go-tree-sitter-bare` v1.10.0 as its runtime.
- That runtime is **not** cgo-free. `tree.go` and `query.go` both open with
  `// #include "sitter.h"` followed by `import "C"`.
- Building a program that imports the Python grammar with `CGO_ENABLED=0` fails
  outright: *build constraints exclude all Go files in
  .../go-sitter-forest/python@v1.9.10*. With `CGO_ENABLED=1` the same import
  compiles.
- No wasm-backed Go binding exists under the obvious names
  (`wasilibs/go-tree-sitter`, `smacker/go-tree-sitter-wasm`,
  `tree-sitter/go-tree-sitter-wasm` are all absent from the module proxy).

Why this is a blocker rather than an inconvenience: the constraint is not a
preference, it is load-bearing for the release gate. Round 9 says `make verify`
must run on a clean macOS machine with only Go and make installed, and that
anything it needs beyond that is a defect. cgo needs a C toolchain, which such a
machine does not have. Accepting cgo would not merely add a dependency; it would
invalidate the definition of done.

Everything built so far compiles with `CGO_ENABLED=0`, which was confirmed while
testing this.

Four ways out, none of them a tidy-up:

1. **Accept cgo for the two languages.** Contradicts round 2, round 8 and round
   12, breaks `CGO_ENABLED=0`, makes cross-compilation a cross-toolchain
   problem, and puts a C compiler in the release gate's prerequisites.
2. **Find or build a wasm-backed runtime.** Keeps every constraint. Nothing
   off the shelf does it, so this is a project of its own.
3. **Write parsers by hand.** Round 2 already surveyed the pure-Go options and
   found none current: gpython targets Python 3.4, and the two production Go
   TypeScript parsers keep theirs under `internal/` deliberately.
4. **Ship 0.1.0 reading Go only**, which is what round 8 decided before round 10
   reopened it. The analyzer interface exists, so the other two remain additions
   rather than redesigns.

This is a decision about what 0.1.0 is, so it is recorded here rather than
resolved in code.

**Taken: option 4.** 0.1.0 reads Go only.

**Superseded after 0.1.1 by a fifth way out** that the four above did not
contain, because all four asked which runtime and the answer was which binary.
See **Unblocked** above. What survives from this round is the finding itself,
and the reason the release gate still runs the Go-only build.

The reasoning is short. The whole pipeline works through the Go path today and
`make verify` runs on a clean machine, which is what round 9 defined done as.
Option 1 changes that definition; options 2 and 3 postpone the release by a
project each.

Round 16 listed three languages as a condition of done, and that condition was
written on the assumption that a cgo-free runtime existed. The assumption was
wrong, and round 12 said in as many words that measuring might reopen the
choice. It reopened earlier and harder than expected, which changes the timing
rather than the principle.

What follows from it, and is done:

- The analyzer interface is defined and documented, so the other two languages
  remain additions rather than a redesign. See docs/analyzer-interface.md.
- `graph` reports which languages the build reads, every run. Nobody should
  learn the scope by pointing the program at a Python repository and wondering
  why the graph is nearly empty.
- The README says it plainly, which round 8 asked for and no version of the
  README had said until now.

What it costs, stated rather than glossed: 0.1.0 ships reading one language of
the three that were settled. Anyone who wanted the other two gets an interface
and a written account of why, which is less than they wanted.

## Still open

Deferred deliberately, to be settled when the work reaches them:

- Whether a drawing should carry evidence back to the source it came from, per
  box, so that a component opens the file it was drawn from. **The commit is now
  carried** and the section above says how; what is still open is the rest. The
  data is there and unused: stage 1 records a path and a line for every node,
  and the stage-2 schema has nowhere to put them, so the chain breaks at the
  contract rather than for want of evidence. Closing it means `compose` taking
  the graph as a second input, because asking a model to transcribe line numbers
  would produce numbers that are wrong in a way nothing downstream can detect.
- Which architectures the binary targets. Both mac architectures are now built,
  packaged and run by `make dist`; `make linux` still refuses, because nothing
  here can run a linux binary and the rule is that nothing ships until it has
  been run. A native linux runner is what that waits on, and it is now the only
  thing it waits on.

  What a plugin needs once it has the binary is no longer open. The server says
  what it is for at `initialize`, offers the stage-2 instruction as a prompt,
  and serves each schema as a resource under its own `$id`, so a client learns
  the pipeline from the server rather than from a document beside it. Getting
  the binary onto a machine that cannot build it was the part that remained, and
  is answered above for macOS.

  Not answered: nothing publishes the archives. `make dist` produces them and
  proves them; putting them where somebody can fetch them, and whether that
  comes with notarisation, has not been decided.
- Whether stage 3 should fold a page that is too wide, rather than only unfold
  one that is too thin. Measured and left undone; the section above says what
  the measurement found and why folding is stage 2's decision rather than stage
  3's. The page is legible while it is wide, which is what made this not
  urgent.
