// Package validate is the gate at the stage-2 boundary.
//
// Stage 2 happens outside this binary, so a returned UML model is untrusted
// input and is checked here before any later stage touches it. compose and
// render may assume a validated model, and say so, rather than re-validating
// in silence.
//
// The check has two halves. The embedded JSON Schema settles shape: which
// fields exist, their types, which are required, and that nothing unexpected
// is present. What a schema cannot express is settled here: that ids are
// unique, that every reference resolves, and that the families a model claims
// to support are the families it actually carries. A model declaring sequence
// support without lifelines is refused at this boundary rather than three
// stages later.
package validate
