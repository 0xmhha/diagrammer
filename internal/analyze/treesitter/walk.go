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
		w.readFile(abs)
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

func (w *walker) readFile(abs string) {
	rel := w.relative(abs)
	source, err := os.ReadFile(abs)
	if err != nil {
		w.warn("read %s: %v", rel, err)
		return
	}
	w.filesRead++

	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(w.lang.grammar()))

	tree, err := parser.ParseString(context.Background(), nil, source)
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

// firstError finds where the grammar first lost its footing, so the report
// sends a reader to a line rather than to a file.
func firstError(n sitter.Node, source []byte) (int, string) {
	if n.IsError() || n.IsMissing() {
		what := "unparsable"
		if n.IsMissing() {
			what = "missing " + n.Type()
		}
		return int(n.StartPoint().Row) + 1, what
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

func importTarget(n sitter.Node, source []byte) string {
	text := strings.TrimSpace(n.Content(source))
	for _, prefix := range []string{"import ", "from ", "import"} {
		text = strings.TrimPrefix(text, prefix)
	}
	if i := strings.IndexAny(text, "\n;"); i >= 0 {
		text = text[:i]
	}
	if i := strings.Index(text, " import "); i >= 0 {
		text = text[:i]
	}
	text = strings.Trim(strings.TrimSpace(text), `"'`)
	if i := strings.Index(text, " "); i >= 0 {
		text = text[:i]
	}
	return strings.Trim(text, `"'`)
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

func (w *walker) noteImport(fromDir, target string) {
	w.addEdge(packageID(fromDir), packageID(normalizeTarget(target)), graph.EdgeImport)
}

// normalizeTarget turns an import's text into something that might name a
// directory in this tree. A relative path is kept; anything else is left alone
// and simply will not resolve, which is reported rather than guessed at.
func normalizeTarget(target string) string {
	target = strings.TrimSuffix(target, ".sol")
	target = strings.TrimSuffix(target, ".js")
	target = strings.TrimSuffix(target, ".ts")
	target = strings.ReplaceAll(target, ".", "/")
	return strings.Trim(path.Clean(target), "/")
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
