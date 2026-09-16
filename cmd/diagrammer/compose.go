package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xmhha/diagrammer/internal/compose"
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/invariant"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// composers maps a family to the function that builds its diagram source.
//
// It is the one place the set is written down. A family the model may declare
// but that has no entry here is one whose composer has not been built, and the
// difference is reported rather than hidden behind an empty output directory.
func composers() map[uml.Family]func(string, *uml.Model) (*diagram.Document, error) {
	return map[uml.Family]func(string, *uml.Model) (*diagram.Document, error){
		uml.FamilyComponent: compose.Component,
	}
}

// checkers maps a family to the completeness rule its documents must obey.
//
// It is kept beside composers deliberately: a family gains an entry in both at
// once, so a composer cannot be added without the rule that holds it to
// something.
func checkers() map[uml.Family]func(*uml.Model, *diagram.Document) []invariant.Problem {
	return map[uml.Family]func(*uml.Model, *diagram.Document) []invariant.Problem{
		uml.FamilyComponent: invariant.Component,
	}
}

func joinProblems(problems []invariant.Problem) string {
	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n  ")
}

// runCompose is stage 3: a UML model in, one diagram source per family out.
func runCompose(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("compose", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var (
		out    = flags.String("o", ".", "write the documents into this directory")
		family = flags.String("family", "", "compose only this family instead of every declared one")
	)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: diagrammer compose <codegraph.json> -o <dir> [flags]")
		flags.PrintDefaults()
	}
	path, err := parseOperand(flags, args, "codegraph.json path")
	if err != nil {
		return err
	}

	doc, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// The stage-2 boundary has exactly one gate, and this is it: the same check
	// validate runs, not a second opinion that could disagree with it. Compose
	// says it ran rather than running it quietly, so nobody is left thinking
	// the model reached here unchecked.
	model, err := validate.Codegraph(path, doc)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "%s: validated, %d declared families\n", path, len(model.Families))

	wanted, err := familiesToCompose(model, *family)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", *out, err)
	}

	for _, f := range wanted {
		build := composers()[f]
		if build == nil {
			return fmt.Errorf("the model declares the %q family, but its composer is not built yet; use -family to choose one that is", f)
		}
		if checkers()[f] == nil {
			return fmt.Errorf("the %q family has a composer but no completeness rule, so its output would go unchecked", f)
		}
		document, err := build(path, model)
		if err != nil {
			return fmt.Errorf("compose %s: %w", f, err)
		}
		// The family's completeness rule is checked over what was actually
		// built, before anything is written. A rule enforced only inside the
		// composer proves nothing about the document that leaves it.
		if problems := checkers()[f](model, document); len(problems) > 0 {
			return fmt.Errorf("the %s document breaks its own completeness rule, which is a defect in the composer:\n  %s",
				f, joinProblems(problems))
		}

		encoded, err := encodeDocument(document)
		if err != nil {
			return err
		}
		// The document is checked against its own schema before it is written.
		// A composer that drifts from the contract should fail here, where the
		// bug is, rather than at the renderer that reads the file.
		if err := schema.Validate(schema.Diagram, encoded); err != nil {
			return fmt.Errorf("the %s document does not satisfy the diagram schema, which is a defect in the composer rather than in the model: %w", f, err)
		}
		target := filepath.Join(*out, string(f)+".diagram.json")
		if err := os.WriteFile(target, encoded, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		reportDocument(stdout, target, document)
	}
	return nil
}

// familiesToCompose resolves what to emit, honouring what the model declares.
//
// What a UML model can express is a property of its contents rather than of the
// command line, so asking for a family it does not declare is refused. Drawing
// one anyway would produce a diagram nobody could trust.
func familiesToCompose(model *uml.Model, requested string) ([]uml.Family, error) {
	if requested == "" {
		return model.Families, nil
	}
	for _, f := range model.Families {
		if string(f) == requested {
			return []uml.Family{f}, nil
		}
	}
	return nil, fmt.Errorf("the model does not declare the %q family; it declares %s", requested, joinFamilies(model.Families))
}

// encodeDocument renders a document deterministically. Byte-identical output
// across runs is part of the contract, so nothing here may depend on map
// ordering: the diagram types hold no maps, and every slice arrives sorted.
func encodeDocument(d *diagram.Document) ([]byte, error) {
	encoded, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode document: %w", err)
	}
	return append(encoded, '\n'), nil
}

// reportDocument summarises what was composed, leading with the accounting.
//
// The dropped count is the number worth seeing: it says how much of what the
// model asserted the pages could not show, and every one of them is recorded on
// the box it belonged to.
func reportDocument(w io.Writer, path string, d *diagram.Document) {
	a := d.Accounting
	fmt.Fprintf(w, "%s: %d level(s), %d proven, %d drawn, %d recorded\n",
		path, len(d.Levels), a.Proven, a.Drawn, a.Dropped)
}
