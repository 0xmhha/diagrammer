package validate

import (
	"fmt"
	"sort"

	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// Coverage is how much of a code graph a model claimed to stand for.
//
// It exists because a model is written by something that is not a program, and
// everything else it says is taken on trust. A component's name is whatever it
// chose to call something, its description is prose, and its nesting is a
// judgement. None of that can be held to the tree it came from.
//
// A graph node id can. It is in the graph or it is not, and what sits beneath it
// is a fact the graph already recorded. So a model that says which nodes each
// component stands for is the only kind that can be asked whether it looked at
// the whole tree, and this is that question answered by arithmetic rather than
// by reading the model and being impressed.
type Coverage struct {
	// Claimed is how many graph nodes the model accounted for, counting a node
	// as accounted for when it or any ancestor was named.
	Claimed int
	// Nodes is how many the graph holds.
	Nodes int
	// Unclaimed names the graph's top-level areas that no component stood for,
	// sorted. It is the useful half: a count says a third is missing and these
	// say which third.
	Unclaimed []string
	// Stated is how many ids the model named. A model that named none has no
	// coverage rather than no gaps, and the two must not read the same.
	Stated int
}

// Complete reports whether every area of the graph was accounted for.
func (c Coverage) Complete() bool { return c.Stated > 0 && len(c.Unclaimed) == 0 }

// String is the one-line summary a command prints.
func (c Coverage) String() string {
	if c.Stated == 0 {
		return "coverage: not stated; no component says which graph nodes it stands for"
	}
	pct := 0.0
	if c.Nodes > 0 {
		pct = 100 * float64(c.Claimed) / float64(c.Nodes)
	}
	if len(c.Unclaimed) == 0 {
		return fmt.Sprintf("coverage: %d of %d nodes (%.0f%%), every area accounted for",
			c.Claimed, c.Nodes, pct)
	}
	return fmt.Sprintf("coverage: %d of %d nodes (%.0f%%), %d area(s) no component stands for",
		c.Claimed, c.Nodes, pct, len(c.Unclaimed))
}

// Against checks a model's claims against the graph it says it came from.
//
// An id the graph does not have is a defect and comes back in the report: it is
// a claim about something that is not there, and a claim nobody can check is
// what this field exists to avoid. Leaving areas unaccounted for is not a
// defect. A model is a map rather than a census and is allowed to leave things
// out; what it is not allowed to do is leave them out silently, which is why
// they are counted and named instead.
func Against(m *uml.Model, g *graph.Graph) (Coverage, error) {
	parent := make(map[string]string, len(g.Nodes))
	for _, n := range g.Nodes {
		parent[n.ID] = n.Parent
	}

	claimed := map[string]bool{}
	report := &Report{Source: "accountsFor"}
	if m.Component != nil {
		for _, c := range m.Component.Components {
			for _, id := range c.AccountsFor {
				if _, ok := parent[id]; !ok {
					report.add("component "+c.ID,
						"stands for graph node %q, which the graph does not have", id)
					continue
				}
				claimed[id] = true
			}
		}
	}
	if len(report.Problems) > 0 {
		return Coverage{}, report
	}

	// The root is left out of the count. It is the analysed tree itself, so a
	// component claiming it would account for everything by saying nothing, and
	// counting it against a model that did not claim it puts a percentage
	// nobody can reach on every report.
	cov := Coverage{Nodes: len(g.Nodes) - 1, Stated: len(claimed)}
	unclaimedAreas := map[string]bool{}
	for _, n := range g.Nodes {
		if n.ID == g.Root {
			continue
		}
		if accountedFor(n.ID, parent, claimed) {
			cov.Claimed++
			continue
		}
		if area := areaOf(n.ID, parent, g.Root); area != "" {
			unclaimedAreas[area] = true
		}
	}
	for area := range unclaimedAreas {
		cov.Unclaimed = append(cov.Unclaimed, area)
	}
	sort.Strings(cov.Unclaimed)
	return cov, nil
}

// accountedFor reports whether id or any ancestor of it was named.
//
// Naming a package accounts for what is inside it. That is what keeps this
// field a handful of ids rather than a transcription of the tree, and it is
// sound because the nesting is the graph's own rather than the model's.
func accountedFor(id string, parent map[string]string, claimed map[string]bool) bool {
	for seen := 0; id != "" && seen <= len(parent); seen++ {
		if claimed[id] {
			return true
		}
		id = parent[id]
	}
	return false
}

// areaOf returns the ancestor of id that sits directly under the root, which is
// the granularity worth naming to somebody.
//
// A list of four thousand unaccounted functions is not a report anybody reads.
// "p2p and les are missing" is.
func areaOf(id string, parent map[string]string, root string) string {
	for seen := 0; id != "" && seen <= len(parent); seen++ {
		up := parent[id]
		if up == root || up == "" {
			if id == root {
				return ""
			}
			return id
		}
		id = up
	}
	return ""
}
