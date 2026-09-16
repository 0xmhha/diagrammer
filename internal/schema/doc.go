// Package schema holds the JSON Schema files that define diagrammer's stage
// boundaries, and compiles them for use at run time.
//
// The schemas are the contract, and the Go types in the sibling packages are
// checked against them rather than the other way round. Stage 2 is performed
// outside this binary by a plugin's skill, so whatever that skill works to has
// to be machine-readable without Go; a hand-transcribed copy would drift from
// its original without anyone noticing until the output was wrong. The same
// argument covers any plugin driving the MCP server: it can read a schema file
// and cannot read a Go type.
//
// The files are embedded rather than read from disk, which is what keeps the
// binary self-contained and `make verify` offline.
//
// See docs/decisions.md for the decision and its reasoning.
package schema
