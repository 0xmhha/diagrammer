//go:build !cgo

package command

import (
	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/analyze/goast"
)

// analyzers is what this build can read: Go, and nothing else.
//
// This is the build the release gate runs and the one that needs no C
// toolchain. The same source with cgo enabled reads four languages; which one a
// person has is reported by `graph` on every run, so nobody has to guess.
func analyzers() *analyze.Registry {
	return analyze.NewRegistry(goast.Analyzer{})
}
