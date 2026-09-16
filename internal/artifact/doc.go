// Package artifact holds the drawn form of one page, and reads it back out of
// an emitted document.
//
// It exists so the composition rules can judge what was actually produced
// rather than what the renderer meant to produce. A rule run against the
// renderer's own working state proves only that the renderer agrees with
// itself; run against the artifact, it proves the drawing is sound.
package artifact
