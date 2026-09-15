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
