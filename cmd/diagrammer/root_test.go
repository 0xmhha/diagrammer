package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/command"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// What the root refuses.
//
// The threat is not a hostile operator; it is a path that arrived over MCP
// because a model chose it, and that model has just been handed the doc
// comments of a repository it did not write. These check the refusal holds for
// the ways out that actually exist, rather than only for the obvious one.

func rootOf(t *testing.T, dir string) aRoot {
	t.Helper()
	r, err := newRoot(dir)
	if err != nil {
		t.Fatalf("root %s: %v", dir, err)
	}
	return r
}

// TestTheRootRefusesTheWaysOut covers each escape separately, because they fail
// for different reasons and a single "outside" test would pass while two of
// them still worked.
func TestTheRootRefusesTheWaysOut(t *testing.T) {
	home := t.TempDir()
	inside := filepath.Join(home, "work")
	if err := os.MkdirAll(filepath.Join(inside, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "elsewhere")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.json")
	if err := os.WriteFile(secret, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A link inside the root pointing out of it. This is the one a path-prefix
	// check on the unresolved string lets through.
	link := filepath.Join(inside, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	root := rootOf(t, inside)

	for _, c := range []struct {
		name string
		path string
		want bool // true if it should be allowed
	}{
		{"the root itself", inside, true},
		{"a file within it", filepath.Join(inside, "graph.json"), true},
		{"a directory within it", filepath.Join(inside, "sub"), true},
		{"a sibling directory", outside, false},
		{"a file in a sibling", secret, false},
		{"climbing out with ..", filepath.Join(inside, "..", "elsewhere", "secret.json"), false},
		{"through a symlink that leaves", filepath.Join(link, "secret.json"), false},
		{"the symlink itself", link, false},
		{"an absolute path elsewhere", "/etc/passwd", false},
		// A directory whose name merely begins with the root's is not inside
		// it, and a string-prefix test would say it was.
		{"a neighbour with a longer name", inside + "-old", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := root.holds(c.path)
			if c.want && err != nil {
				t.Errorf("%s should be inside the root and was refused: %v", c.path, err)
			}
			if !c.want && err == nil {
				t.Errorf("%s is outside the root and was allowed", c.path)
			}
		})
	}
}

// TestTheRefusalSaysHowToWidenIt is not politeness. The person who can lift the
// restriction is the one who started the server, and they are not the one
// reading the error: a model is. A refusal that does not say what to do with it
// gets retried with a different path until something works.
func TestTheRefusalSaysHowToWidenIt(t *testing.T) {
	root := rootOf(t, t.TempDir())
	err := root.holds("/etc/passwd")
	if err == nil {
		t.Fatal("/etc/passwd was allowed")
	}
	for _, want := range []string{"/etc/passwd", "-root", "outside"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal never says %q: %s", want, err)
		}
	}
}

// TestEveryPathAnOperationTakesIsChecked is the guard that matters most,
// because the failure it prevents is silent: a capability grows an argument
// that names a file, nobody adds it to Paths, and that one argument reaches
// anywhere while every other is confined.
func TestEveryPathAnOperationTakesIsChecked(t *testing.T) {
	// The fields that are paths, by the only honest test available: a request
	// field whose name reads like a path. If this finds one Paths does not
	// return, either the field is not a path or Paths forgot it, and both are
	// worth stopping for.
	pathish := map[string]bool{
		"Source": true, "Model": true, "Document": true, "Out": true, "Graph": true,
	}

	for _, op := range command.Ops() {
		req, err := command.New(op)
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		t.Run(string(op), func(t *testing.T) {
			named := len(fieldsNamedLikePaths(req, pathish))
			if got := len(req.Paths()); got != named {
				t.Errorf("%s has %d field(s) that name a path and Paths returns %d", op, named, got)
			}
		})
	}
}

// TestAToolRefusesBeforeItActs checks the confinement over the real protocol,
// and checks the order: a path outside the root must be refused rather than
// half-acted-on, so nothing is created before the refusal.
func TestAToolRefusesBeforeItActs(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "escaped.json")

	session := connectRooted(t, root)
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      string(command.OpInstruct),
		Arguments: map[string]any{"out": outside},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !result.IsError {
		t.Error("writing outside the root came back as a success")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Errorf("%s exists, so the refusal came after the write rather than before it", outside)
	}

	// The same call inside the root works, so the refusal is about where the
	// path is and not about the argument being rejected outright.
	within := filepath.Join(root, "prompt.md")
	ok, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      string(command.OpInstruct),
		Arguments: map[string]any{"out": within},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if ok.IsError {
		t.Errorf("writing inside the root was refused: %+v", ok.Content)
	}
	if _, err := os.Stat(within); err != nil {
		t.Errorf("%s was not written: %v", within, err)
	}
}

// TestTheCommandLineIsNotConfined records the other half of the decision. A
// person typing a path at a terminal can already write anywhere their shell
// can, so restricting the CLI would protect nobody and would break the ordinary
// use of writing output beside a repository.
func TestTheCommandLineIsNotConfined(t *testing.T) {
	out := filepath.Join(t.TempDir(), "prompt.md")
	var stdout, stderr strings.Builder
	if err := run([]string{"instruct", "-o", out}, &stdout, &stderr); err != nil {
		t.Fatalf("instruct: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("the command line refused a path outside the working directory: %v", err)
	}
}

// fieldsNamedLikePaths lists a request's exported string fields whose names are
// in the given set.
func fieldsNamedLikePaths(req command.Request, pathish map[string]bool) []string {
	var out []string
	v := reflect.ValueOf(req).Elem()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Type.Kind() == reflect.String && pathish[f.Name] {
			out = append(out, f.Name)
		}
	}
	return out
}
