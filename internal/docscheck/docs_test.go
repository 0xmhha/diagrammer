package docscheck_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..")
}

func read(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

// constantsIn extracts the integer constants a file declares, by parsing it.
//
// Parsing beats grepping here for the same reason the artifact is parsed rather
// than pattern-matched: the moment somebody reformats a const block, a regular
// expression stops seeing what it was watching, and stops complaining.
func constantsIn(t *testing.T, rel string) map[string]string {
	t.Helper()
	path := filepath.Join(repoRoot(t), rel)
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	out := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Values) != len(value.Names) {
				continue
			}
			for i, name := range value.Names {
				lit, ok := value.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					continue
				}
				out[name.Name] = lit.Value
			}
		}
	}
	return out
}

// TestEveryThresholdIsDocumentedWithItsValue is the check the release checklist
// asked a person to do.
//
// A threshold is a decision, and docs/thresholds.md exists so that changing one
// is done the way it was arrived at rather than by taste. A document quoting a
// number the code no longer uses is worse than no document: it is a wrong
// answer that looks researched.
func TestEveryThresholdIsDocumentedWithItsValue(t *testing.T) {
	sources := []string{
		"internal/compose/component.go",
		"internal/invariant/composition.go",
		"internal/render/layout.go",
	}
	// Only the thresholds are documented. A constant that is plumbing rather
	// than a decision does not belong in a document about decisions.
	documented := map[string]bool{
		"minBoxesPerLevel": true, "expandedMaxBoxes": true,
		"minSegment": true, "separation": true, "LabelClearance": true, "borderRun": true,
		"channelX": true, "channelY": true, "laneGap": true, "stub": true,
		"channelMaxLanes": true,
	}

	doc := read(t, "docs/thresholds.md")
	found := map[string]bool{}

	for _, source := range sources {
		for name, value := range constantsIn(t, source) {
			if !documented[name] {
				continue
			}
			found[name] = true
			// The document may write a pixel value as "16px" or a count as
			// "16"; both have to carry the number the code uses.
			pattern := regexp.MustCompile("`" + regexp.QuoteMeta(name) + "` = " + regexp.QuoteMeta(value) + `(px)?[^0-9]`)
			if !pattern.MatchString(doc) {
				t.Errorf("docs/thresholds.md does not say %s is %s, which is what %s uses",
					name, value, source)
			}
		}
	}

	for name := range documented {
		if !found[name] {
			t.Errorf("docs/thresholds.md documents %q, which no source declares any more", name)
		}
	}
}

// TestEveryRuleIsNamedInTheContract keeps docs/invariants.md describing the
// rules that exist.
//
// A rule the contract does not mention is one nobody agreed to; a rule the
// contract describes and the code does not have is a promise.
func TestEveryRuleIsNamedInTheContract(t *testing.T) {
	doc := read(t, "docs/invariants.md")

	// The rule names are the string values, not the Go identifiers: the
	// document is written for a reader, and the reader sees the values.
	for _, source := range []string{
		"internal/invariant/common.go",
		"internal/invariant/composition.go",
		"internal/invariant/ladder.go",
		"internal/invariant/families.go",
	} {
		for _, rule := range stringConstantsIn(t, source) {
			// The rule that fires for a family with no rules is machinery
			// rather than a promise about a drawing, so it is not in the
			// contract.
			if rule == "unknown-family" {
				continue
			}
			// The name has to appear as a name, in backticks, rather than
			// anywhere inside a longer word. Substring matching would let
			// `border-run` be satisfied by `border-runner`, and a check that
			// can be satisfied by accident is not one.
			if !strings.Contains(doc, "`"+rule+"`") {
				t.Errorf("rule %q is not named in docs/invariants.md", rule)
			}
		}
	}
}

func stringConstantsIn(t *testing.T, rel string) []string {
	t.Helper()
	path := filepath.Join(repoRoot(t), rel)
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	var out []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if !strings.HasPrefix(name.Name, "Rule") || i >= len(value.Values) {
					continue
				}
				lit, ok := value.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				out = append(out, strings.Trim(lit.Value, `"`))
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestTheReadmeListsTheCommandsThatExist keeps the front page describing the
// program. Someone reading it is deciding whether to run the thing.
func TestTheReadmeListsTheCommandsThatExist(t *testing.T) {
	readme := read(t, "README.md")
	main := read(t, "cmd/diagrammer/main.go")

	pattern := regexp.MustCompile(`\{"([a-z]+)", "`)
	var commands []string
	for _, match := range pattern.FindAllStringSubmatch(main, -1) {
		commands = append(commands, match[1])
	}
	if len(commands) == 0 {
		t.Fatal("no commands were found in main.go, so this test is watching nothing")
	}

	for _, name := range commands {
		if name == "version" {
			continue // about the binary rather than about the pipeline
		}
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("the README never mentions the %q command", name)
		}
	}
}

// TestTheScopeIsStatedWhereSomebodyWillSeeIt is the one round 8 asked for and
// no version of the README carried until it was checked.
//
// It used to assert the README said "Go source only". That stopped being true
// when the second build arrived, and the check now asks the question the claim
// was standing in for: does a reader learn which languages they get, and does
// the program say so at run time. Nobody should discover the scope by pointing
// it at a repository and wondering why the graph came back nearly empty.
func TestTheScopeIsStatedWhereSomebodyWillSeeIt(t *testing.T) {
	readme := strings.ToLower(read(t, "README.md"))
	for _, want := range []string{"reads go and nothing else", "python, solidity"} {
		if !strings.Contains(readme, want) {
			t.Errorf("the README does not say %q, so a reader cannot tell which build reads what", want)
		}
	}

	// And the program says it at run time, not only in a document somebody may
	// never open.
	operations := read(t, "internal/command/operations.go")
	if !strings.Contains(operations, "languages read") {
		t.Error("graph does not report which languages the build reads")
	}

	// Both registries exist, and each says what it holds. A build tag that
	// silently fell away would leave one of them covering both cases.
	for _, path := range []string{"internal/command/analyzers.go", "internal/command/analyzers_cgo.go"} {
		body := read(t, path)
		if !strings.Contains(body, "//go:build") {
			t.Errorf("%s carries no build tag, so both builds would use it", path)
		}
	}
}

// TestEveryShippedDocumentExists guards the list the release checklist names.
func TestEveryShippedDocumentExists(t *testing.T) {
	for _, name := range []string{
		"docs/invariants.md",
		"docs/analyzer-interface.md",
		"docs/thresholds.md",
		"docs/licensing.md",
		"docs/decisions.md",
		"THIRD_PARTY_NOTICES.md",
		"README.md",
	} {
		info, err := os.Stat(filepath.Join(repoRoot(t), name))
		if err != nil {
			t.Errorf("%s is a shipped document and is not there: %v", name, err)
			continue
		}
		if info.Size() < 500 {
			t.Errorf("%s is %d bytes, which is too little to be the document it claims to be",
				name, info.Size())
		}
	}
}

// TestEveryPackageHasADocFile keeps the package documentation where Go's own
// convention puts it.
//
// A doc comment long enough to be worth reading belongs in a file of its own,
// so that it is the first thing found rather than something sitting above
// whichever source file happened to be alphabetically first. It also stops the
// documentation from moving every time that file is split or renamed.
//
// Exactly one file per package may carry it: two package comments is a
// compile-time error in Go, but one in doc.go and a second left behind in a
// source file is the kind of thing that survives a careless move.
func TestEveryPackageHasADocFile(t *testing.T) {
	root := repoRoot(t)
	packages := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if path != root && (name == "vendor" || name == "testdata" || name == "bin" ||
			name == "out" || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		matches, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil {
			return err
		}
		for _, m := range matches {
			if !strings.HasSuffix(m, "_test.go") {
				packages[path] = true
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(packages) == 0 {
		t.Fatal("no packages were found, so this test is watching nothing")
	}

	for dir := range packages {
		rel, _ := filepath.Rel(root, dir)
		docPath := filepath.Join(dir, "doc.go")
		raw, err := os.ReadFile(docPath)
		if err != nil {
			t.Errorf("%s has no doc.go", rel)
			continue
		}
		if !packageComment.Match(raw) {
			t.Errorf("%s/doc.go carries no package comment", rel)
		}

		// The comment lives in one place. A second one left in a source file
		// would not compile, but a source file whose top comment merely looks
		// like one is a real trap, so the check names the file it found.
		others, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, other := range others {
			if other == docPath || strings.HasSuffix(other, "_test.go") {
				continue
			}
			body, err := os.ReadFile(other)
			if err != nil {
				continue
			}
			if packageComment.Match(body) {
				name, _ := filepath.Rel(root, other)
				t.Errorf("%s also opens with a package comment; it belongs in doc.go", name)
			}
		}
	}
}

// packageComment matches a doc comment at the very top of a file, immediately
// above the package clause.
var packageComment = regexp.MustCompile(`\A(// [^\n]*\n)*// (Package|Command) [^\n]*\n(//[^\n]*\n)*package `)

// TestTheLabelSizeMatchesTheStylesheet holds two numbers together that are
// written down twice.
//
// The renderer measures a label to decide where it fits and the browser draws
// it at whatever the stylesheet says. If those two sizes ever part company the
// page still renders, the checker still passes, and the text sits somewhere the
// checker was never asked about — which is the failure the label rule exists to
// prevent, arriving through the one door the rule cannot see.
func TestTheLabelSizeMatchesTheStylesheet(t *testing.T) {
	const name = "edgeLabelSize"
	declared, ok := constantsIn(t, "internal/render/label.go")[name]
	if !ok {
		t.Fatalf("internal/render/label.go no longer declares %s", name)
	}

	css := read(t, "internal/render/viewer/viewer.css")
	pattern := regexp.MustCompile(`\.edge-label\s*\{[^}]*font-size:\s*(\d+)px`)
	found := pattern.FindStringSubmatch(css)
	if found == nil {
		t.Fatal("viewer.css no longer gives .edge-label a font-size in px, so the two cannot be compared")
	}
	if found[1] != declared {
		t.Errorf("the renderer measures labels at %s and the stylesheet draws them at %spx",
			declared, found[1])
	}
}
