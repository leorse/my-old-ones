package domain

import (
	"reflect"
	"testing"
)

func TestSosa(t *testing.T) {
	tree := map[string]Parents{
		"me":  {"dad", "mum"},
		"dad": {"pgf", "pgm"},
		"mum": {"mgf", ""},
		"pgf": {"x", "y"}, // x : ancêtre commun (implexe)
		"mgf": {"x", ""},
	}
	got := SosaNumbers("me", func(id string) Parents { return tree[id] }, 0)
	want := map[string][]int64{
		"me": {1}, "dad": {2}, "mum": {3}, "pgf": {4}, "pgm": {5}, "mgf": {6},
		"x": {8, 12}, "y": {9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestSosaCycleBounded(t *testing.T) {
	// données corrompues : a est son propre père
	got := SosaNumbers("a", func(id string) Parents { return Parents{Father: "a"} }, 5)
	if len(got["a"]) != 5 {
		t.Fatalf("génération bornée: %v", got["a"])
	}
}

func TestGeneration(t *testing.T) {
	for n, g := range map[int64]int{1: 1, 2: 2, 3: 2, 4: 3, 7: 3, 8: 4} {
		if Generation(n) != g {
			t.Errorf("Generation(%d) = %d", n, Generation(n))
		}
	}
}
