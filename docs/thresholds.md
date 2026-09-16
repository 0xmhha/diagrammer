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

### The drawing's own numbers

These decide whether a route may be drawn, and every one of them has a test
that breaks it. They were chosen here rather than inherited, so they are
defended rather than merely recorded.

- **`minSegment` = 16px.** The shortest run between two bends that still reads
  as a deliberate turn. Below it the bend looks like a wobble in the line, and a
  reader stops trusting that the route means anything.
- **`separation` = 8px.** How far a route keeps from a box it is not attached
  to. Closer, and at ordinary zoom the line and the box border merge into one
  stroke, which says the two are connected when they are not.
- **`LabelClearance` = 10px.** How far a connection's text stays from another
  route. A label is read as belonging to the nearest line, so this is the
  distance at which "nearest" stops being ambiguous. It is larger than
  separation because text has height of its own.
- **`borderRun` = 24px.** How far a route may travel alongside a band's border
  before it reads as tracing the border rather than crossing it. Shorter runs
  are the unavoidable consequence of a route passing close by.
- **`channelX` = 112px, `channelY` = 96px, `laneGap` = 14px, `stub` = 20px.**
  A channel holds `(size - 2*stub) / laneGap` lanes, and a `stub` of clearance at
  each edge is what keeps the turn into the outermost lane longer than
  `minSegment`. These four are one decision rather than four: changing any of
  them without the others makes the router produce routes its own rules refuse.
- **`channelMaxLanes` = 12.** The most lanes a channel is widened to hold. The
  two channel sizes above are now the smallest a channel gets rather than the
  size it always is; a channel more routes want is widened to `2*stub + n*laneGap`
  until it reaches this. See the hub section below for where the number came
  from.

The last point but one is worth stating plainly, because it is the trap. The
router and the rules are two halves of one design. A channel too narrow for its
lanes does not draw a worse diagram; it draws nothing, and every relationship
lands in the record instead.

### Channels follow demand, up to `channelMaxLanes`

One size for every channel is one size for the average, and comparing two real
projects is what showed where the average stops being a guide.

The diagrammer model is layered: what depends on what runs mostly in one
direction and no component collects many dependents. Its busiest component has
two. archify is hub-and-spoke: `renderers/shared` has nine dependents and a
`Geometry` interface has six. Nine routes head for one box, so nine of them want
the same channel, and a channel of 96px holds `(96 - 40) / 14` = four. The other
five were recorded rather than drawn, and the neighbouring channels sat empty.

So each channel is now sized from the routes that will use it. The count reads
the cells stage 3 assigned, never a pixel, which is what lets it run before the
pixels exist; `channelsWanted` in `route.go` is the one place that knows which
channels a route wants, and both the sizing and the router read it.

| | before | after |
|---|---|---|
| component / hub (fixture, 8 dependents) | 10/16 | 16/16 |
| component / archify, all levels | 13/18 | 15/18 |
| component / diagrammer | 14/14 | 14/14 |

Every drop that said "no lane left" is gone. What archify still loses is three
crossings, which is a different cause and not addressed here.

**Why 12 and not more.** The widening has to stop somewhere, or one
over-connected box turns a page into mostly empty channel. The ceiling sits
above every demand that has actually been measured: archify's busiest channel
asks for six lanes, and the hub fixture, which was written to be the bad case,
asks for eight. Nothing observed is refused by the ceiling; it is there for a
model nobody has produced yet.

That leaves a ceiling nothing hits, which is a rule that may as well not exist.
`testdata/codegraph/hub-overflow.codegraph.json` puts sixteen components on one
and is drawn 22 of 32, with ten refused for want of a lane.
`TestTheChannelCeilingRefusesInTheOpen` is what proves the limit still fires. It
is deliberately past the limit and so is not in the drawn-ratio table, which
asks a different question: whether ordinary models are drawn well.

### `drawnFloor` = 0.80

The share of a model's relationships a drawing must actually show.

This one is measured rather than inherited, and it is the only number here with
a before and after.

The accounting invariant says nothing about this. A page that draws one
relationship and records nine is perfectly consistent and perfectly useless, so
the ratio is measured separately and held above a floor.

**A floor, not an equality.** The rules that decide what to draw are the
contract; the ones that decide where to put things are free to improve. Freezing
the ratio would turn every layout improvement into a failing test, which is how
a tuning knob becomes an API.

What the layout scored before the placement work, and after:

| | before | after |
|---|---|---|
| state / diagrammer | 22.2% | 88.9% |
| usecase / diagrammer | 57.1% | 85.7% |
| state / order-service | 57.1% | 85.7% |
| component / order-service | 75.0% | 100% |
| usecase / order-service | 80.0% | 100% |
| component / diagrammer | 85.7% | 100% |
| component / nested-platform | 100% | 100% |
| sequence, both fixtures | 100% | 100% |

Three changes account for it, in the order they were made and measured:

1. **Placement follows the relationships.** Boxes used to be laid out in sorted
   id order, which ignores the lines entirely: two things that talk to each
   other constantly could land at opposite corners, and the route between them
   then crossed the whole page. Arranging connected things next to each other
   was the single largest cause.
2. **A label goes where there is room.** It used to go to the middle of its
   route's longest run and stay there; if anything passed close to that one
   point, the relationship was refused for want of a few pixels on a line with
   plenty of other places to write on.
3. **Boxes one above the other spread across their facing edges.** A route
   between them runs straight down, so a lane taken in the channel moved
   nothing: two relationships between the same pair came out as one line drawn
   twice.

The floor is 0.80 because the worst case after the work is 0.857, and a floor
should sit below what the code achieves rather than at it — a floor equal to the
current score fails on the next fixture that is merely a little harder. It is
not a target: three of the nine cases are below 100% and each has a concrete
reason recorded in its page.

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

**A per-level connection ceiling.** There is no such number, and there does not
need to be one. A channel hands out lanes until it has none left, and the route
that asks next is dropped and recorded. The limit is a consequence of the
geometry rather than a figure someone picked.

**A resolution rate.** How many references an analyzer manages to resolve varies
by language and is a property of the parser rather than a defect in a document.
It is reported in the graph's diagnostics and is not gated on.

## The one number that is defended

The `1e-6` tolerance in the structural SVG comparison, when stage 4 lands, is
not inherited. It is chosen because it sits far below one device pixel and far
above accumulated double error at diagram scale, which is the window a
comparison has to live in to be both meaningful and stable. See
`docs/invariants.md`.
