# diagrammer

Reads a source tree and writes UML diagrams of it.

The work is split into four stages that each run on their own, because one of
them does not happen inside this program.

1. **graph** parses the source with an AST parser and emits a code graph:
   packages, files, types and functions, with the imports and calls it can
   prove, plus the doc comments their authors wrote. Structure and raw
   references only; nothing is interpreted here. Go is read with `go/ast` from
   the standard library, which refuses a file it cannot parse; the other
   languages with tree-sitter, which does not, so every tree is asked whether
   it parsed cleanly and a file that did not is recorded with its line.

   It also records which commit the tree was checked out at, read out of `.git`
   rather than by running git, and every stage after it repeats that record
   unchanged. This is the only stage that can know it, because it is the only
   one that reads the source.
2. A plugin's skill analyses that graph with an LLM and returns a
   **codegraph.json** expressed as a UML model. The binary does not perform this
   stage and never calls a model itself. It hands the graph out and takes the
   model back.

   What it does hand out is the instruction. `instruct` prints what a skill
   performs this stage from: what a code graph holds, the schema a UML model
   must satisfy, what the gate checks beyond that schema, and how to choose what
   to say. It is built from the same embedded schemas the gate uses, so the
   instruction given to a model and the check applied to what it returns cannot
   disagree.
3. **compose** turns that model into diagram-source documents, one per family:
   component, sequence, state and use case.
4. **render** turns a document into a self-contained HTML page: positions,
   routed lines and an embedded viewer that moves between levels and, when the
   pointer rests on a box, fades everything that box is not joined to. The
   page is drawn to a handful of editorial rules, each held by a test:
   hairlines, one accent and only under the pointer, three font roles, no
   shadows, and a drawing that announces itself to a screen reader. A
   relationship the geometry cannot hold is recorded on the page beside the
   drawing rather than dropped in silence, and the commit the drawing was made
   from is named in the header, so a reader knows which version of the code they
   are looking at. It says what was checked out; it is not a claim that nothing
   was uncommitted, and a tree that was not a checkout produces a page that says
   nothing rather than one that guesses.

`validate` guards the boundary between stages 2 and 3, and `serve` exposes the
same capabilities as a local MCP server so any plugin can drive them.

## Taking a diagram somewhere else

`render` is one way out of stage 3. `mermaid` is the other:

```
diagrammer mermaid out/component.diagram.json -o component.md
```

One Markdown file, one fenced Mermaid block per level, from the same document
the page was drawn from. That is the shape a README, Notion or Obsidian embeds
without a build step, and the shape a redrawing tool such as
[diagram-design](https://github.com/cathrynlavery/diagram-design) reads with its
Mermaid importer, so a diagram this program proved can be redrawn in a design
of somebody else's choosing.

The text is not bound by a grid, so it carries every relationship the document
proved, including the ones the page had to record rather than draw, and says
so beside them. The one exception is a recorded message in a sequence diagram:
order is its meaning and a recorded message has none, so it is named in a
comment rather than placed. The command's summary counts all of this.

A use case diagram has no Mermaid grammar of its own and is written as a
flowchart, with actors as stadiums and include and extend as dashed arrows
carrying their stereotype. That is an approximation and the text says so.

`svg` is the third way out, for wherever a page cannot go:

```
diagrammer svg out/component.diagram.json -o out/svg -size slide-16x9
diagrammer render out/component.diagram.json -o overview.html -level overview
```

One file per level, the same drawing the page holds, with its styles resolved so
no page is needed around it and its title and description carried for a screen
reader. `-size` frames it for a slide, a document, a social card or a printed
page; the default, `fit`, is the drawing's own size. A frame smaller than the
drawing scales it down, which the page never does, and the command says how far
and what the smallest label came to, because that is the one thing about the
file a reader cannot see.

`-level` on `render` and `svg` draws one level on its own: the overview alone
is a summary, and a page deep in the tree alone is a detail. Both writers get
the level as a document of its own, so a box that opened a page not present
opens nothing rather than pointing at a page the reader will never find.

`make diagram` writes the Markdown beside every page and the SVG files under
`svg/`.

## How much of the tree the drawing describes

Everything a model returns is judgement. A name is what it chose to call
something, a description is prose, and the nesting is a reading of the code.
None of that can be held to the tree it came from.

One thing can. A component records the graph nodes it stands for, and a node id
is in the graph or it is not:

```
diagrammer validate model.codegraph.json -graph graph.json
coverage: 15659 of 15659 nodes (100%), every area accounted for
```

Naming a package accounts for everything beneath it, so this is a handful of ids
on each component rather than a transcription of the tree.

An id the graph does not have is refused, because it is wrong in a way a program
can see. An area no component stood for is reported and nothing more: a model is
a map rather than a census, and leaving things out is often the right call. What
it may not do is leave them out without saying so. A model that records no ids
is reported as not having said, which is not the same as having covered nothing.

`make diagram` runs this after every model it accepts.

Neither surface has a capability the other lacks, and that is arranged to be
checkable rather than merely intended: both are built from one registry of
operations, and tests assert they cover the same set, that every argument a
request takes is reachable from the command line, and that the tool schema a
plugin reads is complete.

UML is load-bearing rather than decorative. A component diagram carries provided
and required interfaces, ports and dependencies; sequence carries lifelines,
messages, activations and fragments; state carries transitions with trigger,
guard and effect; use case carries actors, a system boundary, include and
extend. That vocabulary is what stage 2 is held to.

## Status

Early, and honest about it.

**Two builds, one source.** `make build` produces a binary that
reads Go and nothing else, needs no C toolchain, and is what the release gate
covers. `make build-polyglot` produces one that also reads Python, Solidity and
JS/TS through tree-sitter, and needs a C compiler because tree-sitter is a C
library and Go links C through cgo.

Which one you have is printed every time `graph` runs, so nobody discovers the
scope by pointing the program at a repository and wondering why the graph came
back nearly empty.

Everything else works end to end for all four families in either build:
`graph`, `validate`, `compose`, `render`, `mermaid`, `svg`, `instruct` and `serve`.

## Build

macOS is the supported target today.

```
make build           # bin/diagrammer, reading Go
make build-polyglot  # bin/diagrammer-polyglot, reading four languages
make check           # fmt, vet, lint, test — before a commit
make verify          # the release gate: fmt-check, vet, test, fixtures
make verify-cgo      # the second gate, for the four-language build
make dist            # archives for a mac that cannot build it
make size            # what the second build costs in bytes
make bench           # what it costs in time, per megabyte read
make help            # every target
```

`make size` and `make bench` exist because the figures for that build were once
quoted from the project that publishes the runtime rather than taken here. They
are taken now, and they are taken again by running those two rather than by
being remembered: 2.63 MB of binary and a parser somewhere between a third and
half again slower than `go/ast`. `docs/thresholds.md` has the table and the
conditions.

`make verify` is the only automated gate. It runs offline on a clean machine
with nothing but Go and make installed; anything it needs that such a machine
lacks is a defect rather than a prerequisite.

## Drawing something, in one command

From a working copy, `make diagram` runs the stages this program owns and stops
where it does not:

```
make diagram SRC=../some/project          # graph and instruction, then stops
make diagram SRC=../some/project AI=1     # calls a model, and draws
make diagram SRC=../some/project MODEL=m.codegraph.json
```

The first stops because stage 2 has not happened, and says which two files to
hand to a skill and what to run next. It reports that as a failure, because
nothing was drawn.

`AI=1` fills the gap by calling a model, and is the only one of the three that
goes from a path to pages in one command. It is opt-in and always will be: it
spends money, and it hands the graph, which carries every doc comment in the
tree, to whoever runs that model. `AI_CMD` is the command, so pointing it at
something else changes nothing here. Whatever comes back goes through
`validate` before anything is drawn.

A graph small enough to send is sent. One too big is not: the model is pointed
at the file and reads it with its own tools, so nothing large enters a prompt
and no size limit applies. A 9.2 MB graph of 15,660 nodes draws in about three
minutes that way. Sending is kept for everything that fits because it takes
seconds rather than minutes; reading is how the thing that used to be
impossible became possible, not a better way to do what already worked.

It is a make target rather than a subcommand on purpose. A subcommand that
drove a model would put stage 2 back inside the binary, and keeping it out is
the decision the whole pipeline is shaped by. A Makefile is glue, and glue is
allowed to know about a model. What a plugin drives is `serve`, which needs no
glue at all.

## Giving it to a machine that cannot build it

`make install` puts the binary on `PATH` through `GOBIN`, and needs a Go
toolchain on the machine that will run it. `make dist` is for one that has none:
an archive per mac architecture, each holding both builds, the licences and
[docs/install.md](docs/install.md), with a `SHA256SUMS` beside them.

Both architectures are cross-compiled, which is not a retreat from the rule that
nothing ships until it has been run. `make dist` unpacks every archive it makes
into a directory that is not this one, with an environment holding nothing but a
`PATH`, and runs the whole pipeline out of it with both binaries, the Intel ones
through Rosetta. What nothing here can run is a Linux binary, which is why
`make linux` still refuses and waits on a native runner.

Packaging found a defect rather than arranging one. clang takes the minimum
macOS from the machine doing the building, so the four-language build was
declaring it needed the builder's own macOS while the Go-only build beside it
went back to 12.0 — a fault that cannot show on the machine that produced it.
Both now declare 12.0, `make dist` reads that number back out of the binaries it
packed, and `docs/install.md` says plainly that it is a declaration rather than
something anyone has run on a mac that old.

The binaries are signed ad-hoc, which is not notarisation: Gatekeeper still
kills a build that arrives carrying a quarantine flag, and `docs/install.md`
carries the one command that fixes it, having been watched doing so.

## Driving it from a plugin

`diagrammer serve` speaks MCP over stdio. Point a client at the binary:

```json
{ "command": "/path/to/diagrammer", "args": ["serve", "-root", "/path/to/work"] }
```

**Every path argument is confined to one directory**, and the default is the
directory the server was started in. A path outside it is refused with the root
named, and only the person who started the server can widen it. `-root /` is how
you say you meant everywhere.

That is not there for the plugin's sake. Stage 1 hands a model the doc comments
of a repository it did not write, and the next tool call takes a path that names
a file to overwrite. The distance between those two is short enough to be worth
closing, and the command line is left alone, because a person typing a path can
already write anywhere their shell can.

Three things a caller can ask for before it does any work, which is the whole
of what it needs to know:

- **The server's instructions**, sent at `initialize`: what the four stages are,
  which order they go in, and that the caller performs the second one itself.
  A list of tools cannot say that, and a client that never learns it will use
  the four one at a time without knowing what it is holding.
- **The `stage-2` prompt**, which is the instruction that stage is performed
  from. `instruct` returns the same text as a tool for a client without prompts.
- **The three schemas as resources**, each under the identifier it declares as
  its own `$id`, for a caller that wants one on its own.

The instructions also say the arguments are confined, so a model is told the
rule rather than discovering it by being refused.

Then `graph`, your own model, `validate`, `compose`, `render`. Every path
argument is a path on the machine the server runs on.

## A worked example of stage 2

`instruct` tells a model how to write one. `testdata/codegraph/diagrammer-layered.codegraph.json`
is one written that way: this repository, read from its own code graph, with
seven components on the top page grouped by the stage of the pipeline each
belongs to.

`testdata/codegraph/diagrammer.codegraph.json` beside it is an earlier model of
the same repository, shallower. Both are kept, and the difference between them
is what the instruction is asking for:

```
diagrammer compose testdata/codegraph/diagrammer-layered.codegraph.json -o out
diagrammer render out/component.diagram.json -o out/component.html
```

Five pages and twenty-six relationships, against three pages and fourteen.
Both draw everything they state; the deeper one is able to state more, because
a page that opens is a page you can put less on.

## The schemas are the contract

Three JSON Schema files define the stage boundaries, and they are embedded in
the binary: the stage-1 code graph, the stage-2 UML model, and the stage-3
diagram source. Go types are checked against them by a test that fails the build on
divergence, not the other way round.

The reason is stage 2. It runs outside this program, so whatever a plugin's
skill works to has to be readable without Go, and a hand-transcribed copy would
drift from its original without anyone noticing until the output was wrong.
Where a schema and a document under `docs/` disagree, the schema wins and the
documentation is what gets corrected.

## Documents

- [docs/invariants.md](docs/invariants.md) — the correctness contract: what
  every stage must prove, and what each family means by completeness.
- [docs/thresholds.md](docs/thresholds.md) — every number that decides what a
  page shows, and where it came from.
- [docs/analyzer-interface.md](docs/analyzer-interface.md) — how a language gets
  read, and what adding one actually costs.
- [docs/decisions.md](docs/decisions.md) — what the design settled on, what was
  withdrawn along the way, and what is still open. Read this before changing
  anything structural.
- [docs/install.md](docs/install.md) — what a recipient of a packaged build
  needs: which binary reads what, the minimum macOS, and what macOS does to a
  download.
- [docs/licensing.md](docs/licensing.md) — what may be borrowed and what must be
  attributed.

## Relationship to Archify

The ideas here were first built in
[Archify](https://github.com/tt-a1i/archify), an MIT-licensed Node.js project.
diagrammer is a separate Go program, not a fork, and is not endorsed by or
affiliated with Archify or its authors.

What that permits and what it requires is written down in
[docs/licensing.md](docs/licensing.md), and anything actually borrowed is
recorded in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## License

MIT. See [LICENSE](LICENSE).
