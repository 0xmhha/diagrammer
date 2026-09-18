package validate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/graph"
	"github.com/0xmhha/diagrammer/internal/uml"
	"github.com/0xmhha/diagrammer/internal/validate"
)

// A small tree, shaped like a real one: a root, two areas under it, and leaves
// under those. Everything below is about which of these a model stood for.
//
//	root
//	├── pkg:alpha ── file:a.go ── fn:A
//	└── pkg:beta  ── file:b.go ── fn:B
func tree() *graph.Graph {
	node := func(id, parent string, kind graph.NodeKind) graph.Node {
		return graph.Node{ID: id, Name: id, Kind: kind, Parent: parent}
	}
	return &graph.Graph{
		SchemaVersion: 1,
		Root:          "root",
		Nodes: []graph.Node{
			node("root", "", graph.KindGroup),
			node("pkg:alpha", "root", graph.KindPackage),
			node("file:a.go", "pkg:alpha", graph.KindFile),
			node("fn:A", "file:a.go", graph.KindFunc),
			node("pkg:beta", "root", graph.KindPackage),
			node("file:b.go", "pkg:beta", graph.KindFile),
			node("fn:B", "file:b.go", graph.KindFunc),
		},
	}
}

// modelOf returns a model whose components stand for the ids given, one
// component per entry.
func modelOf(claims ...[]string) *uml.Model {
	m := &uml.Model{
		SchemaVersion: 1,
		Meta:          uml.Meta{Title: "t"},
		Provenance:    uml.Provenance{Origin: uml.OriginHandwritten},
		Families:      []uml.Family{uml.FamilyComponent},
		Component:     &uml.ComponentModel{},
	}
	for i, ids := range claims {
		m.Component.Components = append(m.Component.Components, uml.Component{
			ID:          string(rune('a' + i)),
			Name:        string(rune('A' + i)),
			AccountsFor: ids,
		})
	}
	return m
}

func TestCoverage(t *testing.T) {
	cases := []struct {
		name      string
		claims    [][]string
		claimed   int
		unclaimed []string
	}{{
		// Naming a package accounts for what is inside it. This is the whole
		// reason the field is a handful of ids rather than a transcription, so
		// it is the first thing worth holding.
		name:    "naming both areas accounts for everything under them",
		claims:  [][]string{{"pkg:alpha"}, {"pkg:beta"}},
		claimed: 6,
	}, {
		name:      "naming one area leaves the other named as missing",
		claims:    [][]string{{"pkg:alpha"}},
		claimed:   3,
		unclaimed: []string{"pkg:beta"},
	}, {
		// A leaf accounts for itself and nothing above it, so the area it sits
		// in is still short. Reporting the area rather than the leaves is what
		// makes the answer readable on a real tree.
		name:      "naming a leaf accounts for the leaf",
		claims:    [][]string{{"fn:A"}},
		claimed:   1,
		unclaimed: []string{"pkg:alpha", "pkg:beta"},
	}, {
		name:    "one component may stand for several areas",
		claims:  [][]string{{"pkg:alpha", "pkg:beta"}},
		claimed: 6,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := validate.Against(modelOf(c.claims...), tree())
			if err != nil {
				t.Fatalf("Against: %v", err)
			}
			if got.Claimed != c.claimed {
				t.Errorf("accounted for %d nodes, want %d", got.Claimed, c.claimed)
			}
			// The root is not counted: a component claiming it would account
			// for everything by saying nothing.
			if got.Nodes != 6 {
				t.Errorf("counted %d nodes, want 6 with the root left out", got.Nodes)
			}
			if strings.Join(got.Unclaimed, ",") != strings.Join(c.unclaimed, ",") {
				t.Errorf("unaccounted areas are %v, want %v", got.Unclaimed, c.unclaimed)
			}
			if want := len(c.unclaimed) == 0; got.Complete() != want {
				t.Errorf("Complete is %v, want %v", got.Complete(), want)
			}
		})
	}
}

// TestAnIdTheGraphDoesNotHaveIsRefused is the one that makes the rest mean
// something.
//
// Coverage is arithmetic over claims, and arithmetic over claims nobody checked
// would be a number a model could choose for itself. An id is the only thing in
// a model that can be wrong in a way a program can see, so being wrong about one
// has to stop the command rather than lower a percentage.
func TestAnIdTheGraphDoesNotHaveIsRefused(t *testing.T) {
	_, err := validate.Against(modelOf([]string{"pkg:alpha", "pkg:invented"}), tree())
	if err == nil {
		t.Fatal("want a model claiming a node that is not there refused")
	}

	var report *validate.Report
	if !errors.As(err, &report) {
		t.Fatalf("want a *Report, got %T", err)
	}
	if len(report.Problems) != 1 {
		t.Fatalf("want one problem, got %d: %v", len(report.Problems), report.Problems)
	}
	if !strings.Contains(report.Problems[0].Message, "pkg:invented") {
		t.Errorf("the problem does not name the offending id: %s", report.Problems[0].Message)
	}
}

// TestSayingNothingIsNotTheSameAsCoveringNothing keeps the two apart.
//
// A model with no claims has not failed to cover the tree; it has not been
// asked. Every model written before this field existed is one of those, and
// reporting them as zero per cent would be a number that means the opposite of
// what it looks like.
func TestSayingNothingIsNotTheSameAsCoveringNothing(t *testing.T) {
	silent, err := validate.Against(modelOf([]string{}), tree())
	if err != nil {
		t.Fatalf("Against: %v", err)
	}
	if silent.Stated != 0 {
		t.Errorf("Stated is %d, want 0", silent.Stated)
	}
	if silent.Complete() {
		t.Error("a model that said nothing is reported as complete")
	}
	if !strings.Contains(silent.String(), "not stated") {
		t.Errorf("the summary reads as coverage rather than as silence: %s", silent)
	}

	covering, err := validate.Against(modelOf([]string{"pkg:alpha"}), tree())
	if err != nil {
		t.Fatalf("Against: %v", err)
	}
	if strings.Contains(covering.String(), "not stated") {
		t.Errorf("a model that claimed something reads as silence: %s", covering)
	}
}

// TestAModelWithNoComponentFamilyIsNotAFailure covers the shape a sequence-only
// model has. It claims nothing because it has nowhere to claim from, which is
// silence rather than a gap.
func TestAModelWithNoComponentFamilyIsNotAFailure(t *testing.T) {
	m := &uml.Model{
		SchemaVersion: 1,
		Meta:          uml.Meta{Title: "t"},
		Provenance:    uml.Provenance{Origin: uml.OriginHandwritten},
		Families:      []uml.Family{uml.FamilySequence},
	}
	got, err := validate.Against(m, tree())
	if err != nil {
		t.Fatalf("Against: %v", err)
	}
	if got.Stated != 0 || got.Complete() {
		t.Errorf("want silence, got %+v", got)
	}
}
