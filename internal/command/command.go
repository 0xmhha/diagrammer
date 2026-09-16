// Package command holds the capabilities the binary offers, independent of how
// they are reached.
//
// The CLI and the MCP server are two faces of one set of operations, and
// neither may gain a capability the other lacks. That is a contract rather than
// an aspiration, so it is arranged to be checkable: every operation is declared
// once here, both surfaces are built from that declaration, and a test asserts
// they cover the same set with the same arguments.
//
// Keeping the operations here also keeps the argument names honest. Once 0.1.0
// ships, plugins bind to the MCP tool and argument names, so a rename is a
// breaking change rather than a tidy-up, and it should be as visible as one.
package command

import (
	"context"
	"fmt"
	"sort"
)

// Op names one capability.
type Op string

const (
	// OpGraph is stage 1: source in, code graph out.
	OpGraph Op = "graph"
	// OpValidate is the stage-2 boundary: accept or refuse a returned UML model.
	OpValidate Op = "validate"
	// OpCompose is stage 3: a UML model in, one diagram source per family out.
	OpCompose Op = "compose"
	// OpRender is stage 4: a diagram source in, a self-contained page out.
	OpRender Op = "render"
)

// OutputFile is one thing an operation produced.
//
// Path is empty when the caller did not name one, and the content is handed
// back instead. That is what lets the same operation serve a command writing to
// standard output and a plugin wanting the bytes.
type OutputFile struct {
	Path    string
	Content []byte
}

// Result is what an operation produced.
type Result struct {
	// Summary is for a person: what was read, what was written, and what could
	// not be done. The CLI prints it and the MCP server returns it.
	Summary []string
	Files   []OutputFile
}

func (r *Result) say(format string, args ...any) {
	r.Summary = append(r.Summary, fmt.Sprintf(format, args...))
}

// Request is one operation's arguments.
//
// Each implementation is a plain struct whose JSON tags are the argument names
// both surfaces use, so there is one spelling of every argument rather than one
// per surface.
type Request interface {
	// Op names the capability this request invokes.
	Op() Op
	// Run performs it.
	Run(ctx context.Context) (*Result, error)
}

// operations is the registry both surfaces are built from. A capability absent
// here is reachable from neither, which is the point: there is no way to add
// one to a single surface by accident.
func operations() map[Op]func() Request {
	return map[Op]func() Request{
		OpGraph:    func() Request { return &GraphRequest{} },
		OpValidate: func() Request { return &ValidateRequest{} },
		OpCompose:  func() Request { return &ComposeRequest{} },
		OpRender:   func() Request { return &RenderRequest{} },
	}
}

// Ops lists every capability, in a stable order.
func Ops() []Op {
	all := operations()
	out := make([]Op, 0, len(all))
	for op := range all {
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// New returns an empty request for op, which a surface then fills in from flags
// or from JSON.
func New(op Op) (Request, error) {
	build, ok := operations()[op]
	if !ok {
		return nil, fmt.Errorf("unknown operation %q", op)
	}
	return build(), nil
}

// ErrNotImplemented is returned by an operation 0.1.0 has named but not built.
//
// It is distinct from an unknown operation, so someone asking for a real
// capability is told it is coming rather than that it does not exist.
type ErrNotImplemented struct {
	Op Op
}

func (e *ErrNotImplemented) Error() string {
	return string(e.Op) + " is not implemented yet"
}
