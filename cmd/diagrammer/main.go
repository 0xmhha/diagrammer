// Command diagrammer turns a source tree into UML diagrams, in four stages that
// each run on their own.
//
//	graph     stage 1   AST parse to a code graph
//	          stage 2   a plugin's skill analyses that graph with an LLM and
//	                    returns a UML codegraph; the binary has no subcommand
//	                    for it and never calls a model itself
//	validate  stage 2 boundary   accept or refuse the returned UML model
//	compose   stage 3   UML model to diagram-source documents
//	render    stage 4   document to a self-contained HTML page
//	serve               the same capabilities as a local MCP server
//
// No single command infers its stage from the shape of the file it is handed.
// Stage separation is a requirement, and a command that guesses makes the
// boundaries invisible exactly where they matter most.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
)

// version is set by the linker at release time; see the Makefile.
var version = "dev"

// errNotImplemented is returned by a subcommand that 0.1.0 has named but not
// yet built. It is distinct from an unknown command, so a person typing a real
// subcommand is told it is coming rather than that it does not exist.
var errNotImplemented = errors.New("not implemented yet")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "diagrammer:", err)
		os.Exit(1)
	}
}

// command is one subcommand. Adding one means adding a row to commands, which
// is the only place the surface is written down.
type command struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) error
}

func commands() []command {
	return []command{
		{"graph", "parse a source tree into a code graph", runGraph},
		{"validate", "check a UML codegraph against the stage-2 contract", runValidate},
		{"compose", "turn a UML codegraph into diagram-source documents", runCompose},
		{"render", "turn a diagram-source document into a self-contained page", notImplemented},
		{"serve", "expose the same capabilities as a local MCP server", notImplemented},
		{"version", "print the version and build details", runVersion},
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stdout)
		return nil
	}
	name := args[0]
	if name == "--version" {
		name = "version"
	}
	if name == "-h" || name == "--help" || name == "help" {
		usage(stdout)
		return nil
	}
	for _, c := range commands() {
		if c.name == name {
			return c.run(args[1:], stdout, stderr)
		}
	}
	usage(stderr)
	return fmt.Errorf("unknown command %q", name)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: diagrammer <command> [arguments]")
	fmt.Fprintln(w)
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-9s %s\n", c.name, c.summary)
	}
}

func notImplemented(_ []string, _, _ io.Writer) error {
	return errNotImplemented
}

// parseOperand splits one positional argument out of args, whichever side of it
// the flags are written on.
//
// Go's flag package stops at the first non-flag argument, so `graph . -o out`
// would leave -o unparsed. The command surface puts the operand first, so this
// parses what precedes it, takes it, and parses what follows.
func parseOperand(flags *flag.FlagSet, args []string, what string) (string, error) {
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return "", fmt.Errorf("%s takes %s, got none", flags.Name(), what)
	}
	operand := rest[0]
	if err := flags.Parse(rest[1:]); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("%s takes one %s, got %d", flags.Name(), what, flags.NArg()+1)
	}
	return operand, nil
}

func runVersion(args []string, stdout, _ io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("version takes no arguments, got %d", len(args))
	}
	fmt.Fprintf(stdout, "diagrammer %s (%s %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return nil
}
