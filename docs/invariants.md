# Invariants

The correctness contract. Everything here is checked by a test over fixtures
held in this repository, and `make verify` runs all of it.

Nothing here is checked by comparison with another implementation. The one this
project learned from lives on a branch that is never pushed, and the numbers it
was measured on came from a private repository, so neither could serve as an
oracle a second person could reach. Correctness is therefore stated as
properties a document must have, not as agreement with something else.

## The shape of every rule

**Every element of the model is either rendered, or accounted for with a
reason.**

That is the whole of it. What it means differs by family, because a grid that
cannot route everything and a message ladder that never loses a message are not
the same problem, so each family states its own form of the rule below.

Two things follow from the shape, and both are rules in their own right:

- A rule that never fires cannot be told apart from a rule that was never
  written. Every rule has a fixture that violates it, and a test asserts that
  the violation is caught.
- A rule enforced only by the code that produces a document proves nothing about
  the document. The checks read the emitted artifact, not the producer's
  intentions.

## Determinism

The same input produces byte-identical output, every run.

This is not tidiness. Several of the checks below are comparisons, and a
comparison against something that moves is not a check. Map iteration is
deliberately unordered in Go, so any field derived from a map is sorted before
it is written, and the types that cross a stage boundary hold no maps at all.

Tested by running each stage five times over each fixture and comparing bytes,
and again at the command level by running the built binary twice and comparing
the files.

## Stage 1, the code graph

- Every fixture's graph validates against the embedded graph schema.
- Every fixture's graph is byte-identical across runs.
- A file that does not parse is recorded with its path, its line and the
  parser's message. It is never dropped in silence.

The last one carries the most weight, and will carry more when the tree-sitter
languages land. tree-sitter fails soft: it emits an ERROR node and carries on.
A graph missing a file looks exactly like a graph of a project that never had
it, and no later stage can restore what was never extracted. The diagnostics
block is the only thing that makes the difference visible.

- Exactly one node has no parent, and every other node reaches it by walking
  parents. A subtree hanging off a broken pointer would validate and could never
  be drawn.
- Every edge names two nodes that exist.

## Stage 2, the boundary

Stage 2 happens outside this binary, so what comes back is untrusted input.

`validate` is the only gate. `compose` and `render` assume a validated model
rather than re-checking one, and where compose does run the check it runs the
same one and says so. Two places deciding what is acceptable would be two
contracts.

The check has two halves:

- **Shape**, settled by the embedded JSON Schema: which fields exist, their
  types, which are required, and that nothing unexpected is present.
- **Meaning**, settled by code, because a schema cannot state it: ids are unique
  within a family section, every reference resolves, containment has no cycles,
  an activation does not end before it starts, and the families a model declares
  are the families it actually carries.

A model claiming sequence support without lifelines is refused here rather than
three stages later.

## Stage 3, composition

Per family, and per fixture:

- The document validates against the diagram schema for its family.
- The document is byte-identical across runs.
- The family's completeness rule holds, on the document and on every level
  within it.
- Nothing the model declared is absent from the output.
- The families the model declares and the families `compose` emits agree.
- At least one fixture is shaped so that the record path is actually taken. A
  document that never drops anything satisfies the accounting trivially.

### component

Every relationship the model proves is drawn, or recorded on the box it belongs
to with a reason, and **drawn + dropped == proven** holds exactly.

Proven is counted per level and summed, not counted once for the document. A
relationship between two components nested inside one parent is the business of
two pages: the page showing the parent, where it collapses to a loop and is
recorded on that box, and the page inside the parent, where it is drawn. Both
are true, and counting it on both is what lets a reader of the outer page see
that something inside that box was not shown.

Relationships come from two places. A **dependency** is stated by the model
outright. An **assembly** is derived: one component requires an interface that
another provides, which UML draws as the two halves of a connector meeting, and
which the model implies without writing down.

Also checked:

- Every component appears exactly once across every level, as a box or as a
  band.
- Every connection has both ends on the page that draws it.
- Every box has a cell of its own, inside its level's grid.
- Exactly one level has no parent. Every other is opened from a named box on its
  parent, and that box points back at it.

The only reason a relationship is dropped at this stage is `selfReference`: both
ends roll up to the same box, so drawing it would be a loop that says nothing. A
grid that cannot route a line is a geometric problem and belongs to stage 4,
which is where the rest of the drop reasons will appear.

### sequence

Not built yet. The rule, settled in advance: every message and activation in the
model appears on the diagram; ordering is preserved; no lifeline referenced by a
message is missing.

### state

Not built yet. The rule: every state and transition appears; every transition's
source and target resolve; initial and final states are present where the model
declares them.

### use case

Not built yet. The rule: every actor, use case and association appears; include
and extend resolve to declared use cases; nothing sits outside the system
boundary that the model placed inside it.

## Stage 4, rendering

Not built yet. The rules, settled in advance:

- The family's composition rules pass, checked by our own Go checker reading the
  emitted artifact rather than the renderer's in-memory state.
- Document to HTML is structurally lossless: every element in the document is
  present in the artifact, and the artifact introduces none the document does
  not account for.
- The SVG comparison is exact on element order, tag names, classes, text and
  every non-numeric attribute, with a 1e-6 tolerance on numeric attributes only.
- The renderer-to-viewer DOM contract is pinned by a generated test. A missing
  data attribute leaves the page drawing while interaction dies in silence, and
  no composition rule would notice, because those rules check the drawing rather
  than the attribute vocabulary.
- Output is byte-identical across runs.

Golden HTML byte comparison runs and reports, but never fails the build. Byte
equality has total sensitivity and almost no specificity: a good canary and a
bad specification. A difference is explained or fixed by a person, never
silenced by regenerating the golden file.

The 1e-6 tolerance is chosen because it is far below one device pixel and far
above accumulated double error at diagram scale.

## The release gate

`make verify` exiting zero is the only automated gate, and it is what authorises
a tag. It runs offline on a clean macOS machine with only Go and make installed.
Anything it needs that such a machine lacks is a defect, not a prerequisite.

A short human checklist sits beside it, in `docs/decisions.md`. Nothing on that
list may silently substitute for a test; an item that can be automated moves
into `make verify` rather than staying a habit.
