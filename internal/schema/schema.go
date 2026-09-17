package schema

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/*.json
var files embed.FS

// Name identifies one embedded schema.
type Name string

const (
	// Graph is the stage-1 code graph an analyzer emits.
	Graph Name = "graph"
	// Codegraph is the stage-2 UML model a plugin's skill returns.
	Codegraph Name = "codegraph"
	// Diagram is the stage-3 diagram source compose emits, one per family.
	Diagram Name = "diagram"
)

const (
	schemaDir = "schemas"
	// baseURI namespaces the schemas so a cross-schema $ref resolves against a
	// stable identifier rather than a file path that only exists at build time.
	baseURI = "https://github.com/0xmhha/diagrammer/schemas/"
)

// All lists every embedded schema, ordered by the stage that produces it. It is
// the one place the set is written down: the drift guard and the compiler both
// walk it, so a schema added to the directory but not to this list is caught by
// a test rather than discovered at run time.
func All() []Name {
	return []Name{Graph, Codegraph, Diagram}
}

// URI returns the identifier n declares as its own $id.
//
// It is the same string the schemas use to $ref each other, so a caller handed
// a schema over MCP and a schema resolving a reference inside one are naming
// the same thing. A second identifier would be a second name for one document.
func URI(n Name) string { return baseURI + Filename(n) }

// Filename returns the name of n's file within the embedded directory.
func Filename(n Name) string {
	return string(n) + ".schema.json"
}

// Raw returns n exactly as committed.
//
// The stage-2 skill prompt is built from this, so the instruction given to the
// model and the check applied to what it returns come from one file.
func Raw(n Name) ([]byte, error) {
	b, err := files.ReadFile(path.Join(schemaDir, Filename(n)))
	if err != nil {
		return nil, fmt.Errorf("read embedded schema %q: %w", n, err)
	}
	return b, nil
}

// compiled builds every schema once, on first use. Compilation is pure and the
// result is immutable, so sharing it across callers is safe.
var compiled = sync.OnceValues(compileAll)

func compileAll() (map[Name]*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	names := All()
	for _, n := range names {
		raw, err := Raw(n)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("parse embedded schema %q: %w", n, err)
		}
		if err := c.AddResource(baseURI+Filename(n), doc); err != nil {
			return nil, fmt.Errorf("add embedded schema %q: %w", n, err)
		}
	}
	out := make(map[Name]*jsonschema.Schema, len(names))
	for _, n := range names {
		s, err := c.Compile(baseURI + Filename(n))
		if err != nil {
			return nil, fmt.Errorf("compile embedded schema %q: %w", n, err)
		}
		out[n] = s
	}
	return out, nil
}

// Compiled returns the compiled form of n.
//
// The returned schema is shared and must not be modified.
func Compiled(n Name) (*jsonschema.Schema, error) {
	m, err := compiled()
	if err != nil {
		return nil, err
	}
	s, ok := m[n]
	if !ok {
		return nil, fmt.Errorf("no embedded schema named %q", n)
	}
	return s, nil
}

// Validate reports whether doc satisfies n. doc is JSON text.
//
// A returned validation failure is a *jsonschema.ValidationError, which carries
// the instance location of every offending value; callers that report to a
// person should use it rather than the flat message.
func Validate(n Name, doc []byte) error {
	s, err := Compiled(n)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("parse document: %w", err)
	}
	if err := s.Validate(inst); err != nil {
		return err
	}
	return nil
}
