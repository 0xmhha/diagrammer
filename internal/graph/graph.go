package graph

// NodeKind is what a node stands for in the source tree.
//
// Every analyzer projects its language onto these five, whatever that language
// calls them, so that one graph shape serves every parser path.
type NodeKind string

const (
	// KindGroup is a directory that holds only other directories.
	KindGroup NodeKind = "group"
	// KindPackage is a directory that holds source.
	KindPackage NodeKind = "package"
	KindFile    NodeKind = "file"
	KindType    NodeKind = "type"
	KindFunc    NodeKind = "func"
)

// EdgeKind is the evidence behind a reference, coarsest first.
type EdgeKind string

const (
	// EdgeImport is a declared dependency between packages or files.
	EdgeImport EdgeKind = "import"
	// EdgeCall is one function reaching another.
	EdgeCall EdgeKind = "call"
)

// Language names a parser path.
type Language string

const (
	Go         Language = "go"
	Python     Language = "python"
	JavaScript Language = "javascript"
	TypeScript Language = "typescript"
)

// Graph is what an AST parser proved about a source tree.
//
// Nodes are sorted by id and edges by from, to and kind. That ordering is part
// of the byte-identity guarantee rather than a convenience, so anything
// rebuilding a Graph must restore it before writing.
type Graph struct {
	SchemaVersion int         `json:"schemaVersion"`
	GeneratedBy   string      `json:"generatedBy,omitempty"`
	Root          string      `json:"root"`
	Nodes         []Node      `json:"nodes"`
	Edges         []Edge      `json:"edges"`
	Diagnostics   Diagnostics `json:"diagnostics"`
}

// Node is one element of the source tree.
type Node struct {
	ID   string   `json:"id"`
	Kind NodeKind `json:"kind"`
	// Name is for display and is not an identifier: two nodes may share one.
	Name string `json:"name"`
	// Parent is empty only on the root node.
	Parent   string   `json:"parent,omitempty"`
	Language Language `json:"language,omitempty"`
	// Doc is the declaration's own documentation comment, as written.
	//
	// It is carried because stage 2 attributes meaning, and a doc comment is
	// meaning the author already wrote down. Withholding it would make the
	// model infer from names what the source states outright.
	Doc string `json:"doc,omitempty"`
	// Exported says whether the declaration is part of its package's public
	// surface. It is stored rather than derived from the name: Go reads the
	// first letter, Python a leading underscore and JS/TS an export keyword,
	// so only the analyzer knows which rule applied.
	Exported bool       `json:"exported,omitempty"`
	Source   *SourceRef `json:"source,omitempty"`
}

// SourceRef is where a node was found.
//
// Path is relative to the analyzed root, so a graph does not carry the machine
// that produced it.
type SourceRef struct {
	// Path is a file for a declaration and a directory for a package or group.
	Path string `json:"path"`
	// Line is where a declaration starts, and is absent for anything else.
	Line int `json:"line,omitempty"`
	// Lines is how many lines the node spans. For a declaration this cannot be
	// recovered from anything else in the graph, which is why it is kept; for a
	// container it is the total of the files beneath it.
	Lines int `json:"lines,omitempty"`
	// File is one real file standing for this node, needed when Path names a
	// directory. A package is not a blob, so source evidence has to point at
	// something that can be opened.
	File string `json:"file,omitempty"`
}

// Edge is one proven reference. Repeated references raise Weight rather than
// adding a row, so there is at most one edge per from, to and kind.
type Edge struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Kind   EdgeKind `json:"kind"`
	Weight int      `json:"weight"`
}

// Diagnostics is what the analyzer could not do.
//
// This exists because tree-sitter fails soft: it emits an ERROR node and
// carries on, so code can vanish from a graph that otherwise looks complete. A
// failure that is not recorded here cannot be told apart from source that was
// never written.
type Diagnostics struct {
	// FilesParsed counts every file read, whether or not it parsed. The number
	// that succeeded is this minus len(ParseFailures) and is not stored twice.
	FilesParsed int `json:"filesParsed"`
	// ParseFailures holds one entry per failed file, sorted by path.
	ParseFailures []ParseFailure `json:"parseFailures"`
	// UnresolvedReferences is reported rather than gated. Resolution rates
	// differ by language, and a low one is a property of the parser rather
	// than a defect in the document.
	UnresolvedReferences []UnresolvedReference `json:"unresolvedReferences,omitempty"`
}

// ParseFailure is one file the analyzer could not read.
type ParseFailure struct {
	Path     string   `json:"path"`
	Language Language `json:"language,omitempty"`
	Message  string   `json:"message"`
	Line     int      `json:"line,omitempty"`
}

// UnresolvedReference counts references seen but not tied to a node.
type UnresolvedReference struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}
