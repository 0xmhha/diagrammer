package render

import (
	_ "embed"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The renderer-to-viewer contract is the one thing here that no other check
// catches.
//
// Leave an attribute out and the page still draws, perfectly and in silence,
// while every interaction dies. The composition rules would not notice: they
// judge the drawing, and this is about the vocabulary underneath it. So the
// contract is extracted from the two sides rather than written down a third
// time, and the test fails on any disagreement.

var attributePattern = regexp.MustCompile(`data-[a-z0-9-]+`)

// TestViewerContractMatchesTheViewer holds the declared viewer contract and the
// viewer to each other, in both directions. An attribute the viewer reads and
// nobody promised is a dependency nobody knows about; one that is promised and
// never read is a clause describing nothing.
func TestViewerContractMatchesTheViewer(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range ViewerContract() {
		declared[name] = true
	}
	read := map[string]bool{}
	for _, name := range attributesIn(viewerJS) {
		read[name] = true
		if !declared[name] {
			t.Errorf("the viewer reads %q, which is not in ViewerContract", name)
		}
	}
	for _, name := range ViewerContract() {
		if !read[name] {
			t.Errorf("%q is in ViewerContract but the viewer never reads it", name)
		}
	}
}

// TestRendererWritesTheContract is the half that matters most. The viewer is
// the reader; if the renderer stops writing one of these, the page draws and
// the interaction dies.
func TestRendererWritesTheContract(t *testing.T) {
	// Every family is drawn: an attribute only appears when the thing it
	// describes does, and a ladder has furniture a grid does not.
	written := map[string]bool{}
	for _, attr := range attributesAcrossEveryFamily(t) {
		written[attr] = true
	}
	for _, name := range DOMContract() {
		if !written[name] {
			t.Errorf("the renderer never writes %q, so whatever the viewer does with it cannot work", name)
		}
	}
}

// TestRendererWritesNothingUndeclared catches the opposite: an attribute the
// renderer emits that no contract covers is one nobody is holding to anything.
func TestRendererWritesNothingUndeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range DOMContract() {
		declared[name] = true
	}
	for _, name := range attributesAcrossEveryFamily(t) {
		if !declared[name] {
			t.Errorf("the renderer writes %q, which is in no contract", name)
		}
	}
}

// attributesAcrossEveryFamily collects what the renderer writes when it has
// drawn one of everything.
func attributesAcrossEveryFamily(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, f := range families() {
		page, err := Build(composeFor(t, f.Family, f.Fixture))
		if err != nil {
			t.Fatalf("render %s: %v", f.Family, err)
		}
		for _, attr := range attributesIn(page.HTML()) {
			seen[attr] = true
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func attributesIn(source string) []string {
	seen := map[string]bool{}
	for _, match := range attributePattern.FindAllString(source, -1) {
		seen[match] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TestViewerIsSelfContained keeps the page openable from a file.
//
// An artifact that needed the network would be useless exactly where diagrams
// are most often looked at: on a laptop, offline, from a directory someone was
// handed.
func TestViewerIsSelfContained(t *testing.T) {
	html := buildFixture(t, "nested-platform").HTML()
	// The SVG namespace is a URL and is not a request: it names the dialect the
	// markup is written in and nothing fetches it. It is excluded by name
	// rather than by loosening the check.
	stripped := strings.ReplaceAll(html, `xmlns="http://www.w3.org/2000/svg"`, "")
	for _, forbidden := range []string{"http://", "https://", "<script src", "<link rel=\"stylesheet\""} {
		if strings.Contains(stripped, forbidden) {
			t.Errorf("the page contains %q, so it is not self-contained", forbidden)
		}
	}
	if !strings.Contains(html, "<style>") || !strings.Contains(html, "<script>") {
		t.Error("the page does not carry its own style and behaviour")
	}
}
