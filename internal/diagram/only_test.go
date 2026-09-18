package diagram_test

import (
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/diagram"
)

func tree() *diagram.Document {
	return &diagram.Document{
		SchemaVersion: 1,
		Family:        diagram.FamilyComponent,
		Meta:          diagram.Meta{Title: "t"},
		Levels: []diagram.Level{
			{ID: "overview", Title: "Overview",
				Boxes:      []diagram.Box{{ID: "a", Label: "A", Opens: "level:a"}, {ID: "b", Label: "B"}},
				Accounting: diagram.Accounting{Proven: 3, Drawn: 2, Dropped: 1}},
			{ID: "level:a", Title: "A", Parent: "overview", OpensFrom: "a",
				Boxes:      []diagram.Box{{ID: "a1", Label: "A1"}},
				Accounting: diagram.Accounting{Proven: 1, Drawn: 1}},
		},
		Accounting: diagram.Accounting{Proven: 4, Drawn: 3, Dropped: 1},
	}
}

func TestOnlyKeepsOneLevelAndNothingThatPointsOutOfIt(t *testing.T) {
	doc := tree()
	got, err := doc.Only("overview")
	if err != nil {
		t.Fatalf("Only: %v", err)
	}
	if len(got.Levels) != 1 || got.Levels[0].ID != "overview" {
		t.Fatalf("kept %d level(s), want overview alone", len(got.Levels))
	}
	// The box that opened level:a opens nothing now, because there is no
	// level:a on this page to open.
	if got.Levels[0].Boxes[0].Opens != "" {
		t.Errorf("box a still opens %q, which is not on the page", got.Levels[0].Boxes[0].Opens)
	}
	// The level's own accounting is what it was; the document's is now that.
	if got.Accounting != (diagram.Accounting{Proven: 3, Drawn: 2, Dropped: 1}) {
		t.Errorf("accounting is %+v", got.Accounting)
	}
	// And the original is not touched: Only is a copy.
	if doc.Levels[0].Boxes[0].Opens != "level:a" || len(doc.Levels) != 2 {
		t.Error("Only changed the document it was given")
	}
}

func TestOnlyADeepLevelStandsOnItsOwn(t *testing.T) {
	got, err := tree().Only("level:a")
	if err != nil {
		t.Fatalf("Only: %v", err)
	}
	l := got.Levels[0]
	if l.Parent != "" || l.OpensFrom != "" {
		t.Errorf("a level alone still says it sits beneath %q, opened from %q", l.Parent, l.OpensFrom)
	}
}

func TestOnlyNamesTheLevelsWhenAskedForOneThatIsNotThere(t *testing.T) {
	_, err := tree().Only("level:z")
	if err == nil {
		t.Fatal("want a refusal")
	}
	for _, want := range []string{`"level:z"`, "overview", "level:a"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %s: %v", want, err)
		}
	}
}
