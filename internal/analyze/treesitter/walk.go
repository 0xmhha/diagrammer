//go:build cgo

package treesitter

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"

	"github.com/0xmhha/diagrammer/internal/graph"
)

// collect walks the tree and reads every file this language claims.
func (w *walker) collect(ctx context.Context) error {
	w.unresolved = map[string]int{}

	err := filepath.WalkDir(w.root, func(abs string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			w.warn("walk %s: %v", w.relative(abs), err)
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if abs != w.root && (skippedDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			rel := w.relative(abs)
			if w.isExcluded(rel) || w.tooDeep(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		// Only regular files are read. A symlink is not one, and following a
		// symlink into a file outside the tree would put that file's contents
		// in a graph of this one, recorded under an in-tree name.
		if !entry.Type().IsRegular() || !w.claims(entry.Name()) {
			return nil
		}
		w.readFile(ctx, abs)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", w.root, err)
	}
	return nil
}

func (w *walker) claims(name string) bool {
	for _, ext := range w.lang.extensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func (w *walker) readFile(ctx context.Context, abs string) {
	rel := w.relative(abs)
	source, err := os.ReadFile(abs)
	if err != nil {
		w.warn("read %s: %v", rel, err)
		return
	}
	w.filesRead++

	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(w.lang.grammar()))

	tree, err := parser.ParseString(ctx, nil, source)
	if err != nil {
		w.recordFailure(rel, 0, err.Error())
		return
	}
	defer tree.Close()

	root := tree.RootNode()
	// tree-sitter does not refuse a file it cannot parse; it emits an ERROR
	// node and carries on. Asking is the only way to know, and a file that
	// parsed with errors has code missing from the graph with nothing else to
	// say so.
	if root.HasError() {
		line, detail := firstError(root, source)
		w.recordFailure(rel, line, detail)
	}

	dir := path.Dir(rel)
	if dir == "." {
		dir = "."
	}
	w.walkNode(root, source, rel, dir, "")
}

// firstError finds where the grammar first lost its footing, and says what it
// lost it on.
//
// The message used to be the word "unparsable", which sends a reader to a line
// and leaves them to work out the rest. Two files in the reference tree this
// project reads are reported here, and what is at both is a NUL byte used as a
// separator inside a template literal:
//
//	const key = `${from}` + "\x00" + `${to}`;
//
// Node accepts that and the grammar does not, so the file is valid JavaScript
// that this parser cannot read all of. A reader told only "unparsable" has no
// way to reach that conclusion; told the column and the kind of thing, they can
// look and see it in a second.
//
// The offending text is deliberately not quoted. It is a byte the grammar could
// not place, which is exactly the kind of byte that should not be pasted into a
// message somebody's terminal will render.
func firstError(n sitter.Node, source []byte) (int, string) {
	if n.IsMissing() {
		return int(n.StartPoint().Row) + 1,
			fmt.Sprintf("column %d: a %s is missing here", n.StartPoint().Column+1, n.Type())
	}
	if n.IsError() {
		return int(n.StartPoint().Row) + 1,
			fmt.Sprintf("column %d: the grammar does not accept what is here", n.StartPoint().Column+1)
	}
	for i := range int(n.ChildCount()) {
		if line, detail := firstError(n.Child(uint32(i)), source); line > 0 {
			return line, detail
		}
	}
	return 0, ""
}

// walkNode records every declaration the grammar named, and the calls made
// inside each one.
func (w *walker) walkNode(n sitter.Node, source []byte, file, dir, enclosing string) {
	kind, isDecl := w.lang.decls[n.Type()]
	current := enclosing

	if isDecl {
		if name := declName(n, source); name != "" {
			decl := declaration{
				id:       declID(dir, file, name),
				kind:     kind,
				name:     name,
				file:     file,
				line:     int(n.StartPoint().Row) + 1,
				lines:    int(n.EndPoint().Row-n.StartPoint().Row) + 1,
				doc:      w.docFor(n, source),
				exported: w.isExported(n, name),
			}
			w.declsIn[dir] = append(w.declsIn[dir], decl)
			w.byName[name] = append(w.byName[name], decl.id)
			current = decl.id
		}
	}

	if w.lang.imports[n.Type()] && current == "" {
		if target := importTarget(n, source); target != "" {
			w.noteImport(dir, target)
		}
	}
	if w.lang.calls[n.Type()] && current != "" {
		if name := calleeName(n, source); name != "" {
			w.noteCall(current, name)
		}
	}

	for i := range int(n.ChildCount()) {
		w.walkNode(n.Child(uint32(i)), source, file, dir, current)
	}
}

// declName reads a declaration's name.
//
// Every grammar here puts it in a field called name, except where a declaration
// wraps another node that carries it.
func declName(n sitter.Node, source []byte) string {
	if id := n.ChildByFieldName("name"); !id.IsNull() {
		return strings.TrimSpace(id.Content(source))
	}
	for i := range int(n.ChildCount()) {
		child := n.Child(uint32(i))
		switch child.Type() {
		case "identifier", "type_identifier", "property_identifier":
			return strings.TrimSpace(child.Content(source))
		}
	}
	return ""
}

func calleeName(n sitter.Node, source []byte) string {
	fn := n.ChildByFieldName("function")
	if fn.IsNull() {
		return ""
	}
	text := strings.TrimSpace(fn.Content(source))
	// A qualified call keeps only its last part. Resolving the qualifier needs
	// types, which no grammar here has, and the last part is what a later stage
	// can still match on.
	if i := strings.LastIndexAny(text, ".:"); i >= 0 && i+1 < len(text) {
		text = text[i+1:]
	}
	if text == "" || strings.ContainsAny(text, "()[] \t\n") {
		return ""
	}
	return text
}

// importTarget reads the module an import names.
//
// From the grammar rather than from the text. Every grammar here puts the
// module in a field or in a string child, and cutting the statement apart with
// string surgery instead is how every import in a JavaScript tree came out as
// "{": `import { x } from './a.mjs'` trimmed down to the first word after the
// keyword, which is a brace.
func importTarget(n sitter.Node, source []byte) string {
	for _, field := range []string{"source", "module_name", "name", "path"} {
		if child := n.ChildByFieldName(field); !child.IsNull() {
			return unquote(child.Content(source))
		}
	}
	// Solidity and Python's plain form carry it as a child rather than a field.
	if found := firstStringOrName(n, source); found != "" {
		return found
	}
	return ""
}

func firstStringOrName(n sitter.Node, source []byte) string {
	for i := range int(n.NamedChildCount()) {
		child := n.NamedChild(uint32(i))
		switch child.Type() {
		case "string", "string_literal":
			return unquote(child.Content(source))
		case "dotted_name", "identifier", "relative_import":
			return unquote(child.Content(source))
		}
	}
	return ""
}

func unquote(text string) string {
	text = strings.TrimSpace(text)
	for _, q := range []string{`"`, "'", "`"} {
		text = strings.Trim(text, q)
	}
	return strings.TrimSpace(text)
}

// docFor reads a declaration's documentation, wherever its language keeps it.
func (w *walker) docFor(n sitter.Node, source []byte) string {
	switch w.lang.docStyle {
	case docFirstString:
		return pythonDocstring(n, source)
	default:
		return precedingComment(n, source)
	}
}

func pythonDocstring(n sitter.Node, source []byte) string {
	body := n.ChildByFieldName("body")
	if body.IsNull() || body.NamedChildCount() == 0 {
		return ""
	}
	first := body.NamedChild(0)
	if first.Type() != "expression_statement" || first.NamedChildCount() == 0 {
		return ""
	}
	literal := first.NamedChild(0)
	if literal.Type() != "string" {
		return ""
	}
	return cleanDoc(strings.Trim(literal.Content(source), `"'`))
}

func precedingComment(n sitter.Node, source []byte) string {
	// A comment sits above whatever the reader sees, and what the reader sees
	// may be a wrapper: `export function f` is an export statement holding a
	// function declaration, and the comment is the export's sibling rather than
	// the function's. Climbing to the outermost wrapper first is what stops
	// every exported declaration from arriving undocumented.
	for parent := n.Parent(); !parent.IsNull(); parent = parent.Parent() {
		if !strings.HasPrefix(parent.Type(), "export") {
			break
		}
		n = parent
	}

	var lines []string
	for prev := n.PrevSibling(); !prev.IsNull() && prev.Type() == "comment"; prev = prev.PrevSibling() {
		text := strings.TrimSpace(prev.Content(source))
		text = strings.TrimPrefix(text, "///")
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/**")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimPrefix(text, "/*")
		lines = append([]string{strings.TrimSpace(text)}, lines...)
	}
	return cleanDoc(strings.Join(lines, "\n"))
}

// cleanDoc trims a comment down to what a reader would call the documentation,
// and caps it. The schema caps it too; doing it here keeps the graph from
// carrying a licence header that happened to sit above a declaration.
func cleanDoc(text string) string {
	const limit = 1024
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*")))
	}
	joined := strings.TrimSpace(strings.Join(out, "\n"))
	if len(joined) > limit {
		return joined[:limit]
	}
	return joined
}

// isExported answers the question each language answers differently.
func (w *walker) isExported(n sitter.Node, name string) bool {
	if w.lang.name == graph.JavaScript || w.lang.name == graph.TypeScript {
		// Visibility is a keyword above the declaration rather than anything in
		// the name, so the name cannot answer it.
		for parent := n.Parent(); !parent.IsNull(); parent = parent.Parent() {
			if strings.HasPrefix(parent.Type(), "export") {
				return true
			}
		}
		return false
	}
	return w.lang.exported(name)
}

// noteImport records an import to be resolved once the walk knows what is in
// the tree.
//
// It cannot be resolved as it is found: whether a target names a directory this
// walk read is not known until the walk is over, and without that check every
// bare specifier — node:fs, a package from the registry — resolves to the root
// and the graph fills with edges to nowhere.
func (w *walker) noteImport(fromDir, target string) {
	w.pendingImports = append(w.pendingImports, pendingImport{fromDir: fromDir, target: target})
}

// resolveImport turns an import's text into the directory it names, or nothing.
//
// Languages disagree about what an import string is, and reading them all the
// same way is how every edge in a JavaScript tree went missing: `./shared/x.mjs`
// was being resolved against the analyzed root rather than against the file
// that wrote it, so it never matched anything.
//
// Nothing is guessed. A target that does not resolve to a directory in this
// tree is counted in the diagnostics rather than pointed at whatever it most
// resembles: an edge to the wrong package is worse than a missing one, because
// a reader cannot tell it is wrong.
func (w *walker) resolveImport(fromDir, target string) string {
	if target == "" {
		return ""
	}
	switch w.lang.name {
	case graph.Python:
		// A dotted module name is a path from the root. A leading dot is a
		// relative import, and each one climbs a level.
		climbed := fromDir
		for strings.HasPrefix(target, ".") {
			target = target[1:]
			if climbed != "." && climbed != "" {
				climbed = path.Dir(climbed)
			}
		}
		module := strings.ReplaceAll(target, ".", "/")
		if climbed != "." && climbed != "" {
			return clean(path.Join(climbed, module))
		}
		return clean(module)
	default:
		// The rest name files. A relative one is relative to whoever wrote it,
		// which is the part that was wrong.
		target = trimSourceSuffix(target)
		if strings.HasPrefix(target, ".") {
			return clean(path.Join(fromDir, target))
		}
		return clean(target)
	}
}

func trimSourceSuffix(target string) string {
	for _, ext := range []string{".sol", ".mjs", ".cjs", ".jsx", ".tsx", ".js", ".ts"} {
		if strings.HasSuffix(target, ext) {
			return strings.TrimSuffix(target, ext)
		}
	}
	return target
}

func clean(p string) string {
	p = strings.Trim(path.Clean(p), "/")
	if p == "" || p == "." {
		return "."
	}
	return p
}

func (w *walker) noteCall(from, name string) {
	w.pendingCalls = append(w.pendingCalls, pendingCall{from: from, name: name})
}

func (w *walker) warn(format string, args ...any) {
	if len(w.warnings) >= maxWarnings {
		return
	}
	w.warnings = append(w.warnings, fmt.Sprintf(format, args...))
}

func (w *walker) recordFailure(rel string, line int, message string) {
	w.parseFailures = append(w.parseFailures, graph.ParseFailure{
		Path: rel, Language: w.lang.name, Message: message, Line: line,
	})
}

func (w *walker) relative(abs string) string {
	rel, err := filepath.Rel(w.root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func (w *walker) isExcluded(rel string) bool {
	for _, prefix := range w.excluded {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	return false
}

func (w *walker) tooDeep(rel string) bool {
	if w.opts.MaxDepth <= 0 || rel == "." {
		return false
	}
	return strings.Count(rel, "/")+1 > w.opts.MaxDepth
}

func packageID(dir string) string {
	if dir == "." || dir == "" {
		return rootNodeID
	}
	return "pkg:" + dir
}

func fileID(rel string) string { return "file:" + rel }

func declID(dir, file, name string) string {
	return "decl:" + file + "." + name
}

func (w *walker) addEdge(from, to string, kind graph.EdgeKind) {
	if from == "" || to == "" || from == to {
		return
	}
	key := string(kind) + "\x00" + from + "\x00" + to
	if existing, ok := w.edges[key]; ok {
		existing.Weight++
		return
	}
	w.edges[key] = &graph.Edge{From: from, To: to, Kind: kind, Weight: 1}
}
