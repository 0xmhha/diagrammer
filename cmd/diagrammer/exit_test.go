package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An exit code is what a script branches on, and nothing else here would notice
// it changing. The rule is narrower than "something went wrong", and it is the
// one docs/decisions.md states: exit 1 when the artefact the caller asked for
// was not produced, exit 0 when it was and some of the material could not be
// used.

func fixturePath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "testdata"}, parts...)...)
}

func TestWhatAnExitCodeMeans(t *testing.T) {
	dir := t.TempDir()
	model := fixturePath("codegraph", "order-service.codegraph.json")

	// A document that satisfies no schema, for the refusal cases.
	nonsense := filepath.Join(dir, "nonsense.json")
	if err := os.WriteFile(nonsense, []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name  string
		args  []string
		fails bool
		// produced is a path that must exist afterwards when the command
		// succeeded, so that "exit 0" is held to mean the artefact is there.
		produced string
	}{{
		name:     "a tree with a file that will not parse still yields a graph",
		args:     []string{"graph", fixturePath("src", "go-broken"), "-o", filepath.Join(dir, "broken.json")},
		produced: filepath.Join(dir, "broken.json"),
	}, {
		name:     "a page that cannot hold every relationship is still a page",
		args:     []string{"compose", fixturePath("codegraph", "tangle.codegraph.json"), "-o", dir},
		produced: filepath.Join(dir, "component.diagram.json"),
	}, {
		name:     "a model that validates",
		args:     []string{"validate", model},
		produced: "",
	}, {
		name:  "a source tree that is not there",
		args:  []string{"graph", filepath.Join(dir, "absent"), "-o", filepath.Join(dir, "never.json")},
		fails: true,
	}, {
		name:  "a document that does not satisfy its stage's contract",
		args:  []string{"validate", nonsense},
		fails: true,
	}, {
		name:  "a diagram source that is not one",
		args:  []string{"render", nonsense, "-o", filepath.Join(dir, "never.html")},
		fails: true,
	}, {
		name:  "a family the model does not declare",
		args:  []string{"compose", model, "-o", dir, "-family", "workflow"},
		fails: true,
	}} {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			err := run(c.args, &stdout, &stderr)

			switch {
			case c.fails && err == nil:
				t.Errorf("the command produced nothing and reported success: %q", stderr.String())
			case !c.fails && err != nil:
				t.Errorf("the artefact was asked for and the command failed: %v", err)
			}
			if c.fails || c.produced == "" {
				return
			}
			if _, statErr := os.Stat(c.produced); statErr != nil {
				t.Errorf("the command succeeded and %s is not there: %v", c.produced, statErr)
			}
		})
	}
}

// TestNothingIsRefusedInSilence is the other half of exit 0. A run that could
// not use some of its material has to say so, or the caller has a file and no
// way to know it is short.
func TestNothingIsRefusedInSilence(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	if err := run([]string{"graph", fixturePath("src", "go-broken"), "-o", filepath.Join(dir, "g.json")},
		&stdout, &stderr); err != nil {
		t.Fatalf("graph: %v", err)
	}
	said := stderr.String()
	if !strings.Contains(said, "could not read") {
		t.Errorf("a run that skipped a file said nothing about it: %q", said)
	}
	if !strings.Contains(said, "bad.go") {
		t.Errorf("a run that skipped a file did not name it: %q", said)
	}
}

// TestOutputBelongsToWhoeverRanItEvenWhenItExisted closes the half that
// os.WriteFile does not cover on its own. The mode it takes applies when the
// file is created; a file somebody made world-readable stays world-readable
// while this program fills it with the doc comments of a private repository.
func TestOutputBelongsToWhoeverRanItEvenWhenItExisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if err := run([]string{"graph", fixturePath("src", "go-basic"), "-o", path}, &stdout, &stderr); err != nil {
		t.Fatalf("graph: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("a file that existed at 0644 is %o after being written with private prose", mode)
	}
}
