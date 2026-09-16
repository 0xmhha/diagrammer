// Package render turns a stage-3 diagram source into a self-contained page.
//
// This is where geometry lives. Stage 3 decided which cell a box sits in; here
// that becomes a position and a size, connections become routed polylines, and
// the whole thing becomes SVG inside an HTML page that needs nothing else to
// open.
//
// A relationship that cannot be routed without breaking a composition rule is
// dropped and recorded on the box it belonged to, exactly as stage 3 records
// what a page could not hold. Routing every relationship on every diagram is a
// hard problem and nothing requires it; what is required is that nothing goes
// missing without a trace.
package render
