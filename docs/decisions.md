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
    diagrammer instruct [-o prompt.md]            what stage 2 is performed from
    diagrammer serve                              the same, as a local MCP server
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

Every subcommand reports failures on stderr with a non-zero exit status and names
the offending input path where one exists.

`instruct` was added after 0.1.1 and is the one capability that reads nothing.
Round 14 said the stage-2 skill prompt is built from the embedded schemas so
that the instruction given to a model and the check applied to what it returns
come from one file. Nothing built it: for two releases the only way to learn
what stage 2 must return was to find the schema in the source, which is a
requirement on whoever drives the pipeline that this program was supposed to
meet. It is a capability rather than a document in the repository for the same
reason the schemas are embedded rather than read from disk.

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

So the viewer is written here, small, and covers what 0.1.0 named: level
navigation, which is the drill-down capability round 9 listed. The DOM contract
requirement from round 15 stands unchanged in intent — a missing data attribute
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

Nothing on this list may silently substitute for a test. An item that can be
automated moves into `make verify` rather than staying a habit.

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
  kind. The mechanism is the unfold window: a level unfolds one generation at a
  time while it holds fewer than 6 members, and stops before the next generation
  would exceed 24. Go's hierarchy has an extra file generation that lands inside
  that window; a language without one jumps from package straight to functions
  and usually overshoots. A fix written against "JS lacks file nodes" fixes the
  wrong thing.
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

**The soft failure is answered rather than accepted.** tree-sitter does not
refuse a file it cannot parse: it emits an ERROR node and carries on, which is
why this project does not use it for Go, where the standard library gives a
parser that says no. It can be asked, though. Every tree is checked with
HasError and a file that parsed with errors is recorded with the position of the
first one, so a silent gap becomes a reported one. A fixture with a deliberately
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

- Whether source evidence, which needs git, is in 0.1.0.
- Which architectures the binary targets. macOS is built and tested; `make
  linux` refuses on purpose, because a cross-compiled binary is not the binary
  that was tested and the native runner to do it properly does not exist yet.

  What a plugin needs once it has the binary is no longer open. The server says
  what it is for at `initialize`, offers the stage-2 instruction as a prompt,
  and serves each schema as a resource under its own `$id`, so a client learns
  the pipeline from the server rather than from a document beside it. Getting
  the binary onto the machine is `make install` today, and packaging it for one
  it was not built on is the part that remains.
- Error policy and exit codes for parse failures, levels that cannot be laid out,
  and output path collisions.
- The measured speed and binary cost of the tree-sitter runtime. The published
  figures, roughly 3.9x slower than native C and roughly 20 MB of growth, are
  that project's own and have never been taken here. This is the only number
  either document carries that was inherited rather than measured, which is what
  `docs/thresholds.md` exists to forbid. It is owed now that a shipped build
  links the runtime, and the result may still reopen the choice: the Go-only
  build is unaffected either way, so what is at stake is whether the second one
  is worth its size.
- How the unfold window behaves for a language without a file generation. To be
  decided by measurement on the fixtures rather than assumed.
