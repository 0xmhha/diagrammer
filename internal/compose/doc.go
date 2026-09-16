// Package compose turns a stage-2 UML model into stage-3 diagram sources.
//
// It decides what appears on which page and which relationships each page can
// show. It does not decide pixels: a box carries the cell it sits in, and a
// connection carries its ends. Where a box sits is a layout decision and
// belongs here; how large it is and which way a line bends are rendering
// decisions and belong to stage 4.
package compose
