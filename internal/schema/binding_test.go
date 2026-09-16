package schema_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/schema"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// The schemas are the contract and the Go types are a convenience over them, so
// the two must not be able to disagree quietly. These tests are the drift guard
// that decision depends on: without one, the decision to make the schema
// authoritative buys nothing, because nothing would notice when a Go struct
// stopped matching it.
//
// Every object in a schema is bound to the struct that mirrors it. A schema
// object with no binding fails, and so does a binding whose fields differ from
// the schema's properties in name or in whether they are required.

// bindings maps a JSON pointer within a schema to the Go type that mirrors it.
// "#" is the schema's root object.
var bindings = map[schema.Name]map[string]reflect.Type{
	schema.Graph: {
		"#":                    reflect.TypeOf(graph.Graph{}),
		"#/$defs/node":         reflect.TypeOf(graph.Node{}),
		"#/$defs/sourceRef":    reflect.TypeOf(graph.SourceRef{}),
		"#/$defs/edge":         reflect.TypeOf(graph.Edge{}),
		"#/$defs/diagnostics":  reflect.TypeOf(graph.Diagnostics{}),
		"#/$defs/parseFailure": reflect.TypeOf(graph.ParseFailure{}),
		"#/$defs/diagnostics/properties/unresolvedReferences/items": reflect.TypeOf(graph.UnresolvedReference{}),
	},
	schema.Codegraph: {
		"#":                  reflect.TypeOf(uml.Model{}),
		"#/properties/meta":  reflect.TypeOf(uml.Meta{}),
		"#/$defs/provenance": reflect.TypeOf(uml.Provenance{}),
		"#/$defs/provenance/properties/sourceGraph": reflect.TypeOf(uml.SourceGraph{}),

		"#/$defs/componentModel": reflect.TypeOf(uml.ComponentModel{}),
		"#/$defs/component":      reflect.TypeOf(uml.Component{}),
		"#/$defs/port":           reflect.TypeOf(uml.Port{}),
		"#/$defs/interface":      reflect.TypeOf(uml.Interface{}),
		"#/$defs/interface/properties/operations/items": reflect.TypeOf(uml.Operation{}),
		"#/$defs/dependency":                            reflect.TypeOf(uml.Dependency{}),

		"#/$defs/sequenceModel":                      reflect.TypeOf(uml.SequenceModel{}),
		"#/$defs/lifeline":                           reflect.TypeOf(uml.Lifeline{}),
		"#/$defs/message":                            reflect.TypeOf(uml.Message{}),
		"#/$defs/activation":                         reflect.TypeOf(uml.Activation{}),
		"#/$defs/fragment":                           reflect.TypeOf(uml.Fragment{}),
		"#/$defs/fragment/properties/operands/items": reflect.TypeOf(uml.Operand{}),

		"#/$defs/stateModel": reflect.TypeOf(uml.StateModel{}),
		"#/$defs/state":      reflect.TypeOf(uml.State{}),
		"#/$defs/transition": reflect.TypeOf(uml.Transition{}),

		"#/$defs/usecaseModel":                               reflect.TypeOf(uml.UsecaseModel{}),
		"#/$defs/usecaseModel/properties/system":             reflect.TypeOf(uml.System{}),
		"#/$defs/usecaseModel/properties/actors/items":       reflect.TypeOf(uml.Actor{}),
		"#/$defs/usecaseModel/properties/usecases/items":     reflect.TypeOf(uml.Usecase{}),
		"#/$defs/usecaseModel/properties/associations/items": reflect.TypeOf(uml.Association{}),
		"#/$defs/usecaseModel/properties/includes/items":     reflect.TypeOf(uml.Include{}),
		"#/$defs/usecaseModel/properties/extends/items":      reflect.TypeOf(uml.Extend{}),
	},
	schema.Diagram: {
		"#":                                          reflect.TypeOf(diagram.Document{}),
		"#/properties/meta":                          reflect.TypeOf(diagram.Meta{}),
		"#/properties/provenance":                    reflect.TypeOf(diagram.Provenance{}),
		"#/$defs/accounting":                         reflect.TypeOf(diagram.Accounting{}),
		"#/$defs/level":                              reflect.TypeOf(diagram.Level{}),
		"#/$defs/level/properties/grid":              reflect.TypeOf(diagram.Grid{}),
		"#/$defs/box":                                reflect.TypeOf(diagram.Box{}),
		"#/$defs/box/properties/ports/items":         reflect.TypeOf(diagram.Port{}),
		"#/$defs/droppedRelationship":                reflect.TypeOf(diagram.DroppedRelationship{}),
		"#/$defs/region":                             reflect.TypeOf(diagram.Region{}),
		"#/$defs/connection":                         reflect.TypeOf(diagram.Connection{}),
		"#/$defs/activation":                         reflect.TypeOf(diagram.Activation{}),
		"#/$defs/fragment":                           reflect.TypeOf(diagram.Fragment{}),
		"#/$defs/fragment/properties/operands/items": reflect.TypeOf(diagram.FragmentOperand{}),
	},
}

// TestEverySchemaObjectIsBound fails when a schema grows an object that no Go
// type mirrors, or keeps a binding for one it no longer has. It is what stops a
// schema change from landing without the Go side following it.
func TestEverySchemaObjectIsBound(t *testing.T) {
	for _, name := range schema.All() {
		t.Run(string(name), func(t *testing.T) {
			found := objectPointers(t, name)
			bound := bindings[name]

			for _, p := range found {
				if _, ok := bound[p]; !ok {
					t.Errorf("schema object %s has no Go type bound to it; add one to bindings", p)
				}
			}
			for p := range bound {
				if !contains(found, p) {
					t.Errorf("binding %s names no object in the schema; remove it", p)
				}
			}
		})
	}
}

// TestSchemaBinding compares each bound struct's JSON fields with the schema
// properties they stand for, in both name and whether they are required.
func TestSchemaBinding(t *testing.T) {
	for _, name := range schema.All() {
		doc := parse(t, name)
		for pointer, typ := range bindings[name] {
			t.Run(string(name)+" "+pointer, func(t *testing.T) {
				node := resolve(t, doc, pointer)
				wantProps, wantRequired := schemaFields(t, node, pointer)
				gotProps, gotOptional := goFields(t, typ)

				if diff := diffSets(wantProps, gotProps); diff != "" {
					t.Errorf("%s: fields differ from schema properties\n%s", typ, diff)
				}

				for _, p := range sortedKeys(wantProps) {
					if _, ok := gotProps[p]; !ok {
						continue // already reported above
					}
					required := wantRequired[p]
					optional := gotOptional[p]
					switch {
					case required && optional:
						t.Errorf("%s: field %q is required by the schema but carries omitempty", typ, p)
					case !required && !optional:
						t.Errorf("%s: field %q is optional in the schema but lacks omitempty", typ, p)
					}
				}
			})
		}
	}
}

// TestEmbeddedSchemasCompile proves the committed files are valid JSON Schema.
// A schema that cannot compile would otherwise only fail when a document was
// first validated against it, which may be long after the commit that broke it.
func TestEmbeddedSchemasCompile(t *testing.T) {
	for _, name := range schema.All() {
		if _, err := schema.Compiled(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// --- helpers -----------------------------------------------------------------

func parse(t *testing.T, name schema.Name) map[string]any {
	t.Helper()
	raw, err := schema.Raw(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

// objectPointers lists every node in the schema that declares properties, which
// is what makes it an object a Go struct has to mirror.
func objectPointers(t *testing.T, name schema.Name) []string {
	t.Helper()
	var found []string
	var walk func(node any, pointer string)
	walk = func(node any, pointer string) {
		switch n := node.(type) {
		case map[string]any:
			if _, ok := n["properties"].(map[string]any); ok {
				found = append(found, pointer)
			}
			for _, k := range sortedKeys(n) {
				walk(n[k], pointer+"/"+k)
			}
		case []any:
			for i, v := range n {
				walk(v, fmt.Sprintf("%s/%d", pointer, i))
			}
		}
	}
	walk(parse(t, name), "#")
	sort.Strings(found)
	return found
}

func resolve(t *testing.T, doc map[string]any, pointer string) map[string]any {
	t.Helper()
	var cur any = doc
	for _, step := range strings.Split(strings.TrimPrefix(pointer, "#"), "/") {
		if step == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("pointer %s: %q is not an object", pointer, step)
		}
		cur, ok = m[step]
		if !ok {
			t.Fatalf("pointer %s: no key %q", pointer, step)
		}
	}
	out, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("pointer %s does not name an object", pointer)
	}
	return out
}

// schemaFields returns the property names an object declares and which of them
// it requires.
func schemaFields(t *testing.T, node map[string]any, pointer string) (props map[string]bool, required map[string]bool) {
	t.Helper()
	raw, ok := node["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s declares no properties", pointer)
	}
	if additional, ok := node["additionalProperties"]; !ok || additional != false {
		t.Errorf("%s must set additionalProperties to false, or a document could carry fields no Go type reads", pointer)
	}
	props = make(map[string]bool, len(raw))
	for k := range raw {
		props[k] = true
	}
	required = make(map[string]bool)
	// An object with no required array requires nothing, which is a legitimate
	// shape rather than a malformed one.
	list, _ := node["required"].([]any)
	for _, r := range list {
		name, ok := r.(string)
		if !ok {
			t.Fatalf("%s: required entry is not a string", pointer)
		}
		if !props[name] {
			t.Errorf("%s requires %q but does not declare it", pointer, name)
		}
		required[name] = true
	}
	return props, required
}

// goFields returns the JSON names a struct exposes and which of them carry
// omitempty.
func goFields(t *testing.T, typ reflect.Type) (names map[string]bool, optional map[string]bool) {
	t.Helper()
	if typ.Kind() != reflect.Struct {
		t.Fatalf("%s is not a struct", typ)
	}
	names = make(map[string]bool, typ.NumField())
	optional = make(map[string]bool)
	for i := range typ.NumField() {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			t.Errorf("%s.%s has no json tag, so it cannot be matched to a schema property", typ, f.Name)
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "-" {
			continue
		}
		if name == "" {
			t.Errorf("%s.%s has an empty json name", typ, f.Name)
			continue
		}
		names[name] = true
		for _, opt := range parts[1:] {
			if opt == "omitempty" {
				optional[name] = true
			}
		}
	}
	return names, optional
}

func diffSets(want, got map[string]bool) string {
	var b strings.Builder
	for _, k := range sortedKeys(want) {
		if !got[k] {
			fmt.Fprintf(&b, "  missing in Go: %s\n", k)
		}
	}
	for _, k := range sortedKeys(got) {
		if !want[k] {
			fmt.Fprintf(&b, "  missing in schema: %s\n", k)
		}
	}
	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
