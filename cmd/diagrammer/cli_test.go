package main

import (
	"io"
	"reflect"
	"testing"

	"github.com/0xmhha/diagrammer/internal/command"
)

// TestEveryRequestFieldHasAFlag is the half of the two-surfaces contract that
// the tool schema cannot check.
//
// The MCP server gets its arguments from the request struct, so it reaches
// every field by construction. The CLI does not: a field with no flag behind it
// would be a capability a plugin has and a person does not, and nothing else
// would notice. So each case names a command line that sets every field, and
// the test fails on any field still holding its zero value.
//
// Adding a field to a request breaks this test until the flag and the case
// arrive with it, which is the point.
func TestEveryRequestFieldHasAFlag(t *testing.T) {
	cases := []struct {
		op    command.Op
		args  []string
		build func([]string, io.Writer) (any, error)
	}{
		{
			op:   command.OpGraph,
			args: []string{"src", "-o", "out.json", "-tests", "-max-depth", "3", "-exclude", "docs"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildGraphRequest(args, w)
			},
		}, {
			op:   command.OpValidate,
			args: []string{"model.json", "-graph", "graph.json"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildValidateRequest(args, w)
			},
		}, {
			op:   command.OpCompose,
			args: []string{"model.json", "-o", "dir", "-family", "component"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildComposeRequest(args, w)
			},
		}, {
			op:   command.OpRender,
			args: []string{"doc.json", "-o", "out.html", "-level", "overview"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildRenderRequest(args, w)
			},
		}, {
			op:   command.OpInstruct,
			args: []string{"-o", "prompt.md"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildInstructRequest(args, w)
			},
		}, {
			op:   command.OpMermaid,
			args: []string{"doc.json", "-o", "out.md"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildMermaidRequest(args, w)
			},
		}, {
			op:   command.OpSVG,
			args: []string{"doc.json", "-o", "out", "-level", "overview", "-size", "slide-16x9"},
			build: func(args []string, w io.Writer) (any, error) {
				return buildSVGRequest(args, w)
			},
		},
	}

	if len(cases) != len(command.Ops()) {
		t.Fatalf("this table covers %d operations, the registry declares %d", len(cases), len(command.Ops()))
	}

	for _, c := range cases {
		t.Run(string(c.op), func(t *testing.T) {
			req, err := c.build(c.args, io.Discard)
			if err != nil {
				t.Fatalf("build the request: %v", err)
			}
			value := reflect.ValueOf(req).Elem()
			typ := value.Type()
			for i := range typ.NumField() {
				if value.Field(i).IsZero() {
					t.Errorf("%s.%s was left at its zero value, so no flag reaches it",
						typ.Name(), typ.Field(i).Name)
				}
			}
		})
	}
}

// TestFlagsMayFollowTheOperand keeps the command surface working as written.
//
// Go's flag package stops at the first non-flag argument, so `graph . -o out`
// would silently leave -o unparsed. The surface puts the operand first, which
// makes this the case that regresses.
func TestFlagsMayFollowTheOperand(t *testing.T) {
	before, err := buildGraphRequest([]string{"-o", "x.json", "src"}, io.Discard)
	if err != nil {
		t.Fatalf("flags before the operand: %v", err)
	}
	after, err := buildGraphRequest([]string{"src", "-o", "x.json"}, io.Discard)
	if err != nil {
		t.Fatalf("flags after the operand: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("the two orders disagree:\n  before %+v\n  after  %+v", before, after)
	}
}

func TestSplitList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a,,b", []string{"a", "b"}},
		{",a,", []string{"a"}},
	}
	for _, c := range cases {
		if got := splitList(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitList(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
