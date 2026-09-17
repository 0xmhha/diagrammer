//go:build cgo

package command

import (
	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/analyze/goast"
	"github.com/0xmhha/diagrammer/internal/analyze/treesitter"
)

// analyzers is what this build can read: Go with the standard library, and
// Python, Solidity and JS/TS through tree-sitter.
//
// Go keeps its own analyzer here rather than moving to a grammar. go/ast
// refuses a file it cannot parse and is always current with the compiler;
// tree-sitter's Go grammar carries no type parameters on a method declaration
// and fails soft, so a generic method would vanish with nothing to say it was
// there. Using a grammar for Go would be a pure loss.
func analyzers() *analyze.Registry {
	return analyze.NewRegistry(append(
		[]analyze.Analyzer{goast.Analyzer{}},
		treesitter.Analyzers()...,
	)...)
}
