// Package diagram holds the Go form of a stage-3 diagram source.
//
// These types mirror schemas/diagram.schema.json, which is the contract. They
// are checked against it by TestSchemaBinding; when the two disagree the schema
// is right and this file is what gets corrected.
//
// A document here is laid out but not drawn. Boxes carry the cell they sit in
// rather than a pixel position, and connections carry their endpoints rather
// than a route: where a box sits is a layout decision, while how large it is
// and which way a line bends are rendering decisions that belong to stage 4.
package diagram
