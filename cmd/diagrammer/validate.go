package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// runValidate is the gate at the stage-2 boundary.
//
// It is the only place a returned UML model is checked. compose and render
// assume a validated model rather than re-validating in silence, so a model
// that gets past here is trusted by everything downstream.
func runValidate(args []string, stdout, _ io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("validate takes one codegraph.json path, got %d arguments", len(args))
	}
	path := args[0]

	doc, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	model, err := validate.Codegraph(path, doc)
	if err != nil {
		return err
	}

	// Saying which families were accepted is worth a line: it is the promise
	// compose will be held to, and a model that declares fewer families than
	// its author expected is easier to notice here than three stages later.
	fmt.Fprintf(stdout, "%s: valid, %d %s (%s)\n",
		path,
		len(model.Families),
		plural(len(model.Families), "family", "families"),
		joinFamilies(model.Families),
	)
	return nil
}

func joinFamilies(families []uml.Family) string {
	names := make([]string, len(families))
	for i, f := range families {
		names[i] = string(f)
	}
	return strings.Join(names, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
