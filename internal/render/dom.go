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
	attrLevel      = "data-level"
	attrLevelTitle = "data-level-title"
	attrLevelLink  = "data-level-link"
	attrLevelBack  = "data-level-back"

	attrBoxID    = "data-box-id"
	attrBoxOpens = "data-opens"
	attrRegionID = "data-region-id"

	attrEdgeID       = "data-edge-id"
	attrEdgeFrom     = "data-edge-from"
	attrEdgeTo       = "data-edge-to"
	attrEdgeFromSide = "data-edge-from-side"
	attrEdgeToSide   = "data-edge-to-side"
	attrEdgeLabelFor = "data-edge-label-for"

	// attrCompositionPoints carries the routed polyline, unrounded.
	//
	// It is part of the artifact's contract rather than an implementation
	// detail: the composition checker reads it back out of the emitted document
	// to judge what was drawn, and the rules test distances against thresholds,
	// where a value sitting on a threshold can have its verdict flipped by
	// rounding.
	attrCompositionPoints = "data-composition-points"
)

// The artifact has two readers, and they read different things.
//
// The viewer moves between pages and needs to know which page is which and
// which box opens which. The composition checker judges the drawing and needs
// the geometry. Listing them apart is what lets each side be held to the
// attributes it actually depends on: an attribute the viewer never touches is
// not a broken viewer contract, and geometry the viewer ignores is not dead
// weight.

// ViewerContract lists what the embedded viewer reads.
func ViewerContract() []string {
	return []string{attrLevel, attrLevelLink, attrLevelBack, attrBoxOpens}
}

// CheckerContract lists what reading the artifact back depends on.
func CheckerContract() []string {
	return []string{
		attrLevel, attrLevelTitle, attrBoxID, attrBoxOpens, attrRegionID,
		attrEdgeID, attrEdgeFrom, attrEdgeTo, attrEdgeFromSide, attrEdgeToSide,
		attrEdgeLabelFor, attrCompositionPoints,
	}
}

// DOMContract is everything the renderer must write, which is the union of what
// its two readers need.
func DOMContract() []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{ViewerContract(), CheckerContract()} {
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
