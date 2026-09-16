// Package analyze defines what a language analyzer is.
//
// There is one implementation today and the interface exists anyway. Adding a
// language should be an addition rather than a redesign, and the only way to
// know that is to have written down what the addition has to satisfy before
// there is a second one to compare against. An interface discovered after the
// fact tends to describe whichever implementation came first.
//
// See docs/analyzer-interface.md for what a new analyzer has to get right,
// including the part that is not obvious from the signature.
package analyze
