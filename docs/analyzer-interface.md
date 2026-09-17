# The analyzer interface

How a language gets read, and what adding one costs.

There is one analyzer today. The interface exists anyway, because an interface
discovered after the fact tends to describe whichever implementation came first,
and the whole claim being made here is that adding a language is an addition
rather than a redesign.

## The contract

`internal/analyze.Analyzer`. Three methods:

- **`Language()`** names what it reads.
- **`Extensions()`** lists the suffixes it claims, each with its leading dot.
  This is what lets a mixed tree be routed without asking every analyzer to walk
  all of it.
- **`Analyze(ctx, root, opts)`** walks the tree and returns a `graph.Graph`,
  including the group and package hierarchy above the declarations.

The contract is narrow deliberately. An analyzer decides what its own language
means; it does not decide what a graph is. Every one of them emits the same five
node kinds and two edge kinds, so nothing downstream ever has to know which
language it is looking at.

Analyzers are assembled into a `Registry` by the caller rather than registering
themselves into a package-level variable. A global register would make the set
of languages depend on which packages happened to be linked, which is exactly
the kind of thing that is discovered by pointing the program at a repository and
wondering why it returned almost nothing.

## What a new analyzer has to get right

The signature says none of this, and all of it matters.

**Diagnostics are the point, not the afterthought.** A file that will not parse
goes into `Diagnostics.ParseFailures` with its path, its line and the parser's
own message. It is never dropped in silence. This matters more for some parsers
than others: tree-sitter fails soft, emitting an ERROR node and carrying on, so
code can vanish from a graph that otherwise looks complete. A graph missing a
file and a graph of a project that never had one are the same document unless
somebody wrote down the difference.

**`FilesParsed` counts every file opened**, whether or not it parsed. The number
that succeeded is that minus the failures, and is deliberately not stored twice.

**Output must be byte-identical across runs.** Sort everything before returning
it. Map iteration is unordered in Go by design, so any field derived from a map
has to be sorted, and the graph types hold no maps for that reason.

**`exported` is stored rather than derived.** The rule differs by language: Go
reads the first letter, Python a leading underscore, JS/TS an export keyword.
Only the analyzer knows which rule applied, so downstream cannot recompute it.

**Doc comments are carried.** Stage 2 attributes meaning, and a doc comment is
meaning the author already wrote down. Leaving it out makes the model infer from
names what the source states outright.

**Cancellation is honoured.** A walk over a large tree is the one place this
program can be left running with nothing to show for it.

**Paths are relative to the analyzed root**, so a graph does not carry the
machine that produced it.

## The part that is not a parser problem

Adding a language is mostly not about parsing. The first thing to measure is how
its hierarchy interacts with the unfold window.

A level unfolds one generation at a time while it holds fewer than six members,
and stops before the next generation would exceed twenty-four. Go's hierarchy
has a file generation between the package and its declarations, and that extra
step lands inside the window. Python and JavaScript, as the reference
implementation read them, emit no file nodes: a thin package unfolds straight to
its functions, usually overshoots twenty-four, and so does not unfold at all.

A fix written against "that language lacks file nodes" fixes the wrong thing.
The mechanism is the window, not the node kind, and the answer is either a
synthetic intermediate level or different thresholds for that language. Which
one is a question for measurement on fixtures, not for taste. See
docs/thresholds.md.

## What this build reads

It depends on the build, and the program says which every time `graph` runs.

`CGO_ENABLED=0`, which is what `make build` produces and what the release gate
covers: **Go**, with `go/ast` from the standard library.

`CGO_ENABLED=1`, which is what `make build-polyglot` produces: Go, plus
**Python, Solidity, JavaScript and TypeScript** through tree-sitter. It needs a
C toolchain, which is why it is a second binary and a second gate rather than
the default. See docs/decisions.md.

Go keeps `go/ast` in both builds. tree-sitter's Go grammar carries no type
parameters on a method declaration, so a generic method does not parse, and it
fails soft, so the method would vanish with nothing to say it was there. Using a
grammar for Go would be a pure loss.

## Adding a fifth language

If it has a tree-sitter grammar, it is a row in the table in
`internal/analyze/treesitter/language.go`: which node types are types and which
are functions, which name an import, where the documentation lives, and how the
language decides what is public. The walk is shared.

If it does not, it is a new analyzer satisfying the interface above, and the
list in `internal/command/analyzers_cgo.go` gains a line.

Either way the first thing to measure is the unfold window, not the parser.
