package render

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/vcs"
)

const testCommit = "4f1d9c8b2a7e6053fc1b8d94a2e70f6c5b3a1d82"

// The page is where the commit is finally for something. Everything upstream of
// here carries it between files a person does not open; this is the one place
// somebody looking at a drawing can read which code it describes.

func TestThePageNamesTheCommitItWasDrawnFrom(t *testing.T) {
	page := &Page{
		Title:    "A drawing",
		Revision: &vcs.Revision{Commit: testCommit, Ref: "refs/heads/main"},
	}
	html := page.HTML()

	// The whole object name, not an abbreviation. The schemas refuse an
	// abbreviated commit, and a page showing one would be offering a reader the
	// exact value the contract will not take back.
	if !strings.Contains(html, ">"+testCommit+"<") {
		t.Errorf("the page does not show the whole commit:\n%s", header(t, html))
	}
	if !strings.Contains(html, attrRevision+`="`+testCommit+`"`) {
		t.Errorf("the page does not carry %s, so nothing can read the commit back out of it", attrRevision)
	}
	if !strings.Contains(html, "refs/heads/main") {
		t.Error("the page does not say which branch the commit was on")
	}
}

func TestADetachedHeadIsDrawnWithoutABranch(t *testing.T) {
	page := &Page{Title: "A drawing", Revision: &vcs.Revision{Commit: testCommit}}
	html := page.HTML()

	if !strings.Contains(html, testCommit) {
		t.Fatal("the commit is missing")
	}
	// "on " with nothing after it is what a naive template does here, and it
	// reads as a page that lost something rather than one that never had it.
	if strings.Contains(html, "</code> on </p>") || strings.Contains(html, "on \n") {
		t.Errorf("the page offers a branch it does not have:\n%s", header(t, html))
	}
}

// TestAPageWithNoRevisionSaysNothing is the one that keeps the feature honest.
// Most documents will carry no commit, because most trees are not read out of a
// checkout, and an empty label where a commit should be is a page that looks
// broken rather than one that is silent.
func TestAPageWithNoRevisionSaysNothing(t *testing.T) {
	html := (&Page{Title: "A drawing"}).HTML()
	if strings.Contains(html, `class="revision"`) {
		t.Errorf("a page with no revision drew the line anyway:\n%s", header(t, html))
	}
}

// TestTheCommitIsEscaped guards the one place a value read out of a repository
// reaches an HTML page.
//
// The schema pattern would refuse this commit, but render does not revalidate
// what compose handed it, and a page is a thing people open in a browser.
func TestTheCommitIsEscaped(t *testing.T) {
	page := &Page{Title: "A drawing", Revision: &vcs.Revision{
		Commit: `"><script>alert(1)</script>`,
		Ref:    `refs/heads/"><script>alert(2)</script>`,
	}}
	// Scoped to the header, because the page embeds the viewer and a page that
	// holds no script at all is a page with no viewer in it.
	head := header(t, page.HTML())

	if strings.Contains(head, "<script>") || strings.Contains(head, `"><`) {
		t.Errorf("a commit reached the page unescaped:\n%s", head)
	}
	if !strings.Contains(head, "&lt;script&gt;") {
		t.Errorf("the commit does not appear escaped either, so it went somewhere else:\n%s", head)
	}
}

// TestBuildCarriesTheRevisionFromTheDocument holds the two halves together: the
// document states it, Build has to put it on the Page, and only then does the
// HTML above have anything to write.
func TestBuildCarriesTheRevisionFromTheDocument(t *testing.T) {
	doc := &diagram.Document{
		SchemaVersion: 1,
		Family:        diagram.FamilyComponent,
		Meta:          diagram.Meta{Title: "A drawing"},
		Provenance: diagram.Provenance{
			Model:    "model.json",
			Revision: &vcs.Revision{Commit: testCommit, Ref: "refs/heads/main"},
		},
		Levels: []diagram.Level{{
			ID:    "root",
			Title: "Root",
			Grid:  diagram.Grid{Rows: 1, Cols: 1},
			Boxes: []diagram.Box{{ID: "a", Label: "A", Row: 0, Col: 0}},
		}},
	}

	page, err := Build(doc)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if page.Revision == nil || page.Revision.Commit != testCommit {
		t.Fatalf("the document stated %s and the page carries %+v", testCommit, page.Revision)
	}
}

// header returns the part of a page worth reading in a failure message.
func header(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, "<header>")
	end := strings.Index(html, "</header>")
	if start < 0 || end < 0 {
		return html
	}
	return html[start : end+len("</header>")]
}
