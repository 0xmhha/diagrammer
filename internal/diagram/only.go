package diagram

import (
	"fmt"
	"sort"
	"strings"
)

// Only returns a copy of doc holding one level.
//
// It is what `-level` means on the commands that draw: the overview alone, or
// one page from deep in the tree, as a document in its own right. A box that
// opened a level no longer present has nothing to open, so that is cleared
// rather than left pointing at a page the reader will never find, and the
// level's own place in the tree is cleared for the same reason. Its accounting
// is untouched: what it proved and drew does not change because it is alone.
func (d *Document) Only(id string) (*Document, error) {
	var kept *Level
	for i := range d.Levels {
		if d.Levels[i].ID == id {
			kept = &d.Levels[i]
			break
		}
	}
	if kept == nil {
		ids := make([]string, 0, len(d.Levels))
		for _, l := range d.Levels {
			ids = append(ids, l.ID)
		}
		sort.Strings(ids)
		return nil, fmt.Errorf("no level %q; the document has %s", id, strings.Join(ids, ", "))
	}

	level := *kept
	level.Parent = ""
	level.OpensFrom = ""
	level.Boxes = append([]Box(nil), kept.Boxes...)
	for i := range level.Boxes {
		level.Boxes[i].Opens = ""
	}

	out := *d
	out.Levels = []Level{level}
	out.Accounting = level.Accounting
	return &out, nil
}
