# Third-party notices

diagrammer's own code is released under the MIT license in `LICENSE`. This file
records everything in this repository that came from somewhere else, and the
terms that came with it.

Every entry must name what was taken, where it came from, and under what
licence. An entry is added in the same commit that brings the material in, not
afterwards.

## Archify

diagrammer reimplements ideas first built in
[Archify](https://github.com/tt-a1i/archify), which is distributed under the
MIT license. Archify is a Node.js project; diagrammer is a separate Go program
and is not a fork.

Reimplementing behaviour observed in an MIT project creates no obligation on
its own. Copying material does. What follows is what was actually taken, and
what was deliberately not.

**Nothing of Archify's is copied into this repository.** That is a stronger
claim than "no rows below", so here is what it rests on.

### What was taken, and why it carries no obligation

`internal/analyze/goast/goast.go` is a copy of `analyzers/go/goscan.go` from the
working tree this project was developed alongside. Its walk, its node-id scheme
and its type and call resolution are recognisable line by line; the output layer
and the diagnostics are new.

That file is not Archify's. It does not exist on Archify's `origin/main`, and
every commit that has ever touched it is by this project's author. The same is
true of `pyscan.py`, `jsscan.mjs`, `codegraph.mjs` and
`renderers/architecture/layers.mjs`, none of which are upstream and all of which
are the author's own. `docs/licensing.md` records how that was checked and lists
them.

The file says so in its own header, so a reader does not have to find this page
to learn where it came from.

### What would have needed a row, and was avoided

Two things were settled early as copies and then were not copied, each for a
reason recorded in `docs/decisions.md`:

- **The viewer asset.** Round 3 decided to embed Archify's `assets/template.html`
  verbatim, 780 KB, which would have needed a row here and the copyright notice
  below shipped with every distribution. It was not embedded: 47 of the 195
  `data-*` attributes its viewer reads serve features this project has no
  equivalent of, and the rest assume a document format the design later
  replaced. The viewer here is this project's own, 155 lines.
- **The full-width character table.** A single regular expression of roughly 46
  ranges in `renderers/shared/utils.mjs`, with deliberate departures from the
  Unicode standard. Transcribing it exactly was only ever required to match that
  implementation's output byte for byte, and that comparison was removed from
  the contract. Character width here is derived from the Unicode standard
  through `golang.org/x/text/width`, which produces different widths and is the
  right basis for a renderer that is not imitating another one.

### What was learned rather than taken

The composition rules, the level-splitting and unfold rules, the thresholds and
the idea of recording every relationship a drawing could not hold were all
understood from Archify and written here from scratch. Behaviour, algorithms and
naming conventions are not copyrightable on their own, and `docs/licensing.md`
says so in more detail.

| Material | Taken from | Status |
|---|---|---|
| (none) | | |

The table is empty and the notice below is kept anyway, so that adding a row
later is a one-line change rather than a change that also has to remember to
bring a licence with it.

```
MIT License

Copyright (c) 2026 tt-a1i (Archify)
Copyright (c) 2025 Cocoon AI

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## diagram-design

The page's drawing rules and the frames a standalone SVG is delivered in were
taken from [diagram-design](https://github.com/cathrynlavery/diagram-design),
an editorial design system for diagrams distributed under the MIT license.
diagram-design is a skill that has a model draw HTML by hand; diagrammer draws
by program and took the rules, not the tool.

This section was added after the material it records, which the rule at the top
of this file says not to do. It is here now, and the lapse is noted rather than
tidied away.

### What was learned rather than taken

Hairline strokes, one accent reserved for what the reader is looking at, three
font roles by what a text is, no shadows, corners under ten, every coordinate
on a four-pixel grid, and a drawing that announces itself to a screen reader
with a role, a title and a description. These are rules, which are not
copyrightable, and each was written into `viewer.css` and `svg.go` from its
statement rather than from any file of theirs. `docs/decisions.md` records
which were taken and which were measured and left.

### What was taken

| Material | Taken from | Where it is | Status |
|---|---|---|---|
| Nine frame presets: the names `doc-inline`, `doc-wide`, `slide-16x9`, `slide-4x3`, `social-og`, `social-square`, `print-a4-landscape`, `print-letter-landscape`, `fit`, and their dimensions | `references/output-spec.md` | `internal/render/standalone.go` | Recorded. A table of standard slide and page sizes under short descriptive names is a statement of fact rather than a substantial portion of software, and the notice below is kept regardless. |

### What was not taken

No HTML, CSS, JavaScript, reference text, example, icon or font. The three
faces diagram-design loads from Google Fonts are not used, because a page here
opens from a file with no network; the three *roles* are taken and filled with
system font stacks. Its `self_check.py` was run against a page of ours for
information and is not in this repository or its gate.

```
MIT License

Copyright (c) 2025 Cathryn Lavery

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Go modules vendored into this repository

These are in the repository, under `vendor/`, as well as compiled into the
binary that ships. Both facts oblige attribution, and every one of their
licences requires the notice to travel with a copy.

They are vendored rather than fetched because the release gate is defined as
running on a clean machine with no network. `go build` would otherwise reach for
the module proxy, which is exactly the kind of prerequisite that definition
exists to forbid. Each module keeps its own `LICENSE` file where it sits.

Versions are the ones `go.mod` pins. All of these licences are permissive and
none of them restricts what this project may do; the obligation is attribution.

| Module | Version | Licence |
|---|---|---|
| github.com/santhosh-tekuri/jsonschema/v6 | v6.0.3 | Apache-2.0 |
| github.com/modelcontextprotocol/go-sdk | v1.8.0 | Apache-2.0 and MIT (see below) |
| github.com/google/jsonschema-go | v0.4.3 | MIT |
| github.com/segmentio/encoding | v0.5.4 | MIT |
| github.com/segmentio/asm | v1.1.3 | MIT |
| github.com/yosida95/uritemplate/v3 | v3.0.2 | BSD-3-Clause |
| golang.org/x/oauth2 | v0.35.0 | BSD-3-Clause |
| golang.org/x/sync | v0.20.0 | BSD-3-Clause |
| golang.org/x/sys | v0.41.0 | BSD-3-Clause |
| golang.org/x/text | v0.14.0 | BSD-3-Clause |
| golang.org/x/time | v0.15.0 | BSD-3-Clause |

Compiled into the four-language build only, and vendored so that gate runs
offline too. They are tree-sitter grammars, transpiled from C, and the runtime
that drives them.

| Module | Version | Licence |
|---|---|---|
| github.com/alexaandru/go-tree-sitter-bare | v1.11.0 | MIT |
| github.com/alexaandru/go-sitter-forest/python | v1.9.10 | MIT |
| github.com/alexaandru/go-sitter-forest/solidity | v1.9.3 | MIT |
| github.com/alexaandru/go-sitter-forest/javascript | v1.9.2 | MIT |
| github.com/alexaandru/go-sitter-forest/typescript | v1.9.4 | MIT |

Each vendored grammar directory carries one `LICENSE`: the MIT terms of
go-sitter-forest, naming Maxim Sukharev and Alex Ungur. Sukharev is the author
of `smacker/go-tree-sitter`, which this runtime is derived from, and the notice
is how that lineage is stated.

**The upstream tree-sitter grammar's own notice does not travel with it**, and
that is written down here rather than assumed either way. A grammar's `parser.c`
is generated from its `grammar.js` by the tree-sitter tool, and go-sitter-forest
licenses what it publishes under the terms above. Whether the generated parser
also carries the grammar author's terms is a question this project has not
answered and does not need to answer to ship: the rows above name what was
taken and under what licence it was received, which is the obligation. If a
grammar is ever read for its rules rather than linked as a dependency, that
question has to be answered first.

The MCP Go SDK is mid-transition from MIT to Apache-2.0. New contributions are
Apache-2.0; contributions whose authors have not consented to relicensing remain
MIT. Its `LICENSE` file states this, and both licences are satisfied by the row
above plus the copy shipped with the module.

Four of these arrive only because the MCP SDK supports transports this project
does not use: oauth2, uritemplate, sync and time are reached through its HTTP
and authorisation paths. They are compiled in and never executed on the stdio
transport the server runs on. They are listed anyway, because what ships is what
has to be attributed, not what runs.

This table is regenerated rather than remembered. `go list -deps ./cmd/diagrammer`
names every package the binary links, and anything new in that list belongs here
in the same commit that adds it. `make tidy` regenerates `vendor/` alongside it,
and `make verify` refuses to run against a stale one.

## No affiliation

diagrammer is not endorsed by or affiliated with Archify or its authors. Names
and trademarks remain the property of their owners.
