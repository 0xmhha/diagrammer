package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/validate"
)

const fixture = "../../testdata/codegraph/order-service.codegraph.json"

func TestRun(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// wantOut is a substring expected on stdout when the command succeeds.
		wantOut string
		// wantErr is a substring expected in the error when it fails.
		wantErr string
	}{
		{name: "no arguments lists the commands", wantOut: "usage: diagrammer"},
		{name: "help lists the commands", args: []string{"help"}, wantOut: "validate"},
		{name: "version prints", args: []string{"version"}, wantOut: "diagrammer "},
		{name: "--version is the same", args: []string{"--version"}, wantOut: "diagrammer "},
		{name: "version takes no arguments", args: []string{"version", "extra"}, wantErr: "takes no arguments"},
		{name: "unknown command is refused", args: []string{"nonsense"}, wantErr: `unknown command "nonsense"`},
		{name: "validate accepts the fixture", args: []string{"validate", fixture}, wantOut: "valid, 4 families"},
		{name: "validate names the families", args: []string{"validate", fixture}, wantOut: "component, sequence, state, usecase"},
		{name: "validate needs a path", args: []string{"validate"}, wantErr: "takes one codegraph.json path"},
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
			if !strings.Contains(stdout.String(), c.wantOut) {
				t.Errorf("stdout does not contain %q:\n%s", c.wantOut, stdout.String())
			}
		})
	}
}

// TestUnbuiltCommandsAreNamed keeps a subcommand that 0.1.0 has planned
// distinguishable from one that does not exist. Someone typing `compose` should
// be told it is coming, not that they mistyped.
func TestUnbuiltCommandsAreNamed(t *testing.T) {
	for _, name := range []string{"graph", "compose", "render", "serve"} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run([]string{name}, &stdout, &stderr)
			if !errors.Is(err, errNotImplemented) {
				t.Errorf("want errNotImplemented, got %v", err)
			}
		})
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
