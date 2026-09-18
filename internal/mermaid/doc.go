// Package mermaid writes a stage-3 document as Mermaid text.
//
// It is a second way out of stage 3, beside the renderer. The renderer draws a
// page and is bound by a grid: a relationship the grid cannot hold is recorded
// on the page rather than drawn. Mermaid lays itself out, so this carries every
// relationship the document proved, including the ones the page had to record.
//
// What comes out is one Markdown file with one fenced block per level, which is
// the shape both a README and a redrawing tool such as diagram-design already
// read. Nothing here is a drawing: no coordinate, size or colour survives.
// Structure, relationships, nesting and order do, and the accounting says how
// many of each.
//
// Ids are rewritten. Stage 3 ids are authored to be unique and readable, with
// dots and colons in them, and Mermaid gives those characters meaning. Every
// box gets an id Mermaid accepts, chosen deterministically so two runs over one
// document produce one text.
package mermaid
