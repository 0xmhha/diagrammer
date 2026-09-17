//go:build cgo

package treesitter

import (
	"strings"
	"unsafe"

	"github.com/alexaandru/go-sitter-forest/javascript"
	"github.com/alexaandru/go-sitter-forest/python"
	"github.com/alexaandru/go-sitter-forest/solidity"
	"github.com/alexaandru/go-sitter-forest/typescript"

	"github.com/0xmhha/diagrammer/internal/graph"
)

// docStyle says where a language keeps a declaration's documentation.
type docStyle int

const (
	// docPreceding is a comment immediately above the declaration, which is
	// what C-like languages do.
	docPreceding docStyle = iota
	// docFirstString is a string literal as the first statement of the body,
	// which is what Python does.
	docFirstString
)

// language is everything that differs between the grammars.
//
// The walk itself is the same for all of them: find the declarations, read
// their names, attach their documentation. What differs is which node types
// mean what, and that is data rather than code.
type language struct {
	name       graph.Language
	extensions []string
	grammar    func() unsafe.Pointer
	// decls maps a grammar's node type to the kind of graph node it becomes.
	decls map[string]graph.NodeKind
	// imports lists the node types that name another module.
	imports  map[string]bool
	calls    map[string]bool
	docStyle docStyle
	// exported decides whether a name is part of the public surface. The rule
	// differs by language and only the analyzer knows which one applied, which
	// is why the graph carries the answer rather than the name.
	exported func(string) bool
}

// notUnderscored is the convention Python and Solidity share: a leading
// underscore means "not for you". It is a convention rather than a rule in
// either language, and the graph says so by carrying what the analyzer decided
// rather than asking a later stage to guess.
func notUnderscored(name string) bool {
	return name != "" && !strings.HasPrefix(name, "_")
}

func languages() []language {
	return []language{
		{
			name:       graph.Python,
			extensions: []string{".py"},
			grammar:    python.GetLanguage,
			decls: map[string]graph.NodeKind{
				"class_definition":    graph.KindType,
				"function_definition": graph.KindFunc,
			},
			imports:  map[string]bool{"import_statement": true, "import_from_statement": true},
			calls:    map[string]bool{"call": true},
			docStyle: docFirstString,
			exported: notUnderscored,
		},
		{
			name:       graph.Solidity,
			extensions: []string{".sol"},
			grammar:    solidity.GetLanguage,
			decls: map[string]graph.NodeKind{
				"contract_declaration":  graph.KindType,
				"interface_declaration": graph.KindType,
				"library_declaration":   graph.KindType,
				"struct_declaration":    graph.KindType,
				"enum_declaration":      graph.KindType,
				"function_definition":   graph.KindFunc,
				"modifier_definition":   graph.KindFunc,
				"event_definition":      graph.KindFunc,
			},
			imports:  map[string]bool{"import_directive": true},
			calls:    map[string]bool{"call_expression": true},
			docStyle: docPreceding,
			exported: notUnderscored,
		},
		{
			name:       graph.JavaScript,
			extensions: []string{".js", ".mjs", ".cjs", ".jsx"},
			grammar:    javascript.GetLanguage,
			decls:      scriptDecls(),
			imports:    map[string]bool{"import_statement": true},
			calls:      map[string]bool{"call_expression": true},
			docStyle:   docPreceding,
			exported:   exportedInScript,
		},
		{
			name:       graph.TypeScript,
			extensions: []string{".ts", ".tsx", ".mts", ".cts"},
			grammar:    typescript.GetLanguage,
			decls:      scriptDecls(),
			imports:    map[string]bool{"import_statement": true},
			calls:      map[string]bool{"call_expression": true},
			docStyle:   docPreceding,
			exported:   exportedInScript,
		},
	}
}

func scriptDecls() map[string]graph.NodeKind {
	return map[string]graph.NodeKind{
		"class_declaration":              graph.KindType,
		"interface_declaration":          graph.KindType,
		"type_alias_declaration":         graph.KindType,
		"enum_declaration":               graph.KindType,
		"function_declaration":           graph.KindFunc,
		"generator_function_declaration": graph.KindFunc,
		"method_definition":              graph.KindFunc,
	}
}

// exportedInScript is a placeholder the walker overrides.
//
// JS and TS decide visibility with a keyword above the declaration rather than
// with the name, so the answer is not in the name at all. The walker looks at
// the declaration's parent instead, and this exists only so the table has the
// same shape for every language.
func exportedInScript(string) bool { return false }
