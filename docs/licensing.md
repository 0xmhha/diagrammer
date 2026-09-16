# Licensing rules for writing code in this repository

diagrammer is MIT (`LICENSE`). It reimplements ideas first built in Archify,
which is also MIT. That makes almost everything permitted and two things
mandatory. This page states them so they are settled before code is written,
not argued about afterwards.

## What needs no attribution

Reading Archify to understand *what* it does, then writing Go that does the
same thing, creates no obligation. Behaviour, algorithms, file formats and
naming conventions are not copyrightable on their own. The composition rules,
the level-splitting rule, the relationship recording rule and the region-band
layout can all be reimplemented freely.

Restating a rule in our own words in a design document is also fine.

## What needs attribution

Copying material is different. If any of the following enters the repository,
the same commit must add a row to `THIRD_PARTY_NOTICES.md` naming the file and
its origin, and the Archify copyright notice must ship with the distribution:

- JSON schema files taken as-is
- Viewer HTML, CSS or JavaScript
- SVG templates or generated asset data
- Any file that is a translation of an Archify source file close enough that
  the original is recognisable line by line

A Go file that follows the same algorithm but was typed here is not a copy. A
Go file produced by mechanically transliterating an `.mjs` file is.

## Which Archify files are actually Archify's

The working tree this project reads from is a local branch that is 20 commits
ahead of its origin, and a good deal of what looks like Archify is this
project's author's own work committed there. Those files carry no obligation at
all, and treating them as third-party would add notices for material nobody
else wrote.

Checked by asking two questions of each file: does it exist on `origin/main`,
and who has ever committed to it.

**The author's own. Copy freely; no notices row.**

| File | What it is |
|---|---|
| `analyzers/go/goscan.go` | the Go analyzer |
| `analyzers/python/pyscan.py` | the Python analyzer |
| `analyzers/javascript/jsscan.mjs` | the JS/TS analyzer |
| `analyzers/codegraph.mjs` | the converter, and every rule deciding what to draw |
| `renderers/architecture/layers.mjs` | the drill-down level composition |

None of these exist on `origin/main`, and every commit touching them is by the
author of this project.

**Third-party. Reimplementing is free; copying takes a notices row.**

`renderers/shared/geometry.mjs`, `renderers/shared/utils.mjs`,
`renderers/shared/text-fit.mjs`, `renderers/architecture/render-architecture.mjs`,
`renderers/sequence/render-sequence.mjs`,
`renderers/lifecycle/render-lifecycle.mjs`, `assets/template.html`,
`schemas/*.schema.json` and `scripts/check-render-output.mjs`.

Several of these have the author among their contributors, which does not make
the file theirs: a file with several authors is jointly held, and the others did
not agree to anything beyond the MIT terms.

One of them deserves naming ahead of time. The full-width character table in
`renderers/shared/utils.mjs` is a single regular expression of roughly 46
ranges with deliberate departures from the Unicode standard, and
`golang.org/x/text/width` does not reproduce it. Transcribing it exactly, which
is what correctness requires, is copying. When that lands it takes a notices
row; deriving an equivalent table from Unicode data instead is the only way to
avoid one, and it will not produce the same widths.

## Where the record lives

Three places, each for a different reader:

- **`THIRD_PARTY_NOTICES.md`** is for somebody who received a copy and wants to
  know what is in it. It says what was taken, what carries no obligation and
  why, and what would have needed a row and was deliberately not copied.
- **A file header** is for somebody reading that file. `goast.go` says in its
  own package comment that most of it came from elsewhere, because a reader of
  eleven hundred lines should not have to find this page to learn that.
- **This page** is for somebody deciding whether a new thing may be brought in.

The three say the same thing at different distances. If they ever disagree, the
notices file is what ships with a copy and therefore wins.

## What to do when unsure

Write the origin down. A line in `THIRD_PARTY_NOTICES.md` costs nothing and is
worth far more than a later argument about whether something was a copy. If a
file is borrowed, say so in the file header as well:

```go
// Ported from Archify (MIT, https://github.com/tt-a1i/archify).
// See THIRD_PARTY_NOTICES.md.
```

## What is never allowed

- Presenting diagrammer as an Archify release, fork, or official port
- Using the Archify name or marks in a way that implies endorsement
- Removing or altering a copyright notice on material that carries one
