// Package docscheck holds the tests that keep the shipped documents honest.
//
// It exists because of a line in the release checklist: confirm the shipped
// documents match the code they describe. That was a human item, and the same
// checklist says an item that can be automated moves into `make verify` rather
// than staying a habit. Most of it can.
//
// The checks read the code rather than a second copy of the numbers. A test
// that restated a threshold would be a third place for it to be wrong.
//
// It has no code of its own on purpose. Putting these in one of the packages
// they check would make that package's tests fail for a reason that has nothing
// to do with it.
package docscheck
