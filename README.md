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
2. A plugin's skill analyses that graph with an LLM and returns a
   **codegraph.json** expressed as a UML model. The binary has no subcommand for
   this and never calls a model itself. It hands the graph out and takes the
   model back.
3. **compose** turns that model into diagram-source documents, one per family:
   component, sequence, state and use case.
4. **render** turns a document into a self-contained HTML page: positions,
   routed lines and an embedded viewer for moving between levels. A relationship
   the geometry cannot hold is recorded on the page beside the drawing rather
   than dropped in silence.

`validate` guards the boundary between stages 2 and 3, and `serve` exposes the
same capabilities as a local MCP server so any plugin can drive them.

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

**Two builds, one source.** `make build` produces a binary that reads Go and nothing else, needs no C toolchain, and is what the release gate covers.
`make build-polyglot` produces one that also reads Python, Solidity and JS/TS
through tree-sitter, and needs a C compiler because tree-sitter is a C library
and Go links C through cgo.

Which one you have is printed every time `graph` runs, so nobody discovers the
scope by pointing the program at a repository and wondering why the graph came
back nearly empty.

Everything else works end to end for all four families in either build:
`graph`, `validate`, `compose`, `render` and `serve`.

## Build

macOS is the supported target today.

```
make build           # bin/diagrammer, reading Go
make build-polyglot  # bin/diagrammer-polyglot, reading four languages
make check           # fmt, vet, lint, test — before a commit
make verify          # the release gate: fmt-check, vet, test, fixtures
make verify-cgo      # the second gate, for the four-language build
make help            # every target
```

`make verify` is the only automated gate. It runs offline on a clean machine
with nothing but Go and make installed; anything it needs that such a machine
lacks is a defect rather than a prerequisite.

Linux builds will come later, on a native runner rather than by cross
compiling, so that the binary that ships is the binary that was tested.

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
