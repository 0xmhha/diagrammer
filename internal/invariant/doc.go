// Package invariant holds the completeness rules each diagram family must obey.
//
// The general form is the same for all of them: every element of the model is
// either rendered or accounted for with a reason. What that means differs by
// family, and the rules are written per family rather than shared, because the
// component family's "drawn, or recorded on the box" is specific to a grid that
// cannot show everything at once. A sequence diagram does not drop messages the
// way a grid drops relationships.
//
// The rules live here rather than inside compose so that stage 4 checks the
// same ones over what it actually emitted. A rule enforced only by the code
// that produces the document proves nothing about the document.
//
// See docs/invariants.md for the contract these implement.
package invariant
