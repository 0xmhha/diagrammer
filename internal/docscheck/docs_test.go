package docscheck_test

import (
	"github.com/0xmhha/diagrammer/internal/instruct"
	"github.com/0xmhha/diagrammer/internal/invariant"
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
		"channelMaxLanes": true, "channelSlack": true,
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
		"docs/install.md",
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

// TestTheInstallNotesQuoteTheFloorTheyShipWith holds the packaged document to
// the one number it states about the binaries beside it.
//
// The minimum macOS is decided in one place, the Makefile, because clang takes
// it from whoever did the building otherwise, and a floor that depends on the
// builder is not a floor. `make dist-check` reads the number back out of every
// packed binary, so the binary and the Makefile cannot part company. The
// document is the one that could, and it is the only one of the three a
// recipient actually reads.
func TestTheInstallNotesQuoteTheFloorTheyShipWith(t *testing.T) {
	const name = "MACOS_FLOOR"
	floor, ok := makeVariable(t, name)
	if !ok {
		t.Fatalf("the Makefile no longer sets %s, so nothing decides what the packages declare", name)
	}
	if !strings.Contains(read(t, "docs/install.md"), "macOS "+floor) {
		t.Errorf("docs/install.md never says the minimum is macOS %s, which is what `make dist` packages", floor)
	}
}

// makeVariable reads one simply-expanded variable out of the Makefile.
//
// Read rather than asked for, because asking means running make, and a test
// that runs the build system to learn a constant has made the build system a
// dependency of the test suite. The pattern wants the whole line so that a
// variable mentioned in a recipe is not mistaken for the one that sets it.
func makeVariable(t *testing.T, name string) (string, bool) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*:?=\s*(\S+)\s*$`)
	found := pattern.FindStringSubmatch(read(t, "Makefile"))
	if found == nil {
		return "", false
	}
	return found[1], true
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

// TestTheStylesheetDoesNotShrinkADrawing holds the other end of the size the
// renderer declares.
//
// svgFor writes the width and height the scene was laid out at, so that a
// drawing wider than the page overflows and is scrolled to rather than scaled
// down. A stylesheet rule that caps the drawing puts it back: the browser obeys
// the cap, every label goes under the size the fitting refused to go below, and
// nothing anywhere fails, because the composition rules judge the drawing in
// its own units where the labels are still the size they were fitted to.
//
// It is here rather than in the renderer's own tests because neither half is
// wrong alone. The Go side is checked by TestEveryDrawingDeclaresTheSizeItWasLaidOutAt;
// this is the stylesheet keeping its side of it.
func TestTheStylesheetDoesNotShrinkADrawing(t *testing.T) {
	css := read(t, "internal/render/viewer/viewer.css")

	if !strings.Contains(css, ".scene-scroll") {
		t.Fatal("viewer.css has no .scene-scroll, so a drawing too wide for the page has nothing to overflow into")
	}
	if !regexp.MustCompile(`\.scene-scroll\s*\{[^}]*overflow`).MatchString(css) {
		t.Error(".scene-scroll does not scroll, so a wide drawing is clipped rather than reachable")
	}

	rule := regexp.MustCompile(`svg\.scene\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if rule == nil {
		t.Fatal("viewer.css no longer sizes svg.scene, so there is nothing to check")
	}
	for _, capping := range []string{"max-width", "max-height", "transform"} {
		if strings.Contains(rule[1], capping) {
			t.Errorf("svg.scene sets %s, which overrides the size the renderer declared and shrinks the drawing", capping)
		}
	}

	// And the renderer is still declaring it. A test that only read the
	// stylesheet would keep passing after the Go side stopped writing one.
	if !strings.Contains(read(t, "internal/render/svg.go"), "min-width:") {
		t.Error("svgFor no longer declares the size the scene was laid out at")
	}
}

// The instruction stage 2 is performed from says things about this program, and
// this program can change without it noticing. These hold the two together.

// TestTheInstructionQuotesTheComposersNumbers is the guard on the one part of
// the instruction that is not read from a schema.
//
// It tells a model that a level thinner than so many boxes is unfolded and that
// an unfold stops at so many, because those decide whether the model it writes
// comes out as pages or as one flat drawing. The composer owns both numbers.
// The instruction cannot import the composer, so it restates them, and a
// restatement that drifts is a model told to aim at a target that moved.
func TestTheInstructionQuotesTheComposersNumbers(t *testing.T) {
	composer := constantsIn(t, "internal/compose/component.go")
	told := constantsIn(t, "internal/instruct/instruct.go")

	for _, name := range []string{"minBoxesPerLevel", "expandedMaxBoxes"} {
		want, ok := composer[name]
		if !ok {
			t.Errorf("internal/compose/component.go no longer declares %s", name)
			continue
		}
		got, ok := told[name]
		if !ok {
			t.Errorf("the instruction no longer carries %s, so it cannot be held to the composer", name)
			continue
		}
		if got != want {
			t.Errorf("the instruction tells a model %s is %s; the composer uses %s", name, got, want)
		}
	}
}

// TestTheInstructionNamesEveryFamilyTheGateChecks catches the shape of change
// that would hurt most: a family gains checks in the validator and the model is
// never told about them, so it writes documents the gate refuses for reasons it
// was never given.
func TestTheInstructionNamesEveryFamilyTheGateChecks(t *testing.T) {
	gate := read(t, "internal/validate/validate.go")
	told, err := instruct.Stage2()
	if err != nil {
		t.Fatalf("build the instruction: %v", err)
	}

	// The validator has one check function per family, plus checkFamilies for
	// the declaration itself.
	pattern := regexp.MustCompile(`func check([A-Z][a-zA-Z]*)\(`)
	found := map[string]bool{}
	for _, match := range pattern.FindAllStringSubmatch(gate, -1) {
		found[strings.ToLower(match[1])] = true
	}
	if len(found) == 0 {
		t.Fatal("no check functions were found in the validator, so this test is watching nothing")
	}

	for name := range found {
		// checkUsecaseEnds is a helper of checkUsecase rather than a check of
		// its own, and naming it separately would tell a model nothing.
		if name == "usecaseends" {
			continue
		}
		if !strings.Contains(told, "**"+name+"**") {
			t.Errorf("the validator checks %q and the instruction never names it, so a model is refused "+
				"for a reason it was not given", name)
		}
	}
}

// TestTheWrittenRuleCountMatchesTheCode holds a number the other checks cannot
// see.
//
// Every rule's name is held to the contract already. How many there are is
// written out in words, once, and nothing was watching it: a rule added or
// dropped leaves the sentence counting the old set, and a reader has no way to
// tell that from a rule they have missed. It is the same failure the thresholds
// check exists for, in the one place a threshold is spelled rather than
// digitised.
func TestTheWrittenRuleCountMatchesTheCode(t *testing.T) {
	spelled := map[int]string{
		1: "One", 2: "Two", 3: "Three", 4: "Four", 5: "Five", 6: "Six",
		7: "Seven", 8: "Eight", 9: "Nine", 10: "Ten", 11: "Eleven", 12: "Twelve",
	}
	// Lowercased, because the count opens a sentence in one place and sits
	// inside one in the other, and which it is says nothing about the code.
	doc := strings.ToLower(read(t, "docs/invariants.md"))

	for _, c := range []struct {
		what  string
		rules []string
	}{
		{"the rules a grid family is held to", invariant.CompositionRules()},
		{"the rules a ladder is held to", invariant.SequenceRules()},
	} {
		word, ok := spelled[len(c.rules)]
		if !ok {
			t.Fatalf("%s: %d rules, which this test has no word for", c.what, len(c.rules))
		}
		if !strings.Contains(doc, strings.ToLower(word)+" rules") {
			t.Errorf("the code has %d of %s and docs/invariants.md never says %q",
				len(c.rules), c.what, word+" rules")
		}
	}
}
