# Thresholds

Every number that decides what a page shows, and where it came from.

The point of writing them down is not to defend them. It is to record how they
were arrived at, so that changing one is done the same way rather than by taste.

## Status, stated plainly

**None of these have been measured in this repository.** They come from the
reference implementation, where they were arrived at by measuring real
repositories, and they are carried over because arriving at them again from
nothing would be worse than inheriting them.

That inheritance is a loan, not a proof. The measurements behind them were taken
on one private Go repository at one commit, with thresholds that moved three
times in a single day, and no record survives of the flags that produced the
figures. Anyone who re-measures should treat these as a starting point rather
than a baseline to preserve.

## In use

### `minBoxesPerLevel` = 6

Below this, a page is too thin to be worth opening, and unfolds a generation
instead.

A page of two or three boxes costs a reader a click and tells them almost
nothing. The cost of the click is fixed; the value of the page scales with what
is on it, so there is a floor below which opening it is a loss.

Used in `internal/compose/component.go`.

### `expandedMaxBoxes` = 24

The ceiling an unfold may not push a page past.

Unfolding trades a thin page for a fuller one, and past some point the fuller
page is unreadable. When the trade would cross this line the unfold is
abandoned and the thin page is kept: thin is a smaller loss than illegible.

Used in `internal/compose/component.go`.

## Not in use yet

Recorded because they are part of the same family of decisions and will be
needed when the layout work that uses them lands.

### `defaultMaxNodes` = 14

The point past which a page is split into several. Splitting an over-full page
is not implemented: the composer builds levels from the nesting the model chose
and does not yet break one apart.

### `splitTolerance` = 1.4

How far past the ceiling a page may go before splitting, so that a page of
fifteen is not broken into one of fourteen and one of one. Unused until
splitting exists.

## Thresholds that are not here

Two numbers a reader might expect are deliberately absent.

**A per-level connection ceiling.** A grid cannot route an unbounded number of
lines, so at some density a relationship has to be dropped for a geometric
reason. That reason belongs to stage 4, which is where routing happens; a
number invented here, before anything has been routed, would be a guess dressed
as a rule.

**A resolution rate.** How many references an analyzer manages to resolve varies
by language and is a property of the parser rather than a defect in a document.
It is reported in the graph's diagnostics and is not gated on.

## The one number that is defended

The `1e-6` tolerance in the structural SVG comparison, when stage 4 lands, is
not inherited. It is chosen because it sits far below one device pixel and far
above accumulated double error at diagram scale, which is the window a
comparison has to live in to be both meaningful and stable. See
`docs/invariants.md`.
