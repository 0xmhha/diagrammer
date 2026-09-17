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

## What the page does on its own

The drawing is one half of the artifact and the viewer is the other. It moves
between levels, and it answers "what is this one joined to" when the pointer
rests on a box: everything not joined to it fades, and the lines that are stay.

Faded rather than hidden, and nothing moves. A reader who has just found the box
they wanted should not have the page rearrange itself underneath them, and what
is faded is still there to be read.

Two things have to hold for that walk to arrive anywhere, and no drawing rule
cares about either. A line naming a box that is not on its page draws perfectly
and leaves the highlight dark, in silence; so does a label naming a line that
was left out. Both are checked over the emitted page, for every family.

The attributes the walk reads are declared in `ViewerContract`, and a test holds
that list and the viewer's own source to each other in both directions.

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

### What is drawn without its text

A label with nowhere to go loses its text and keeps its line, and the page says
which names it could not write.

The two losses are not the same size. A reader of an unnamed line can still see
that two things are joined; a reader of no line cannot. Refusing the
relationship was the earlier behaviour and it charged the larger price for the
smaller problem.

It is kept out of the accounting deliberately. **drawn + dropped == proven**
counts relationships, and one of these is a relationship the reader can see. The
record is separate, on the page beside the drawing and in what the command
prints, so that nothing goes missing quietly either way.

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
- The row a box sits on is its depth in the dependency graph, so a component
  diagram reads downward: whatever nothing depends on is at the top and what it
  rests on is beneath it. A cycle has no depth, and the edge that closes one is
  left out of the reckoning rather than followed.
- The column a box takes within its row follows the barycentre of its
  neighbours in the rows above and below, so that boxes joined by a line sit
  near each other and the run between them stays short. A long run is the thing
  that crosses.
- A band owns its own columns. Two bands sharing a column would each have to
  reach across the other's boxes, and the two frames would overlap, which says
  the two groups overlap.
- Exactly one level has no parent. Every other is opened from a named box on its
  parent, and that box points back at it.

The only reason a relationship is dropped at this stage is `selfReference`: both
ends roll up to the same box, so drawing it would be a loop that says nothing. A
grid that cannot route a line is a geometric problem and belongs to stage 4,
which is where the rest of the drop reasons will appear.

### shared by every family

Four things are wrong in the same way whatever the family, and are checked once
rather than four times: a connection with an end nobody can see, two boxes in
one cell, a page nobody can reach, and an accounting that does not describe the
document it is attached to.

A fifth is shared in form but not in content. Each family names the relationship
kinds it can mean, and a connection carrying any other is refused: a dependency
in a state machine and a transition in a use case diagram are both nonsense, and
one vocabulary serving every family only works if each of them refuses the rest
of it.

### sequence

Every message and activation in the model appears on the diagram, ordering is
preserved, and no lifeline a message refers to is missing.

Ordering is the one thing a sequence diagram cannot get wrong quietly. The same
messages in a different order describe a different interaction, so the row each
message lands on is checked against the model's own order rather than assumed.
That order is the model's array order and nothing else; a second field
expressing it would be a second thing that can disagree.

Nothing is dropped here, and the rule asserts that rather than assuming it. A
grid cannot route every relationship, which is why the component family needs a
record; a ladder has a rung for every message and no reason to refuse one. The
day something does start dropping messages, it will be a failure rather than a
silence.

### state

Every state and transition appears, every transition's source and target
resolve, and initial and final states are present where the model declares them.

A composite state is drawn as both a box and a band. That is not a duplicate: in
UML the composite state's own box is the frame its substates sit inside, and a
transition has to be able to land on it. The rule permits the repeat for this
family and refuses it for the component family, where two boxes for one
component would say there are two of it.

The kind of a state is carried as its stereotype and checked against the model,
because a final state drawn as an ordinary box is a diagram that reads wrongly
while every count still adds up.

Nothing is dropped.

### use case

Every actor, use case and association appears, include and extend resolve to
declared use cases, and nothing sits outside the system boundary that the model
placed inside it.

The boundary is the diagram's claim about what the system is responsible for, so
where a box sits is not a presentation detail here. A use case drawn outside the
band, or an actor drawn inside it, reverses that claim while every count still
adds up, and is refused.

Include and extend run between use cases. One ending on an actor would satisfy
the endpoint rule, because an actor is a box, and still be nonsense, so it is
checked separately.

Nothing is dropped.

## Stage 4, rendering

Built for all four families.

- The composition rules pass, checked by reading the routed polylines back out
  of the emitted document rather than off the renderer's working state. A
  renderer checking itself proves only that it agrees with itself.
- Document to HTML is structurally lossless: every box in the document is in the
  drawing, every relationship is either drawn or recorded, never both and never
  neither, and the drawing holds nothing the document did not declare.
- The renderer-to-viewer DOM contract is pinned by a test that extracts the
  attributes from both sides and compares them. A missing data attribute leaves
  the page drawing while interaction dies in silence, and no composition rule
  would notice, because those rules check the drawing rather than the vocabulary
  underneath it.
- Output is byte-identical across runs.
- The page is self-contained: no network, no second file, nothing but itself.

### The accounting crosses the boundary

Stage 4's proven is stage 3's drawn. What the pages handed over is what the
drawing was responsible for showing, and **drawn + dropped == proven** holds
here as it does there.

A relationship the geometry cannot hold is dropped and recorded on the box it
left, with the rule that refused it. Routing every relationship on every diagram
is a hard problem and nothing requires one; what is required is that nothing
goes missing without a trace. The record is written beside the drawing in the
page itself, because the reader who needs it is the one looking at the diagram.

### The composition rules, per family

A ladder is not judged like a grid. Messages cross lifelines constantly and that
is how a sequence diagram works; a crossing rule applied there would refuse
every one ever drawn. So each family names the rules it is held to, and a
drawing whose family has none is refused rather than quietly judged by
another's.

Component, state and use case share one set, because all three are boxes joined
by routed lines. Seven rules, each with a drawing that breaks it:

- **endpoint-side** — a route leaves and arrives on the edges it claims, and its
  ends sit on the boxes it names. A line that says it leaves the right edge and
  travels left crosses its own box on the way out.
- **pass-through** — no route enters a box that is not one of its ends.
- **separation** — no route runs closer than 8px to a box it is not attached to,
  where it starts to read as joined to it.
- **label-clearance** — a connection's text keeps clear of every line but its
  own, of every other label, and of every box. The rule measures the rectangle
  the text occupies, not the point it hangs from: a string forty characters long
  has a centre that clears everything and two ends that clear nothing, and
  measuring the centre passed a page where eight of eight labels lay across
  another line.
- **crossing** — no proper intersection between two routes that share no end.
  Two lines arriving at the same box are not a crossing; a reader expects that.
  This is the one rule that condemns a pair rather than a route. Every other
  rule points at one line and says it is wrong on its own; a crossing says at
  least one of two has to go and leaves the choice open. The renderer settles
  the others first and then takes out the fewest routes that leave no crossing
  behind, which on a real project is far fewer than taking out whichever route
  of each pair sorts first. `docs/thresholds.md` has the measurements.
- **minimum-segment** — no run between bends shorter than 16px, below which a
  turn reads as a kink.
- **label-clearance** — a connection's text stays 10px clear of every route but
  its own, so it cannot attach itself to the wrong relationship.
- **border-run** — no route travels more than 24px along a band's border instead
  of crossing it, which would read as the band having a side the diagram never
  meant.

**sequence** has four of its own, each with a drawing that breaks it: every
message has a rung of its own, because two on one rung destroys the ordering
that is the diagram's whole content; an arrow's ends sit on the lifelines it
names; an execution bar sits on the lifeline it names and has height; and
nothing reaches outside the page.

Two of the grid rules hold by construction rather than by luck. Routes travel only in the
channels between cells, so pass-through cannot happen; and the side a route
leaves on is derived from where the two boxes sit, so endpoint-side agrees
unless something else has gone wrong. They are checked anyway, because a rule
that holds by construction today holds by accident tomorrow.

### Shape is meaning

UML gives each thing a shape and the shape is half of what it says. A ringed
circle is an end, an ellipse is something the system does for somebody, a stick
figure is the somebody, a dashed vertical line is time passing. Drawing them all
as rectangles would be perfectly legible and would say the wrong thing.

So a box carries its stereotype into the drawing, and the drawing carries its
rectangle in an attribute of its own rather than leaving it to be recovered from
whichever shape was chosen. A reader of the artifact needs the geometry; making
it understand every shape the renderer might pick would mean it fails silently
on the next one.

### What is not checked

The structural SVG comparison, with its 1e-6 tolerance on numeric attributes,
compares one drawing against another. There is nothing yet to compare against:
it becomes meaningful at the first release, when a change can be held against
what shipped. The tolerance and its justification are recorded in
docs/thresholds.md so the number is not invented later under pressure.

Golden HTML byte comparison is not run for the same reason. When it is, it
reports and never fails the build: byte equality has total sensitivity and
almost no specificity, which makes it a good canary and a bad specification. A
difference is explained or fixed by a person, never silenced by regenerating
the golden file.

## Every rule by name

A rule fires with its name, and this is where a reader looks it up. The names
are a contract too: a failure saying `label-clearance` should lead somewhere,
not just somewhere in a source tree.

Held over every document, whatever its family:

| Name | What it holds |
|---|---|
| `accounting` | drawn plus dropped equals proven, per level and for the document |
| `record-matches-accounting` | the counts describe what is actually there, not what was claimed |
| `connection-endpoints` | a connection has both ends on the page that draws it |
| `box-placement` | every box has a cell of its own, inside its level's grid |
| `drilldown` | exactly one level has no parent, and every other is opened from a named box that points back at it |
| `completeness` | nothing the model declared is absent, and nothing appears that it did not declare |
| `relationship-kind` | a connection carries a kind its family can mean |

Held per family, where the shared set says nothing:

| Name | Family | What it holds |
|---|---|---|
| `no-drops` | sequence, state, use case | nothing was dropped, because these have nowhere to drop one |
| `message-ordering` | sequence | rungs follow the order the model wrote, and an activation does not end before it starts |
| `system-boundary` | use case | what the model placed inside the system is drawn inside it, and include and extend run between use cases |

Held over a drawing, for the three families that are boxes joined by lines:

| Name | What it holds |
|---|---|
| `endpoint-side` | a route leaves and arrives on the edges it claims, and its ends sit on the boxes it names |
| `pass-through` | no route enters a box that is not one of its ends |
| `separation` | no route runs closer than 8px to a box it is not attached to |
| `crossing` | no proper intersection between routes sharing no end |
| `minimum-segment` | no run between bends shorter than 16px |
| `label-clearance` | a connection's text stays 10px clear of every route but its own |
| `border-run` | no route travels more than 24px along a band's border |

Held over a drawn ladder:

| Name | What it holds |
|---|---|
| `rung-distinct` | every message has a rung of its own |
| `message-span` | an arrow's ends sit on the lifelines it names |
| `activation-on-lifeline` | an execution bar sits on the lifeline it names, and has height |
| `inside-canvas` | nothing reaches outside the page |

A test reads these names out of the code and fails when one of them is missing
here, so the table cannot fall behind what the checkers actually do.

## The release gate

`make verify` exiting zero is the only automated gate, and it is what authorises
a tag. It runs offline on a clean macOS machine with only Go and make installed.
Anything it needs that such a machine lacks is a defect, not a prerequisite.

A short human checklist sits beside it, in `docs/decisions.md`. Nothing on that
list may silently substitute for a test; an item that can be automated moves
into `make verify` rather than staying a habit.
