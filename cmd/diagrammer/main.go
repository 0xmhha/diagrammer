package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
)

// version is set by the linker at release time; see the Makefile.
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "diagrammer:", err)
		os.Exit(1)
	}
}

// subcommand is one entry in the CLI surface. It is named for what it is rather
// than "command", because the capabilities themselves live in the command
// package and confusing the two would hide which of them a change touches.
type subcommand struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) error
}

func commands() []subcommand {
	return []subcommand{
		{"graph", "parse a Go source tree into a code graph", runGraph},
		{"validate", "check a UML codegraph against the stage-2 contract", runValidate},
		{"compose", "turn a UML codegraph into diagram-source documents", runCompose},
		{"render", "turn a diagram-source document into a self-contained page", runRender},
		{"mermaid", "write a diagram-source document as Mermaid, one block per level", runMermaid},
		{"svg", "write a diagram-source document's drawings as SVG files, framed to a size", runSVG},
		{"instruct", "print the instruction stage 2 is performed from", runInstruct},
		{"serve", "expose the same capabilities as a local MCP server", runServe},
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
	say(w, "usage: diagrammer <command> [arguments]\n\n")
	for _, c := range commands() {
		say(w, "  %-9s %s\n", c.name, c.summary)
	}
}

// say writes a line of commentary.
//
// The write error is discarded, once, here, rather than at every call site.
// Commentary goes to a terminal or to nothing: a failed write to either is not
// something a caller can act on, and on a closed pipe the process is signalled
// before the error is ever returned.
//
// Output that is the product rather than commentary about it is written with
// Write, and that error is returned. deliverResult does exactly that for the
// documents, and runVersion for the version it was asked to print.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
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
	// The version is what the command was asked for rather than commentary
	// about something else, so a failure to write it is a failure of the
	// command.
	_, err := fmt.Fprintf(stdout, "diagrammer %s (%s %s/%s)\n",
		version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return err
}
