package render

// The renderer-to-viewer contract.
//
// Every attribute the viewer reads is named here, and a generated test asserts
// that the viewer reads nothing else and that the renderer writes all of them.
// The reason is a failure mode no other check catches: leave one attribute out
// and the page still draws, perfectly and silently, while every interaction
// dies. The composition rules would not notice, because they judge the drawing
// and this is about the vocabulary underneath it.
//
// The list is deliberately short. It is ours, and it covers what 0.1.0 named —
// navigating between levels and following a box into the one below it — rather
// than a vocabulary inherited from a viewer built for a different document.
const (
	// attrFamily says which kind of diagram a page holds. It is read back by
	// the checker, because the rules a drawing is judged by differ per family
	// and a page that arrived on its own has nothing else to say which it is.
	attrFamily = "data-family"

	// attrRevision carries the commit the drawn source was checked out at.
	//
	// It is in neither contract on purpose. The viewer does not read it and the
	// checker does not judge it: it is for the person looking at the page, and
	// for anything that later wants to ask a finished page which code it was
	// made from without parsing prose.
	attrRevision = "data-revision"

	attrLevel      = "data-level"
	attrLevelTitle = "data-level-title"
	attrLevelLink  = "data-level-link"
	attrLevelBack  = "data-level-back"

	attrBoxID = "data-box-id"
	// attrBoxBounds carries a box's rectangle, written out rather than left to
	// be recovered from whatever shape was drawn.
	//
	// The same reasoning as data-composition-points: a final state is a ringed
	// circle and a use case an ellipse, so a reader working from the drawing
	// would have to understand every shape this renderer might choose. The
	// geometry the checker needs is stated instead.
	attrBoxBounds = "data-box-bounds"
	attrBoxOpens  = "data-opens"
	attrRegionID  = "data-region-id"

	attrEdgeID       = "data-edge-id"
	attrEdgeFrom     = "data-edge-from"
	attrEdgeTo       = "data-edge-to"
	attrEdgeFromSide = "data-edge-from-side"
	attrEdgeToSide   = "data-edge-to-side"
	attrEdgeLabelFor = "data-edge-label-for"

	// The sequence family's own furniture. A bar has to say which lifeline it
	// runs on and a frame what it is, because a rectangle on its own says
	// neither and the checker has only the artifact to go on.
	attrBarID     = "data-bar-id"
	attrBarBox    = "data-bar-box"
	attrFrameID   = "data-frame-id"
	attrFrameKind = "data-frame-kind"

	// attrCompositionPoints carries the routed polyline, unrounded.
	//
	// It is part of the artifact's contract rather than an implementation
	// detail: the composition checker reads it back out of the emitted document
	// to judge what was drawn, and the rules test distances against thresholds,
	// where a value sitting on a threshold can have its verdict flipped by
	// rounding.
	attrCompositionPoints = "data-composition-points"

	// attrLabelBounds carries the rectangle a connection's text occupies.
	//
	// It is written for the same reason as the points: the label rule measures
	// the text, and how wide text is depends on the font and on the size this
	// one label was shrunk to. Working it out again from the string would judge
	// a rectangle the page does not have.
	attrLabelBounds = "data-label-bounds"
)

// The artifact has three readers, and they read different things.
//
// The viewer moves between pages and needs to know which page is which and
// which box opens which. The composition checker judges the drawing and needs
// the geometry. The third is whoever wants to know where the page came from,
// which is neither of those and is usually a person. Listing them apart is what
// lets each side be held to the attributes it actually depends on: an attribute
// the viewer never touches is not a broken viewer contract, and geometry the
// viewer ignores is not dead weight.

// ViewerContract lists what the embedded viewer reads.
//
// The second group is what the highlight needs: which box the pointer is over,
// which lines touch it, and what those lines carry. A drawing says which things
// are joined by putting a line between them, and on a page of any size that is
// a question the reader has to answer by following the line. These let the page
// answer it.
func ViewerContract() []string {
	return []string{
		attrLevel, attrLevelLink, attrLevelBack, attrBoxOpens,
		attrBoxID, attrEdgeID, attrEdgeFrom, attrEdgeTo, attrEdgeLabelFor, attrBarBox,
	}
}

// CheckerContract lists what reading the artifact back depends on.
func CheckerContract() []string {
	return []string{
		attrFamily, attrLevel, attrLevelTitle, attrBoxID, attrBoxBounds, attrBoxOpens, attrRegionID,
		attrEdgeID, attrEdgeFrom, attrEdgeTo, attrEdgeFromSide, attrEdgeToSide,
		attrEdgeLabelFor, attrCompositionPoints, attrLabelBounds,
		attrBarID, attrBarBox, attrFrameID, attrFrameKind,
	}
}

// ProvenanceContract lists what the page says about where it came from rather
// than about the drawing.
//
// It is a list of its own because neither of the other readers touches it. The
// viewer shows the same thing whatever commit a page was made from, and the
// checker judges a drawing by its geometry and not by its history. The reader
// here is the person looking at the page, and anything that wants to ask a
// finished page which code it describes without reading prose out of it.
func ProvenanceContract() []string {
	return []string{attrRevision}
}

// DOMContract is everything the renderer must write, which is the union of what
// its readers need.
func DOMContract() []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{ViewerContract(), CheckerContract(), ProvenanceContract()} {
		for _, name := range list {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
