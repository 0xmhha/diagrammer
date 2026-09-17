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
- **`channelSlack` = 3.** How many lanes a channel holds beyond the routes that
  want it. Counting the routes and stopping there gives every route a lane and
  the wrong one: a route is not looking for any free lane but for one that
  clears what else is in the channel, and with none to spare the only one left
  may be exactly the one that crosses. Measured over seven models, the share
  drawn goes 168, 171, 172, 173, 173 for slack 0 to 4, so it is set at the point
  it stops paying.
- **`channelMaxLanes` = 24.** The most lanes a channel is widened to hold. The
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

**Why 24.** The first answer here was 12, justified as sitting above every
demand measured. That justification was wrong, and it was wrong because only the
committed fixtures had been measured. Running the analyser over fifteen
repositories in the neighbouring tree and grouping each one the way archify's
`analyze` groups, package by package, gives this:

| model | relationships on the level | widest horizontal channel |
|---|---|---|
| ego-lite | 8 | 3 |
| mythril | 9 | 5 |
| archify | 25 | 10 |
| diagrammer | 28 | 14 |
| OpenMMO | 63 | 35 |
| cmux | 531 | 172 |
| claude-code-reference | 2,842 | 505 |

There is no number that sits above all of it. Demand grows with the model and
has no bound, so a ceiling chosen to clear every case is a ceiling that does not
exist. A ceiling of 12 also refused this repository's own model, which is the
plainest possible sign the number was picked from too little.

What the ceiling has to do instead is not refuse a model worth drawing. The
table has a gap in it: everything that fits on one page at all asks for under
fifteen lanes, and everything above that is dense all over rather than
hub-shaped. Widening a channel does not rescue a page with five hundred
relationships on it; nothing does, and the honest answer there is that stage 2
put too much on one level.

24 sits above that band and is also `expandedMaxBoxes`, the most boxes the
composer will unfold onto a page. On such a page one box can collect at most
23 others, so a hub the composer itself created can never exceed the ceiling.
That tie is partial and worth saying so: `expandedMaxBoxes` bounds unfolded
levels, not the overview, whose size comes from how many top-level components
stage 2 declared.

`testdata/codegraph/hub-overflow.codegraph.json` puts thirty components on one,
past both the ceiling and the page size the composer contemplates, and is drawn
38 of 60 with twenty-two refused for want of a lane.
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

### What a crossing costs, and who pays it

A crossing is the only composition rule that condemns two routes at once. The
renderer used to take out every route the checker named, and the checker named
whichever route of each pair sorted first, so the set that went was a cover of
the conflicts by accident rather than by choice.

It now settles every other rule first, which removes some crossings for free,
and then takes out the fewest routes that leave none behind. Finding the
smallest such set is a hard problem in general; these graphs hold tens of routes
and the greedy answer, take out whichever route crosses the most others and look
again, is what a person would do by eye.

Two other changes came out of the same measurements. A detour now leaves by the
side that faces its target, where before it always left downward and a route to
a row above climbed back past its own row and travelled the channel above the
target: the height of the page for a relationship between two rows. And every
route now takes its position on a box edge from one allocator rather than two,
because the two handed out the same first position and a route arriving at a
box's top was drawn along the same line as a route leaving it. No rule saw that.
Two parallel lines on one x do not properly intersect and the pair shared an
endpoint anyway; what noticed was a label sitting on a line that ran underneath
it the whole way.

Measured on models of real projects, grouped package by package:

| model | before | after |
|---|---|---|
| archify | 9/25 | 17/25 |
| diagrammer | 18/28 | 21/28 |
| mythril | 7/9 | 8/9 |
| OpenMMO | 20/63 | 28/63 |
| ego-lite | 8/8 | 8/8 |

The committed fixtures do not move, because each of them produces at most one
crossing and with one crossing every answer removes one route.
`testdata/codegraph/tangle.codegraph.json` was written for that reason: twelve
services each reaching one and five places along the row, which produces
thirty-one crossings at once. Like hub-overflow it is deliberately worse than an
ordinary model and is not in the table below.

### Text is measured, and the page is laid out in the direction it is read

Three things were wrong together, and comparing this tool's pages with
archify's on the same source tree is what showed them.

**Text on a line was never measured.** A box's label has been measured, shrunk
and cut since the first drawing; a line's label was emitted at whatever length
it came in at. On a sequence diagram whose lifelines sit 220px apart, a message
named `Merge(graphs) -> 536 nodes, 477 edges` is 232px wide and lay across three
of them. The rule that was supposed to catch it measured the label as the point
it hangs from, and the centre of a long string clears everything. A label is now
fitted to the room its run has, at `edgeLabelSize` = 11px down to `labelMinSize`
before it is cut, and the rule reads the rectangle. Counted on the pages this
repository draws of itself, text on text went from four occurrences to none.

**Text sat across a line that ran down the page.** Centring it either way put
half of it in the channel on each side, so a transition between two states in
one column had its name written over the routes either side. Text now sits above
a line that runs across the page and beside one that runs down it.

**Nothing in the arrangement said which way anything depended on anything.** A
page filled a square grid in walk order, so two boxes joined by a line could land
three rows and two columns apart, and a relationship between them became a
detour across the page. Rows are now dependency depth and columns belong to the
bands, which is what makes a page read downward and a region frame a block
rather than a scatter. This is what archify's `rankMembers` does, arrived at the
same way: by looking at a page of ours beside a page of theirs.

### A route picks the way that crosses the least

Laying the page out by dependency depth made the rows mean something and left
the columns and the lanes still chosen blind.

Three things were open to a route and none of them was chosen. Which column a
box took within its row was decided by a walk that never looked at the
neighbouring rows. Which lane of a channel a route travelled in was whichever
came next. Which vertical channel a detour climbed was always the one left of
its target. So a line ran 560px along a row channel on this repository's own
diagram, and four separate detours climbed straight through it.

All three now look at something. Columns are ordered by the barycentre
heuristic, each row sorted by the average position of its neighbours in the row
above and below, swept both ways and the arrangement with the fewest crossings
kept. Lanes and the vertical channel are chosen together, by trying every
combination still free and taking the one that crosses the least of what is
already drawn. Ties go to what the code chose before, so a page with nothing in
the way is routed exactly as it was.

Order is what makes looking worth doing. Routes are drawn in order of how much
choice they have, least first: a straight or stacked route has one shape and one
position on each box side, a route between neighbouring rows has one channel,
and a detour has a lane in two channels and any vertical it likes. By the time a
detour picks, what it can see is everything that could not have gone anywhere
else. Within a tier the shortest go first, for the same reason.

The barycentre sweep moves a box to where its neighbours average out, which is
a good guess and only a guess. Two boxes whose neighbours average to the same
place, and a box pulled two ways at once, are both cases an average cannot
answer, so neighbouring boxes are then swapped whenever the swap crosses fewer
lines. That is the question the average was approximating, asked directly.

**Counted in columns, not in list order.** Columns are handed out within a row
and a band, so two boxes far apart in the order can be neighbours on the page.
The first version of this counted crossings by place in the order, which
measured something the page does not have: every swap looked like no
improvement and the step returned what it was given. On this repository's
component diagram it left 29 pairs ordered one way at the top and the other at
the bottom; counting in columns brings it to 19.

Nineteen is what this graph costs in this layering. No arrangement of the
columns removes them, because `command` sits on one row and reaches five boxes
spread across another, and every one of those lines crosses the row between.

Measured on models of real projects, grouped package by package, plus this
repository's own four-family model:

| model | before | after |
|---|---|---|
| archify | 17/25 | 22/25 |
| diagrammer, packages | 21/28 | 21/28 |
| diagrammer, four families | 16/25 | 20/25 |
| OpenMMO | 28/63 | 38/63 |
| mythril | 8/9 | 6/9 |
| ego-lite | 8/8 | 8/8 |
| **total** | **92/158** | **115/158** |

mythril is the one that lost, and it is left in the table rather than out of it.
A heuristic that improves the total is not a heuristic that improves every case,
and a table showing only the cases that moved the right way would be an argument
rather than a measurement.

### A box as wide as what it points at, tried three ways

A box that fans out to many others is the hardest thing on the page: on this
repository's diagram `command` sits on one row and reaches five boxes across
another, and its lines have to fan sideways before they can descend. Sideways is
where lines cross.

The obvious answer is to make such a box wide enough to sit over the block it
points at, so each line leaves above where it is going and drops almost
straight down. It was tried three ways and measured each time, over six models.

**As wide as it reaches.** 117 relationships drawn became 100. A box stretched
over the block it points at lies across the vertical channels beside it, and
every route that used one of those channels has to go round. Teaching the router
to refuse a way through a box did not recover it: 100 became 99.

**As wide as its own lines need,** by the same `2*stub + n*laneGap` a channel
uses: no change at all. A box with ten lines wants 180px and its column is
already 168px wide, so almost nothing grows.

The reason the idea does not pay here is worth writing down, because it is the
same reason dummy nodes are the textbook answer. A vertical channel runs the
whole height of the page. A box wide enough to cover its dependents lies across
the channels its neighbours need, and there is nowhere else for them to go. For
the width to help, a route crossing several rows would have to be free to change
column at each one, which is what a dummy node at every rank gives it.

What does pay is spreading the row out. A row with two boxes in a band five
columns wide used to put them side by side on the left and leave three columns
empty; they are now spread across the band, so each sits over what it points at
without having to grow at all. That alone took the six models from 115 drawn to
117.

**Lines leave in the order they go.** Positions along a box edge used to be
handed out in the order routes asked for them, so a route to the far right could
be given the leftmost position and had to cross everything else leaving that
side before it started. They are now laid along the edge in the order of where
they go, decided for the whole side before any of it is drawn, with the straight
routes already sitting where they had to.

That one is a trade and is recorded as one. It shortens every page measured by
two to three per cent of line, and it costs two relationships of a hundred and
fifty-eight: the order it forces creates a crossing in two places where the
arbitrary order happened not to. Shorter, ordered fan-outs were judged worth
more than two lines out of that many, and the numbers are here so the judgement
can be revisited rather than rediscovered.

### Room to spare in a channel

A channel sized for exactly the routes that want it gives every route a lane and
gives some of them the wrong one.

This is what a page of this repository looked like with none to spare. The
channel between two rows was wanted by four routes, so it was made four lanes
wide, and one of those lanes had another route descending across it. The route
that arrived last had one lane free, it was that one, and the relationship was
recorded instead of drawn. A fifth lane would have held it.

So a channel is made `channelSlack` lanes wider than its demand. Measured over
seven models the drawn count goes 168, 171, 172, 173, 173 as the slack goes 0 to
4; three is where it stops paying. The page grows by 42px per channel that
carries anything, which on this repository's state diagram is 514px wide against
488.

With that and the model arranged as a hierarchy, three of this repository's four
diagrams draw everything they were given.

### A use case page is not a layered one, and the count now knows it

The arrangement step counts the lines that would cross before there is any
geometry to measure, and for a long time it counted only one shape of line: one
between two rows, which meets another between the same rows when their ends are
ordered one way at the top and the other at the bottom.

A line between two boxes on **one** row is nothing like that. It drops into the
channel below the row, runs along it and comes back, so what it occupies is an
interval of columns. Two of those cross when their intervals interleave, and not
when one sits inside the other, which the lanes keep apart. A line climbing from
the row below crosses such a run when it lands strictly inside the interval.

Counting the second kind with the first kind's test was an error rather than an
approximation, and it hid where it did most damage. A use case page puts every
association an actor makes on that actor's row, so almost every line on it is
the shape that was not modelled. Arranging such a page to improve the count made
it worse: this repository's own use case page went from eight of ten drawn to
seven.

With the shape modelled, the same page draws nine, and **nine is the most any
arrangement of its columns reaches.** That is not an estimate: all two hundred
and forty arrangements were tried.

| | before | after |
|---|---|---|
| usecase / diagrammer's own model | 8/10 | 9/10 |
| every committed fixture | 99/100 | **100/100** |
| six real projects | 127/159 | 127/159 |

**The tenth line is reachable and is not worth what it costs.** Searching row
assignments as well as column ones finds an arrangement that draws all ten, and
it works by moving a use case off the row of the actor that reaches it. That row
is the page's meaning: it is what makes a use case diagram read as "this actor
does these things". A line is worth less than that, and the page records the one
it could not draw.

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
