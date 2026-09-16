package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/command"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/validate"
)

const fixture = "../../testdata/codegraph/order-service.codegraph.json"

func TestRun(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// wantOut is a substring expected on stdout when the command succeeds.
		// Stdout carries the product, so only a document appears there.
		wantOut string
		// wantErrOut is a substring expected on stderr. Stderr carries the
		// commentary: what was read, what was written, what could not be done.
		wantErrOut string
		// wantErr is a substring expected in the error when it fails.
		wantErr string
	}{
		{name: "no arguments lists the commands", wantOut: "usage: diagrammer"},
		{name: "help lists the commands", args: []string{"help"}, wantOut: "validate"},
		{name: "version prints", args: []string{"version"}, wantOut: "diagrammer "},
		{name: "--version is the same", args: []string{"--version"}, wantOut: "diagrammer "},
		{name: "version takes no arguments", args: []string{"version", "extra"}, wantErr: "takes no arguments"},
		{name: "unknown command is refused", args: []string{"nonsense"}, wantErr: `unknown command "nonsense"`},
		{name: "validate accepts the fixture", args: []string{"validate", fixture}, wantErrOut: "valid, 4 families"},
		{name: "validate names the families", args: []string{"validate", fixture}, wantErrOut: "component, sequence, state, usecase"},
		{name: "validate needs a path", args: []string{"validate"}, wantErr: "codegraph.json path, got none"},
		{name: "validate takes only one path", args: []string{"validate", "a", "b"}, wantErr: "takes one codegraph.json path"},
		{name: "validate reports a missing file", args: []string{"validate", "no/such/file.json"}, wantErr: "read no/such/file.json"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(c.args, &stdout, &stderr)

			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("want an error mentioning %q, got none", c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("error does not mention %q: %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantOut != "" && !strings.Contains(stdout.String(), c.wantOut) {
				t.Errorf("stdout does not contain %q:\n%s", c.wantOut, stdout.String())
			}
			if c.wantErrOut != "" && !strings.Contains(stderr.String(), c.wantErrOut) {
				t.Errorf("stderr does not contain %q:\n%s", c.wantErrOut, stderr.String())
			}
		})
	}
}

// TestNoCapabilityIsStillUnbuilt records that every operation 0.1.0 named is
// now behind real code. The not-implemented path stays, because the next family
// or language added will need it again, but nothing takes it today.
func TestNoCapabilityIsStillUnbuilt(t *testing.T) {
	for _, op := range command.Ops() {
		req, err := command.New(op)
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		_, err = req.Run(t.Context())
		var notBuilt *command.ErrNotImplemented
		if errors.As(err, &notBuilt) {
			t.Errorf("%s reports that it is not implemented", op)
		}
	}
}

// TestValidateRefusesAndReports checks the failure path end to end: a bad model
// must come back as a report naming the offending location, since that is what
// a person or a plugin acts on.
func TestValidateRefusesAndReports(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "broken.codegraph.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	// Point a message at a lifeline that was never declared.
	damaged := bytes.Replace(raw, []byte(`"to": "ll.store"`), []byte(`"to": "ll.ghost"`), 1)
	if bytes.Equal(damaged, raw) {
		t.Fatal("fixture no longer contains the value this test damages")
	}
	if err := os.WriteFile(broken, damaged, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = run([]string{"validate", broken}, &stdout, &stderr)
	if err == nil {
		t.Fatal("want the damaged model refused, got accepted")
	}
	var report *validate.Report
	if !errors.As(err, &report) {
		t.Fatalf("want a *validate.Report, got %T", err)
	}
	if !strings.Contains(err.Error(), "/sequence/messages/1/to") {
		t.Errorf("report does not locate the defect: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("a refused document must print nothing to stdout, got %q", stdout.String())
	}
}

const (
	basicSrc  = "../../testdata/src/go-basic"
	brokenSrc = "../../testdata/src/go-broken"
)

func TestGraphCommand(t *testing.T) {
	t.Run("writes a schema-valid graph to a file", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "graph.json")
		var stdout, stderr bytes.Buffer
		if err := run([]string{"graph", basicSrc, "-o", out}, &stdout, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		if err := schema.Validate(schema.Graph, raw); err != nil {
			t.Errorf("written graph does not satisfy the schema: %v", err)
		}
		// The summary goes to stderr so that stdout stays usable as a pipe.
		if !strings.Contains(stderr.String(), "nodes") {
			t.Errorf("no summary on stderr: %q", stderr.String())
		}
	})

	t.Run("writes to stdout when no output is named", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := run([]string{"graph", basicSrc}, &stdout, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := schema.Validate(schema.Graph, stdout.Bytes()); err != nil {
			t.Errorf("graph on stdout does not satisfy the schema: %v", err)
		}
	})

	t.Run("accepts flags after the source", func(t *testing.T) {
		// The command surface puts the operand first, which Go's flag package
		// does not handle on its own, so this is the case that would regress.
		var withFlag, plain bytes.Buffer
		var stderr bytes.Buffer
		if err := run([]string{"graph", basicSrc, "-exclude", "internal"}, &withFlag, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := run([]string{"graph", basicSrc}, &plain, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if withFlag.Len() >= plain.Len() {
			t.Error("-exclude after the source had no effect, so the flag was not parsed")
		}
	})

	t.Run("names the files that did not parse", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := run([]string{"graph", brokenSrc}, &stdout, &stderr); err != nil {
			t.Fatalf("a parse failure must not fail the command: %v", err)
		}
		if !strings.Contains(stderr.String(), "bad.go") {
			t.Errorf("the failing file is not named on stderr: %q", stderr.String())
		}
		if !strings.Contains(stderr.String(), "did not parse cleanly") {
			t.Errorf("stderr does not say the file would not parse: %q", stderr.String())
		}
	})

	t.Run("needs a source directory", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := run([]string{"graph"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "source directory") {
			t.Errorf("want a complaint about the missing operand, got %v", err)
		}
	})

	t.Run("refuses a source that is not a directory", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := run([]string{"graph", "main.go"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("want a complaint about the source, got %v", err)
		}
	})
}

const nestedModel = "../../testdata/codegraph/nested-platform.codegraph.json"

func TestComposeCommand(t *testing.T) {
	t.Run("writes a schema-valid document per family", func(t *testing.T) {
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		if err := run([]string{"compose", nestedModel, "-o", dir}, &stdout, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "component.diagram.json"))
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		if err := schema.Validate(schema.Diagram, raw); err != nil {
			t.Errorf("written document does not satisfy the schema: %v", err)
		}
		if !strings.Contains(stderr.String(), "proven") {
			t.Errorf("no accounting on stderr: %q", stderr.String())
		}
	})

	t.Run("says it validated the model rather than doing it quietly", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := run([]string{"compose", nestedModel, "-o", t.TempDir()}, &stdout, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stderr.String(), "validated") {
			t.Errorf("compose did not say it ran the stage-2 check: %q", stderr.String())
		}
	})

	t.Run("refuses a family the model does not declare", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := run([]string{"compose", nestedModel, "-family", "sequence", "-o", t.TempDir()}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "does not declare") {
			t.Errorf("want a refusal naming the undeclared family, got %v", err)
		}
	})

	t.Run("emits every family the model declares", func(t *testing.T) {
		// order-service declares all four, so all four files must appear. A
		// composer added without being reachable from here would otherwise sit
		// unused with nothing to say so.
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		if err := run([]string{"compose", fixture, "-o", dir}, &stdout, &stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, family := range []string{"component", "sequence", "state", "usecase"} {
			path := filepath.Join(dir, family+".diagram.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s was not written: %v", family, err)
				continue
			}
			if err := schema.Validate(schema.Diagram, raw); err != nil {
				t.Errorf("%s does not satisfy the schema: %v", family, err)
			}
		}
	})

	t.Run("refuses a model that does not validate", func(t *testing.T) {
		broken := filepath.Join(t.TempDir(), "broken.codegraph.json")
		if err := os.WriteFile(broken, []byte(`{"schemaVersion":1}`), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		var stdout, stderr bytes.Buffer
		if err := run([]string{"compose", broken, "-o", t.TempDir()}, &stdout, &stderr); err == nil {
			t.Error("want the invalid model refused, got success")
		}
	})
}

func TestRenderCommand(t *testing.T) {
	compose := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		if err := run([]string{"compose", nestedModel, "-family", "component", "-o", dir}, &stdout, &stderr); err != nil {
			t.Fatalf("compose: %v", err)
		}
		return filepath.Join(dir, "component.diagram.json")
	}

	t.Run("writes a self-contained page", func(t *testing.T) {
		doc := compose(t)
		out := filepath.Join(t.TempDir(), "page.html")
		var stdout, stderr bytes.Buffer
		if err := run([]string{"render", doc, "-o", out}, &stdout, &stderr); err != nil {
			t.Fatalf("render: %v", err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read the page: %v", err)
		}
		page := string(raw)
		for _, want := range []string{"<!DOCTYPE html>", "<svg", "<style>", "<script>"} {
			if !strings.Contains(page, want) {
				t.Errorf("the page has no %s", want)
			}
		}
		if !strings.Contains(stderr.String(), "proven") {
			t.Errorf("no accounting on stderr: %q", stderr.String())
		}
	})

	t.Run("refuses a document that is not a diagram source", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := run([]string{"render", nestedModel, "-o", filepath.Join(t.TempDir(), "x.html")}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "diagram schema") {
			t.Errorf("want a refusal naming the schema, got %v", err)
		}
	})

	t.Run("needs a document", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := run([]string{"render"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "diagram source path") {
			t.Errorf("want a complaint about the missing operand, got %v", err)
		}
	})
}

// TestOutputBelongsToWhoeverRanIt holds the file modes to a decision rather
// than to whatever the umask happened to be.
//
// A code graph carries the doc comments of everything it read, and a rendered
// page carries whatever those comments said. Pointed at a private repository,
// the output holds private prose, and world-readable is the wrong default for
// that. Widening it is one chmod the person who wants it can run.
func TestOutputBelongsToWhoeverRanIt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer

	graphPath := filepath.Join(t.TempDir(), "graph.json")
	if err := run([]string{"graph", basicSrc, "-o", graphPath}, &stdout, &stderr); err != nil {
		t.Fatalf("graph: %v", err)
	}
	if err := run([]string{"compose", nestedModel, "-o", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("compose: %v", err)
	}

	cases := []struct {
		path string
		want os.FileMode
		what string
	}{
		{graphPath, 0o600, "a code graph"},
		{dir, 0o700, "an output directory"},
		{filepath.Join(dir, "component.diagram.json"), 0o600, "a diagram source"},
	}
	for _, c := range cases {
		info, err := os.Stat(c.path)
		if err != nil {
			t.Errorf("%s: %v", c.what, err)
			continue
		}
		if got := info.Mode().Perm(); got != c.want {
			t.Errorf("%s is %v, want %v; anyone on the machine can read it", c.what, got, c.want)
		}
	}
}
