package goast

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/graph"
)

// Analyzer reads Go source. The zero value is ready to use.
type Analyzer struct{}

// Language names what this analyzer reads.
func (Analyzer) Language() graph.Language { return graph.Go }

// Extensions lists the suffixes it claims.
func (Analyzer) Extensions() []string { return []string{".go"} }

// Analyze satisfies analyze.Analyzer.
func (Analyzer) Analyze(ctx context.Context, root string, opts analyze.Options) (*graph.Graph, error) {
	return Analyze(ctx, root, Options(opts))
}

// Options tunes a walk. It mirrors analyze.Options so this package can be used
// directly without importing the interface it satisfies.
type Options analyze.Options

// Analyze walks root and returns what go/ast could prove about it.
//
// The returned graph is sorted and therefore byte-identical across runs of the
// same binary over the same source. Files that fail to parse are recorded in
// Diagnostics rather than dropped, because a missing declaration and a
// declaration that was never written look identical once the graph is written.
func Analyze(ctx context.Context, root string, opts Options) (*graph.Graph, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("read root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read root %s: not a directory", root)
	}

	a := &analyzer{
		root:         abs,
		includeTests: opts.IncludeTests,
		maxDepth:     opts.MaxDepth,
		excluded:     normalizeExcludes(opts.Exclude),
		packages:     map[string]*pkgInfo{},
		edges:        map[string]*graph.Edge{},
	}
	a.modulePath = readModulePath(abs)

	if err := a.collect(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.resolveCalls()
	return a.build(), nil
}

// normalizeExcludes turns caller-supplied directories into root-relative
// prefixes. Empty entries are dropped, so a list built by splitting a flag does
// not need cleaning first.
func normalizeExcludes(raw []string) []string {
	var out []string
	for _, part := range raw {
		cleaned := strings.Trim(strings.TrimSpace(filepath.ToSlash(part)), "/")
		if cleaned == "" {
			continue
		}
		out = append(out, cleaned)
	}
	return out
}

const (
	graphSchemaVersion = 1
	rootNodeID         = "root"
	generatedBy        = "diagrammer goast"
	// Directories that never describe the module's own architecture.
	vendorDir   = "vendor"
	testdataDir = "testdata"
)

// skippedDirs are never descended into. node_modules appears in repositories
// that mix Go with a JS front end; including it would swamp the graph.
var skippedDirs = map[string]bool{
	vendorDir:      true,
	testdataDir:    true,
	"node_modules": true,
	".git":         true,
}

type unresolvedKind string

const (
	unresolvedBuiltin   unresolvedKind = "builtin-or-local"
	unresolvedForeign   unresolvedKind = "outside-module"
	unresolvedMethod    unresolvedKind = "method-on-value"
	unresolvedAmbiguous unresolvedKind = "ambiguous-method-name"
	unresolvedShape     unresolvedKind = "unsupported-call-shape"
	unresolvedSymbol    unresolvedKind = "symbol-not-a-func"
)

type stats struct {
	Packages         int `json:"packages"`
	Files            int `json:"files"`
	Declarations     int `json:"declarations"`
	ResolvedCalls    int `json:"resolvedCalls"`
	UnresolvedCalls  int `json:"unresolvedCalls"`
	SkippedTestFiles int `json:"skippedTestFiles"`
	ParseErrors      int `json:"parseErrors"`

	Unresolved map[unresolvedKind]int `json:"unresolvedBreakdown,omitempty"`
}

// typeRef names a type by the package that declares it. An empty pkg means
// the package being analyzed; anything else is a module-relative directory.
type typeRef struct {
	pkg  string
	name string
}

func (t typeRef) ok() bool { return t.name != "" }

// declaration is one top-level type or function in a package.
type declaration struct {
	id       string
	name     string
	receiver string
	kind     graph.NodeKind
	file     string
	line     int
	lines    int
	doc      string
	// results are the declared return types, in order. Constructors are how
	// most local variables get their type, and Go constructors almost always
	// return (T, error), so the whole list is kept rather than just a single
	// value: `client, err := NewClient()` has to reach `client.Do()`.
	results []typeRef
}

// pkgInfo accumulates everything known about one directory of Go files.
type pkgInfo struct {
	id        string
	rel       string
	name      string
	files     []string
	fileLines map[string]int
	lines     int
	doc       string
	decls     map[string]*declaration // key: bare name for funcs/types
	methods   map[string][]*declaration
	methodsOn map[string]map[string]*declaration // receiver type -> method name
	fieldsOf  map[string]map[string]typeRef      // struct type -> field -> type
	importsOf map[string]map[string]string       // file -> alias -> import path
}

type analyzer struct {
	root         string
	modulePath   string
	includeTests bool
	maxDepth     int
	excluded     []string

	packages map[string]*pkgInfo // key: rel dir
	order    []string            // deterministic package order
	edges    map[string]*graph.Edge
	// parseFailures records every file that would not parse, one entry each.
	// It is not capped: a cap would make a large broken tree look partly
	// healthy, which is the failure this whole field exists to prevent.
	parseFailures []graph.ParseFailure
	// warnings holds problems that are not a parse failure, such as a
	// directory that could not be read. They are capped, because one
	// unreadable tree can produce an unbounded number of them.
	warnings []string
	stats    stats
}

// readModulePath returns the module path from go.mod, or "" when the root is
// not a module. A missing go.mod is not fatal: a plain directory of Go files
// still produces a useful graph, only cross-package edges cannot be proven.
func readModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

func (a *analyzer) collect(ctx context.Context) error {
	fset := token.NewFileSet()

	err := filepath.WalkDir(a.root, func(abs string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			a.warn("walk %s: %v", a.relative(abs), err)
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if abs != a.root && (skippedDirs[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
			return filepath.SkipDir
		}
		rel := a.relative(abs)
		if a.isExcluded(rel) {
			return filepath.SkipDir
		}
		if a.maxDepth > 0 && depthOf(rel) > a.maxDepth {
			return filepath.SkipDir
		}
		a.collectDir(fset, abs, rel)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk module: %w", err)
	}
	sort.Strings(a.order)
	return nil
}

// parseExcludes normalizes a comma-separated flag into module-relative
// directory prefixes. Empty entries are dropped so "a,,b" behaves as "a,b".
func parseExcludes(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		cleaned := strings.Trim(strings.TrimSpace(filepath.ToSlash(part)), "/")
		if cleaned == "" || cleaned == "." {
			continue
		}
		out = append(out, cleaned)
	}
	sort.Strings(out)
	return out
}

// isExcluded reports whether rel is an excluded directory or sits beneath one.
func (a *analyzer) isExcluded(rel string) bool {
	for _, prefix := range a.excluded {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	return false
}

func depthOf(rel string) int {
	if rel == "." {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

func (a *analyzer) collectDir(fset *token.FileSet, abs, rel string) {
	entries, err := os.ReadDir(abs)
	if err != nil {
		a.warn("read %s: %v", rel, err)
		return
	}

	var info *pkgInfo
	for _, entry := range entries {
		// Only regular files are read. A symlink is not one, and following a
		// symlink named *.go would pull a file from wherever it points into a
		// graph of this tree — recorded under the in-tree name, so nothing in
		// the output would say the content came from somewhere else. A
		// repository that ships such a link decides which of the reader's files
		// end up in their own diagram.
		if entry.IsDir() || !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if !a.includeTests && strings.HasSuffix(entry.Name(), "_test.go") {
			a.stats.SkippedTestFiles++
			continue
		}
		filePath := filepath.Join(abs, entry.Name())
		file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			a.recordParseFailure(path.Join(rel, entry.Name()), err)
			continue
		}
		if info == nil {
			info = a.newPackage(rel)
		}
		a.collectFile(fset, info, rel, entry.Name(), file)
	}
}

func (a *analyzer) newPackage(rel string) *pkgInfo {
	if existing, ok := a.packages[rel]; ok {
		return existing
	}
	info := &pkgInfo{
		id:        packageID(rel),
		rel:       rel,
		fileLines: map[string]int{},
		decls:     map[string]*declaration{},
		methods:   map[string][]*declaration{},
		methodsOn: map[string]map[string]*declaration{},
		fieldsOf:  map[string]map[string]typeRef{},
		importsOf: map[string]map[string]string{},
	}
	a.packages[rel] = info
	a.order = append(a.order, rel)
	return info
}

func (a *analyzer) collectFile(fset *token.FileSet, info *pkgInfo, rel, fileName string, file *ast.File) {
	a.stats.Files++
	info.files = append(info.files, fileName)
	if info.name == "" {
		info.name = file.Name.Name
	}
	if info.doc == "" && file.Doc != nil {
		info.doc = firstSentence(file.Doc.Text())
	}
	if tokenFile := fset.File(file.Pos()); tokenFile != nil {
		info.lines += tokenFile.LineCount()
		info.fileLines[fileName] = tokenFile.LineCount()
	}

	aliases := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		alias := path.Base(importPath)
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				continue
			}
			alias = spec.Name.Name
		}
		aliases[alias] = importPath
		if target, ok := a.internalPackage(importPath); ok && target != rel {
			a.addEdge(info.id, packageID(target), graph.EdgeImport)
		}
	}
	info.importsOf[fileName] = aliases

	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			a.collectFunc(fset, info, rel, fileName, typed, aliases)
		case *ast.GenDecl:
			if typed.Tok == token.TYPE {
				a.collectTypes(fset, info, rel, fileName, typed, aliases)
			}
		}
	}
}

func (a *analyzer) collectFunc(fset *token.FileSet, info *pkgInfo, rel, fileName string, fn *ast.FuncDecl, aliases map[string]string) {
	receiver := receiverName(fn)
	decl := &declaration{
		name:     fn.Name.Name,
		receiver: receiver,
		kind:     graph.KindFunc,
		file:     path.Join(rel, fileName),
		line:     fset.Position(fn.Pos()).Line,
		lines:    fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1,
	}
	if fn.Doc != nil {
		decl.doc = firstSentence(fn.Doc.Text())
	}
	decl.id = funcID(rel, receiver, fn.Name.Name)

	decl.results = resultTypes(fn.Type.Results, aliases)

	if receiver == "" {
		info.decls[fn.Name.Name] = decl
	} else {
		info.methods[fn.Name.Name] = append(info.methods[fn.Name.Name], decl)
		if info.methodsOn[receiver] == nil {
			info.methodsOn[receiver] = map[string]*declaration{}
		}
		info.methodsOn[receiver][fn.Name.Name] = decl
	}
	a.stats.Declarations++
}

func (a *analyzer) collectTypes(fset *token.FileSet, info *pkgInfo, rel, fileName string, group *ast.GenDecl, aliases map[string]string) {
	for _, spec := range group.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		decl := &declaration{
			id:    typeID(rel, typeSpec.Name.Name),
			name:  typeSpec.Name.Name,
			kind:  graph.KindType,
			file:  path.Join(rel, fileName),
			line:  fset.Position(typeSpec.Pos()).Line,
			lines: fset.Position(typeSpec.End()).Line - fset.Position(typeSpec.Pos()).Line + 1,
		}
		if group.Doc != nil {
			decl.doc = firstSentence(group.Doc.Text())
		}
		info.decls[typeSpec.Name.Name] = decl
		// Field types are what let `s.store.Get()` reach Get: the type of the
		// field is written in the struct, so nothing is inferred.
		if structType, ok := typeSpec.Type.(*ast.StructType); ok && structType.Fields != nil {
			fields := map[string]typeRef{}
			for _, field := range structType.Fields.List {
				ref := typeRefOf(field.Type, aliases)
				if !ref.ok() {
					continue
				}
				if len(field.Names) == 0 {
					fields[ref.name] = ref // embedded: the field is named by its type
					continue
				}
				for _, name := range field.Names {
					fields[name.Name] = ref
				}
			}
			if len(fields) > 0 {
				info.fieldsOf[typeSpec.Name.Name] = fields
			}
		}
		a.stats.Declarations++
	}
}

// resultTypes flattens a result list into one entry per returned value, so a
// caller can index it the way a multi-value assignment does.
func resultTypes(results *ast.FieldList, aliases map[string]string) []typeRef {
	if results == nil {
		return nil
	}
	var out []typeRef
	for _, field := range results.List {
		ref := typeRefOf(field.Type, aliases)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			out = append(out, ref)
		}
	}
	return out
}

func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return baseTypeName(fn.Recv.List[0].Type)
}

func baseTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return baseTypeName(typed.X)
	case *ast.IndexExpr: // generic receiver: Foo[T]
		return baseTypeName(typed.X)
	case *ast.IndexListExpr:
		return baseTypeName(typed.X)
	}
	return ""
}

// internalPackage maps an import path back to a module-relative directory.
// Imports outside the module (standard library, third party) return false so
// the graph stays about the analyzed code rather than its dependencies.
func (a *analyzer) internalPackage(importPath string) (string, bool) {
	if a.modulePath == "" {
		return "", false
	}
	if importPath == a.modulePath {
		return ".", true
	}
	suffix, ok := strings.CutPrefix(importPath, a.modulePath+"/")
	if !ok {
		return "", false
	}
	if _, known := a.packages[suffix]; !known {
		// The package may not be walked yet; accept it and let build() drop
		// edges whose endpoints never materialize.
		return suffix, true
	}
	return suffix, true
}

// resolveCalls is a second pass: every package is known by now, so a call can
// be attributed to a concrete declaration instead of guessed at parse time.

// typeRefOf reads a type as it is written in the source. Only forms whose type
// is spelled out are understood; anything else returns an empty ref and the
// caller attributes nothing rather than guessing.
func typeRefOf(expr ast.Expr, aliases map[string]string) typeRef {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typeRef{name: typed.Name}
	case *ast.StarExpr:
		return typeRefOf(typed.X, aliases)
	case *ast.IndexExpr:
		return typeRefOf(typed.X, aliases)
	case *ast.IndexListExpr:
		return typeRefOf(typed.X, aliases)
	case *ast.ParenExpr:
		return typeRefOf(typed.X, aliases)
	case *ast.SelectorExpr:
		ident, ok := typed.X.(*ast.Ident)
		if !ok {
			return typeRef{}
		}
		if importPath, isImport := aliases[ident.Name]; isImport {
			return typeRef{pkg: importPath, name: typed.Sel.Name}
		}
	}
	return typeRef{}
}

// localTypes maps the variables of one function to the types written for them:
// the receiver, parameters, results, `var` declarations, and short declarations
// whose right-hand side names a type or calls a function with one return value.
//
// Block scoping is deliberately ignored, and a name bound to two different
// types anywhere in the function is discarded instead of guessed at. Being
// wrong here would invent a call that the code does not make.
func (a *analyzer) localTypes(info *pkgInfo, aliases map[string]string, fn *ast.FuncDecl) map[string]typeRef {
	env := map[string]typeRef{}
	conflicted := map[string]bool{}

	bind := func(name string, ref typeRef) {
		if name == "" || name == "_" || !ref.ok() || conflicted[name] {
			return
		}
		if existing, seen := env[name]; seen && existing != ref {
			delete(env, name)
			conflicted[name] = true
			return
		}
		env[name] = ref
	}
	bindFields := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			ref := typeRefOf(field.Type, aliases)
			for _, name := range field.Names {
				bind(name.Name, ref)
			}
		}
	}

	bindFields(fn.Recv)
	bindFields(fn.Type.Params)
	bindFields(fn.Type.Results)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch typed := n.(type) {
		case *ast.DeclStmt:
			decl, ok := typed.Decl.(*ast.GenDecl)
			if !ok || decl.Tok != token.VAR {
				return true
			}
			for _, spec := range decl.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, name := range value.Names {
					if value.Type != nil {
						bind(name.Name, typeRefOf(value.Type, aliases))
						continue
					}
					if index < len(value.Values) {
						bind(name.Name, a.inferType(info, aliases, value.Values[index]))
					}
				}
			}
		case *ast.AssignStmt:
			if typed.Tok != token.DEFINE {
				return true
			}
			// `a, b := f()` has one right-hand side for several names, which is
			// the shape almost every Go constructor is used in.
			if len(typed.Rhs) == 1 && len(typed.Lhs) > 1 {
				for index, left := range typed.Lhs {
					if ident, ok := left.(*ast.Ident); ok {
						bind(ident.Name, a.inferTypeAt(info, aliases, typed.Rhs[0], index))
					}
				}
				return true
			}
			if len(typed.Lhs) != len(typed.Rhs) {
				return true
			}
			for index, left := range typed.Lhs {
				ident, ok := left.(*ast.Ident)
				if !ok {
					continue
				}
				bind(ident.Name, a.inferType(info, aliases, typed.Rhs[index]))
			}
		}
		return true
	})
	return env
}

// inferType reads the type of an expression only where the source states it:
// a composite literal, its address, a conversion, or a call to a function whose
// single return type is declared.
func (a *analyzer) inferType(info *pkgInfo, aliases map[string]string, expr ast.Expr) typeRef {
	return a.inferTypeAt(info, aliases, expr, 0)
}

// inferTypeAt reads the type of the value at `index` of an expression. The
// index only matters for a call: `client, err := NewClient()` binds client to
// result 0 and err to result 1.
func (a *analyzer) inferTypeAt(info *pkgInfo, aliases map[string]string, expr ast.Expr, index int) typeRef {
	switch typed := expr.(type) {
	case *ast.CompositeLit:
		if index != 0 {
			return typeRef{}
		}
		return typeRefOf(typed.Type, aliases)
	case *ast.UnaryExpr:
		if typed.Op == token.AND {
			return a.inferTypeAt(info, aliases, typed.X, index)
		}
	case *ast.ParenExpr:
		return a.inferTypeAt(info, aliases, typed.X, index)
	case *ast.CallExpr:
		return a.inferCallType(info, aliases, typed, index)
	}
	return typeRef{}
}

func (a *analyzer) inferCallType(info *pkgInfo, aliases map[string]string, call *ast.CallExpr, index int) typeRef {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if decl, ok := info.decls[fun.Name]; ok {
			if decl.kind == graph.KindType {
				if index != 0 {
					return typeRef{}
				}
				return typeRef{name: decl.name} // a conversion: T(x)
			}
			return a.absolute(info, resultAt(decl, index))
		}
	case *ast.SelectorExpr:
		ident, ok := fun.X.(*ast.Ident)
		if !ok {
			return typeRef{}
		}
		importPath, isImport := aliases[ident.Name]
		if !isImport {
			return typeRef{}
		}
		rel, internal := a.internalPackage(importPath)
		if !internal {
			return typeRef{}
		}
		target, known := a.packages[rel]
		if !known {
			return typeRef{}
		}
		decl, found := target.decls[fun.Sel.Name]
		if !found {
			return typeRef{}
		}
		if decl.kind == graph.KindType {
			if index != 0 {
				return typeRef{}
			}
			return typeRef{pkg: importPath, name: decl.name}
		}
		return a.absolute(target, resultAt(decl, index))
	}
	return typeRef{}
}

func resultAt(decl *declaration, index int) typeRef {
	if index < 0 || index >= len(decl.results) {
		return typeRef{}
	}
	return decl.results[index]
}

// absolute rewrites a package-local type ref into one that names its package,
// so a type flowing out of another package stays attributed to that package.
func (a *analyzer) absolute(owner *pkgInfo, ref typeRef) typeRef {
	if !ref.ok() || ref.pkg != "" {
		return ref
	}
	if owner.rel == "." {
		return typeRef{pkg: a.modulePath, name: ref.name}
	}
	if a.modulePath == "" {
		return ref
	}
	return typeRef{pkg: a.modulePath + "/" + owner.rel, name: ref.name}
}

// fieldType reads the declared type of `field` on `ref`, in whichever package
// declares that type.
func (a *analyzer) fieldType(info *pkgInfo, ref typeRef, field string) typeRef {
	owner := a.ownerOf(info, ref)
	if owner == nil {
		return typeRef{}
	}
	fields, ok := owner.fieldsOf[ref.name]
	if !ok {
		return typeRef{}
	}
	return a.absolute(owner, fields[field])
}

// ownerOf resolves the package that declares a type ref, or nil when the type
// lives outside the module.
func (a *analyzer) ownerOf(info *pkgInfo, ref typeRef) *pkgInfo {
	if !ref.ok() {
		return nil
	}
	if ref.pkg == "" {
		return info
	}
	rel, internal := a.internalPackage(ref.pkg)
	if !internal {
		return nil
	}
	return a.packages[rel]
}

// methodOn finds the declaration of `name` on `ref`, in whichever package
// declares that type. Nothing is returned unless that exact method exists.
func (a *analyzer) methodOn(info *pkgInfo, ref typeRef, name string) *declaration {
	owner := a.ownerOf(info, ref)
	if owner == nil {
		return nil
	}
	if methods, ok := owner.methodsOn[ref.name]; ok {
		return methods[name]
	}
	return nil
}

// receiverRef reads the type of the value a method is being called on, for the
// two shapes the source states outright: a named variable, and one field step
// from a named variable (`s.store.Get()`).
func (a *analyzer) receiverRef(info *pkgInfo, env map[string]typeRef, expr ast.Expr) typeRef {
	switch typed := expr.(type) {
	case *ast.Ident:
		return env[typed.Name]
	case *ast.ParenExpr:
		return a.receiverRef(info, env, typed.X)
	case *ast.SelectorExpr:
		base := a.receiverRef(info, env, typed.X)
		if !base.ok() {
			return typeRef{}
		}
		return a.fieldType(info, base, typed.Sel.Name)
	}
	return typeRef{}
}

func (a *analyzer) resolveCalls() {
	fset := token.NewFileSet()
	for _, rel := range a.order {
		info := a.packages[rel]
		for _, fileName := range info.files {
			filePath := filepath.Join(a.root, filepath.FromSlash(rel), fileName)
			file, err := parser.ParseFile(fset, filePath, nil, parser.SkipObjectResolution)
			if err != nil {
				continue
			}
			a.resolveFileCalls(info, fileName, file)
		}
	}
}

func (a *analyzer) resolveFileCalls(info *pkgInfo, fileName string, file *ast.File) {
	aliases := info.importsOf[fileName]
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fromID := funcID(info.rel, receiverName(fn), fn.Name.Name)
		env := a.localTypes(info, aliases, fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			a.resolveCall(info, aliases, env, fromID, call)
			return true
		})
	}
}

func (a *analyzer) noteUnresolved(kind unresolvedKind) {
	a.stats.UnresolvedCalls++
	if a.stats.Unresolved == nil {
		a.stats.Unresolved = map[unresolvedKind]int{}
	}
	a.stats.Unresolved[kind]++
}

func (a *analyzer) resolveCall(info *pkgInfo, aliases map[string]string, env map[string]typeRef, fromID string, call *ast.CallExpr) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if target, ok := info.decls[fun.Name]; ok && target.kind == graph.KindFunc {
			a.addEdge(fromID, target.id, graph.EdgeCall)
			a.stats.ResolvedCalls++
			return
		}
		a.noteUnresolved(unresolvedBuiltin)
	case *ast.SelectorExpr:
		ident, ok := fun.X.(*ast.Ident)
		if !ok {
			// Not a bare name: try one field step, which is how a dependency
			// held on a struct is called.
			if ref := a.receiverRef(info, env, fun.X); ref.ok() {
				if decl := a.methodOn(info, ref, fun.Sel.Name); decl != nil {
					a.addEdge(fromID, decl.id, graph.EdgeCall)
					a.stats.ResolvedCalls++
					return
				}
			}
			a.noteUnresolved(unresolvedShape)
			return
		}
		if importPath, ok := aliases[ident.Name]; ok {
			a.resolveCrossPackage(fromID, importPath, fun.Sel.Name)
			return
		}
		// Not an import alias, so this is a method on a value. Prefer the type
		// the source declared for that variable; it names exactly one method.
		if ref, known := env[ident.Name]; known {
			if decl := a.methodOn(info, ref, fun.Sel.Name); decl != nil {
				a.addEdge(fromID, decl.id, graph.EdgeCall)
				a.stats.ResolvedCalls++
				return
			}
		}
		// There is deliberately no fallback to "the only method with this name
		// in the package". It reads as a near-miss heuristic but it is a guess:
		// a call on an interface-typed value was attributed to an unrelated
		// concrete type that merely shared the method name. Without a declared
		// type for the value, nothing is claimed.
		if len(info.methods[fun.Sel.Name]) > 1 {
			a.noteUnresolved(unresolvedAmbiguous)
			return
		}
		a.noteUnresolved(unresolvedMethod)
	default:
		a.noteUnresolved(unresolvedShape)
	}
}

func (a *analyzer) resolveCrossPackage(fromID, importPath, symbol string) {
	rel, ok := a.internalPackage(importPath)
	if !ok {
		a.noteUnresolved(unresolvedForeign)
		return
	}
	target, known := a.packages[rel]
	if !known {
		a.noteUnresolved(unresolvedForeign)
		return
	}
	if decl, ok := target.decls[symbol]; ok && decl.kind == graph.KindFunc {
		a.addEdge(fromID, decl.id, graph.EdgeCall)
		a.stats.ResolvedCalls++
		return
	}
	// The symbol exists in another package but is a type conversion, a
	// variable, or an overloaded method name. The import edge already records
	// the dependency, so nothing finer is claimed here.
	a.noteUnresolved(unresolvedSymbol)
}

func (a *analyzer) addEdge(from, to string, kind graph.EdgeKind) {
	if from == "" || to == "" || from == to {
		return
	}
	key := string(kind) + "\x00" + from + "\x00" + to
	if existing, ok := a.edges[key]; ok {
		existing.Weight++
		return
	}
	a.edges[key] = &graph.Edge{From: from, To: to, Kind: kind, Weight: 1}
}

// ensureAncestors creates grouping nodes for directories that hold no Go files
// themselves, so every package can be reached from the root by walking parents.
func (a *analyzer) ensureAncestors(nodes map[string]graph.Node, rel string) {
	if rel == "." {
		return
	}
	segments := strings.Split(rel, "/")
	for i := 1; i < len(segments); i++ {
		prefix := strings.Join(segments[:i], "/")
		id := packageID(prefix)
		if _, ok := nodes[id]; ok {
			continue
		}
		if _, isPackage := a.packages[prefix]; isPackage {
			continue
		}
		nodes[id] = graph.Node{
			ID:     id,
			Kind:   graph.KindGroup,
			Name:   segments[i-1],
			Parent: parentID(prefix),
			Source: &graph.SourceRef{
				Path: prefix,
				// A grouping directory holds no Go files of its own, so it
				// borrows one from the first package beneath it. Source
				// evidence needs something that can be opened, and every box
				// on a level has to be able to name one.
				File: a.descendantFile(prefix),
			},
		}
	}
}

// descendantFile returns a representative file from the first package below a
// grouping directory, in the deterministic package order.
func (a *analyzer) descendantFile(prefix string) string {
	for _, rel := range a.order {
		if !strings.HasPrefix(rel, prefix+"/") {
			continue
		}
		if file := representativeFile(rel, a.packages[rel].files); file != "" {
			return file
		}
	}
	return ""
}

// representativeFile picks the file that best stands for a package: its
// doc.go, else the file named after the directory, else the first by name.
func representativeFile(rel string, files []string) string {
	if len(files) == 0 {
		return ""
	}
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)
	base := path.Base(rel) + ".go"
	for _, candidate := range []string{"doc.go", base} {
		for _, file := range sorted {
			if file == candidate {
				return path.Join(rel, file)
			}
		}
	}
	return path.Join(rel, sorted[0])
}

func sortedDeclarations(info *pkgInfo) []*declaration {
	all := make([]*declaration, 0, len(info.decls))
	for _, decl := range info.decls {
		all = append(all, decl)
	}
	for _, group := range info.methods {
		all = append(all, group...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].id < all[j].id })
	return all
}

func (a *analyzer) relative(abs string) string {
	rel, err := filepath.Rel(a.root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// recordParseFailure notes a file go/ast refused, with the position it refused
// at when the error carries one.
//
// The position matters: a parse error without a line sends a reader to open the
// file and search, which is exactly the friction that gets a diagnostic ignored.
func (a *analyzer) recordParseFailure(rel string, err error) {
	failure := graph.ParseFailure{
		Path:     rel,
		Language: graph.Go,
		Message:  err.Error(),
	}
	var list scanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		failure.Message = list[0].Msg
		if line := list[0].Pos.Line; line > 0 {
			failure.Line = line
		}
	}
	a.parseFailures = append(a.parseFailures, failure)
}

func (a *analyzer) warn(format string, args ...any) {
	const maxWarnings = 50
	if len(a.warnings) >= maxWarnings {
		return
	}
	a.warnings = append(a.warnings, fmt.Sprintf(format, args...))
}

func fileID(rel string) string {
	return "file:" + rel
}

func packageID(rel string) string {
	if rel == "." || rel == "" {
		return rootNodeID
	}
	return "pkg:" + rel
}

func parentID(rel string) string {
	if rel == "." || rel == "" {
		return ""
	}
	parent := path.Dir(rel)
	if parent == "." || parent == "/" {
		return rootNodeID
	}
	return packageID(parent)
}

func funcID(rel, receiver, name string) string {
	if receiver == "" {
		return "fn:" + rel + "." + name
	}
	return "fn:" + rel + ".(" + receiver + ")." + name
}

func typeID(rel, name string) string {
	return "ty:" + rel + "." + name
}

func packageLabel(rel, name string) string {
	if rel == "." || rel == "" {
		if name != "" {
			return name
		}
		return rootNodeID
	}
	base := path.Base(rel)
	if name != "" && name != base {
		return base + " (" + name + ")"
	}
	return base
}

func declarationLabel(decl *declaration) string {
	if decl.receiver == "" {
		return decl.name
	}
	return decl.receiver + "." + decl.name
}

// firstSentence keeps doc comments short enough to sit in a diagram card.
func firstSentence(doc string) string {
	const maxDocRunes = 140
	trimmed := strings.TrimSpace(doc)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, ". "); idx > 0 {
		trimmed = trimmed[:idx+1]
	}
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	runes := []rune(trimmed)
	if len(runes) > maxDocRunes {
		return string(runes[:maxDocRunes-1]) + "…"
	}
	return trimmed
}

// build assembles the graph: the directory tree first so every declaration has
// a parent, then the declarations, then the edges whose endpoints both exist.
//
// Everything is sorted before it is returned. That ordering is what makes the
// output byte-identical across runs, so it is part of the contract rather than
// a tidy habit.
func (a *analyzer) build() *graph.Graph {
	nodes := map[string]graph.Node{}
	// The root is a group rather than a package, and carries no language: a
	// directory may hold more than one, and a second analyzer merging into this
	// graph must not find the root already claimed.
	nodes[rootNodeID] = graph.Node{
		ID:   rootNodeID,
		Kind: graph.KindGroup,
		Name: filepath.Base(a.root),
	}

	for _, rel := range a.order {
		info := a.packages[rel]
		a.ensureAncestors(nodes, rel)
		nodes[info.id] = graph.Node{
			ID:       info.id,
			Kind:     graph.KindPackage,
			Name:     packageLabel(rel, info.name),
			Parent:   parentID(rel),
			Language: graph.Go,
			Doc:      info.doc,
			Source: &graph.SourceRef{
				Path:  rel,
				Lines: info.lines,
				File:  representativeFile(rel, info.files),
			},
		}
		// Declarations hang off their file, not the package. A Go package is a
		// flat list of hundreds of declarations, which no single view can hold;
		// the file is the grouping the language already gives, and its name is
		// one a reader recognizes.
		for _, fileName := range info.files {
			filePath := path.Join(info.rel, fileName)
			nodes[fileID(filePath)] = graph.Node{
				ID:       fileID(filePath),
				Kind:     graph.KindFile,
				Name:     fileName,
				Parent:   info.id,
				Language: graph.Go,
				Source: &graph.SourceRef{
					Path:  filePath,
					Lines: info.fileLines[fileName],
					File:  filePath,
				},
			}
		}
		for _, decl := range sortedDeclarations(info) {
			nodes[decl.id] = graph.Node{
				ID:       decl.id,
				Kind:     decl.kind,
				Name:     declarationLabel(decl),
				Parent:   fileID(decl.file),
				Language: graph.Go,
				Doc:      decl.doc,
				Exported: ast.IsExported(decl.name),
				Source: &graph.SourceRef{
					Path:  decl.file,
					Line:  decl.line,
					Lines: decl.lines,
					File:  decl.file,
				},
			}
		}
	}

	ordered := make([]graph.Node, 0, len(nodes))
	for _, n := range nodes {
		ordered = append(ordered, n)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	edges := make([]graph.Edge, 0, len(a.edges))
	for _, e := range a.edges {
		if _, ok := nodes[e.From]; !ok {
			continue
		}
		if _, ok := nodes[e.To]; !ok {
			continue
		}
		edges = append(edges, *e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})

	return &graph.Graph{
		SchemaVersion: graphSchemaVersion,
		GeneratedBy:   generatedBy,
		Root:          rootNodeID,
		Nodes:         ordered,
		Edges:         edges,
		Diagnostics:   a.diagnostics(),
	}
}

// diagnostics reports what the walk could not do.
//
// The parse failures are the point of it. A count on its own cannot be acted
// on, and a graph with a file silently missing looks exactly like a graph of a
// project that never had it.
func (a *analyzer) diagnostics() graph.Diagnostics {
	failures := make([]graph.ParseFailure, len(a.parseFailures))
	copy(failures, a.parseFailures)
	sort.Slice(failures, func(i, j int) bool { return failures[i].Path < failures[j].Path })

	unresolved := make([]graph.UnresolvedReference, 0, len(a.stats.Unresolved))
	for kind, count := range a.stats.Unresolved {
		unresolved = append(unresolved, graph.UnresolvedReference{Reason: string(kind), Count: count})
	}
	// Sorted by reason, because ranging a map is deliberately unordered in Go
	// and an unordered field would break byte-identity across runs.
	sort.Slice(unresolved, func(i, j int) bool { return unresolved[i].Reason < unresolved[j].Reason })

	return graph.Diagnostics{
		// Every file the walk opened, whether or not it parsed. The number that
		// succeeded is this minus the failures, and is not stored twice.
		FilesParsed:          a.stats.Files + len(failures),
		ParseFailures:        failures,
		UnresolvedReferences: unresolved,
	}
}
