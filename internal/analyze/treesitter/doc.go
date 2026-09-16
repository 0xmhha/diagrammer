// Package treesitter reads Python, Solidity and JS/TS into a stage-1 code graph.
//
// Every file here but this one is behind a cgo build tag, so a CGO_ENABLED=0
// build compiles an empty package and reads Go only. This file carries no tag
// so that the package always has something to build, and so that `go build
// ./...` on a machine without a C toolchain does not fail on a package whose
// files were all excluded.
//
// # Why cgo
//
// tree-sitter is a C library and Go links C through cgo and nothing else.
// Building the runtime here rather than taking someone's binding changes who
// compiled it, not whether cgo is involved.
//
// The release gate is defined as a clean machine with only Go and make, so it
// runs the cgo-free build and reads Go only. A build that reads four languages
// is a different binary from the same source, and `graph` says which one it is
// every time it runs. See docs/analyzer-interface.md.
//
// # Soft failure, made loud
//
// tree-sitter does not refuse a file it cannot parse. It emits an ERROR node
// and carries on, which is how code goes missing from a graph that looks
// complete — the reason this project does not use it for Go, where the standard
// library gives a parser that says no.
//
// It can be asked, though. Every tree is checked with HasError, and a file that
// parsed with errors is recorded in the diagnostics with the position of the
// first one. That turns a silent gap into a reported one, which is the whole of
// what the contract asks for.
package treesitter
