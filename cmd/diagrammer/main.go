// Command diagrammer reads a source tree and writes diagrams of it.
//
// The analyzers build one code graph from the source; each emitter turns that
// graph into a diagram document. Keeping the two apart is what lets a single
// analysis produce an architecture view, a sequence view and a state view.
package main

import (
	"fmt"
	"os"
	"runtime"
)

// version is set by the linker at release time; see the Makefile.
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "diagrammer:", err)
		os.Exit(1)
	}
}

func run(args []string, out *os.File) error {
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Fprintf(out, "diagrammer %s (%s %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return nil
	}
	fmt.Fprintln(out, "usage: diagrammer version")
	return nil
}
