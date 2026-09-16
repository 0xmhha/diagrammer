// Package goast reads Go source with go/ast and returns a stage-1 code graph.
//
// # Provenance
//
// The walk, the node-id scheme and the type and call resolution below are a
// copy of analyzers/go/goscan.go from the working tree this project was
// developed alongside, recognisable line by line. The output layer and the
// diagnostics block are new: the original counted parse errors, and this
// records each one with its path and line, because a count cannot be acted on.
//
// That file is the author's own rather than the reference project's. It does
// not exist on that project's origin/main and every commit touching it is the
// author's, so no attribution is owed. It is said here anyway: a reader of
// eleven hundred lines should not have to find a licensing document to learn
// that most of them came from somewhere else. See THIRD_PARTY_NOTICES.md and
// docs/licensing.md.
//
// Only the standard library is used. An analyzer that needed `go get` would
// break the self-contained binary this project is built around, and go/ast is
// always current with the compiler that builds it, so a Go 1.27 build
// understands Go 1.27 on the day it ships.
//
// That is also why tree-sitter is not used for Go, though it is used for the
// other languages. The tree-sitter Go grammar's method_declaration rule
// carries no type parameters, so a generic method does not parse; and
// tree-sitter fails soft, emitting an ERROR node and continuing, so the method
// would simply be missing from the graph with nothing to say it was ever
// there. No later stage can restore what was never extracted.
package goast
