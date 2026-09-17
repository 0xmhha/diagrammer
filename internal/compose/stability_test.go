package compose_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/0xmhha/diagrammer/internal/uml"
)

// Stage 2 is performed by a model, and a model asked the same question twice
// does not answer in the same order. That is not a defect anybody can fix in
// the model, so the question worth asking is the one below: when two answers
// say the same thing in a different order, does the same diagram come out?
//
// It very nearly did. Levels, boxes, where each box sits, connections and the
// accounting were all already independent of the order they arrived in. One
// array was not, and a test that had only asked about the drawing would not
// have found it, because stage 4 does not read that array yet.

// shuffleOrderless reorders every array in a model whose order carries no
// meaning, and leaves alone the ones where it does.
//
// A sequence diagram's messages are the exception that makes the rest worth
// stating: the schema says message order is the order of the array, with no
// separate field to disagree with it. Shuffling those does not reorder a
// document, it rewrites one, and the validator says so.
func shuffleOrderless(rng *rand.Rand, model *uml.Model) {
	if c := model.Component; c != nil {
		rng.Shuffle(len(c.Components), func(i, j int) {
			c.Components[i], c.Components[j] = c.Components[j], c.Components[i]
		})
		rng.Shuffle(len(c.Dependencies), func(i, j int) {
			c.Dependencies[i], c.Dependencies[j] = c.Dependencies[j], c.Dependencies[i]
		})
		rng.Shuffle(len(c.Interfaces), func(i, j int) {
			c.Interfaces[i], c.Interfaces[j] = c.Interfaces[j], c.Interfaces[i]
		})
		for k := range c.Components {
			ports := c.Components[k].Ports
			rng.Shuffle(len(ports), func(i, j int) { ports[i], ports[j] = ports[j], ports[i] })
		}
	}
	if u := model.Usecase; u != nil {
		rng.Shuffle(len(u.Actors), func(i, j int) {
			u.Actors[i], u.Actors[j] = u.Actors[j], u.Actors[i]
		})
		rng.Shuffle(len(u.Usecases), func(i, j int) {
			u.Usecases[i], u.Usecases[j] = u.Usecases[j], u.Usecases[i]
		})
	}
	if s := model.State; s != nil {
		rng.Shuffle(len(s.States), func(i, j int) {
			s.States[i], s.States[j] = s.States[j], s.States[i]
		})
		rng.Shuffle(len(s.Transitions), func(i, j int) {
			s.Transitions[i], s.Transitions[j] = s.Transitions[j], s.Transitions[i]
		})
	}
}

func TestTheSameModelInADifferentOrderComposesTheSame(t *testing.T) {
	for _, f := range families() {
		t.Run(string(f.name)+"/"+f.fixture, func(t *testing.T) {
			plain, path := load(t, f.fixture)
			want, err := f.build(path, plain)
			if err != nil {
				t.Fatalf("compose %s: %v", f.name, err)
			}

			// A fresh copy, because the first was consumed by the composer and
			// a composer is allowed to keep what it was handed.
			shuffled, _ := load(t, f.fixture)
			shuffleOrderless(rand.New(rand.NewSource(1)), shuffled)

			got, err := f.build(path, shuffled)
			if err != nil {
				t.Fatalf("compose %s from the shuffled model: %v", f.name, err)
			}

			if asJSON(t, got) != asJSON(t, want) {
				t.Errorf("the same model in a different order composed a different %s document", f.name)
			}
		})
	}
}

// TestShufflingIsDoingSomething is the guard on the test above.
//
// shuffleOrderless walks a model by hand, so a family that grows an array it
// does not know about would leave that array in place, and the test would go on
// passing by shuffling nothing. This fails if the shuffle stopped moving
// anything, which is the one way the check above becomes decoration.
func TestShufflingIsDoingSomething(t *testing.T) {
	moved := 0
	for _, f := range families() {
		plain, _ := load(t, f.fixture)
		shuffled, _ := load(t, f.fixture)
		shuffleOrderless(rand.New(rand.NewSource(1)), shuffled)
		if asJSON(t, plain) != asJSON(t, shuffled) {
			moved++
		}
	}
	if moved == 0 {
		t.Fatal("no fixture was reordered, so the test above is comparing a model with itself")
	}
}

// asJSON is how two documents are compared here: by their bytes, because that
// is what the byte-identity guarantee is about, and a field-by-field comparison
// would pass over exactly the kind of difference this is looking for.
func asJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return string(raw)
}
