package webhandler

import (
	"reflect"
	"testing"
)

func TestNextPowerOf2(t *testing.T) {
	cases := []struct{ in, want int }{
		{1, 1}, {2, 2}, {3, 4}, {4, 4}, {5, 8},
		{6, 8}, {8, 8}, {9, 16}, {16, 16}, {17, 32},
	}
	for _, c := range cases {
		if got := nextPowerOf2(c.in); got != c.want {
			t.Errorf("nextPowerOf2(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBracketPositions(t *testing.T) {
	cases := []struct {
		size int
		want []int
	}{
		{2, []int{1, 2}},
		{4, []int{1, 4, 2, 3}},
		{8, []int{1, 8, 4, 5, 2, 7, 3, 6}},
	}
	for _, c := range cases {
		got := bracketPositions(c.size)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("bracketPositions(%d) = %v, want %v", c.size, got, c.want)
		}
	}
}

func TestBracketPositionsPairings(t *testing.T) {
	// Verify that seed 1 always plays seed N, seed 2 plays seed N-1, etc.
	for _, size := range []int{4, 8, 16} {
		pos := bracketPositions(size)
		for i := 0; i < len(pos)-1; i += 2 {
			a, b := pos[i], pos[i+1]
			if a+b != size+1 {
				t.Errorf("size=%d: pair (%d,%d) should sum to %d, got %d", size, a, b, size+1, a+b)
			}
		}
	}
}

func TestRoundLabel(t *testing.T) {
	cases := []struct {
		round, total int
		want         string
	}{
		{1, 1, "Final"},
		{1, 2, "Semifinals"},
		{2, 2, "Final"},
		{1, 3, "Quarterfinals"},
		{2, 3, "Semifinals"},
		{3, 3, "Final"},
		{1, 4, "Round 1"},
		{2, 4, "Quarterfinals"},
		{3, 4, "Semifinals"},
		{4, 4, "Final"},
	}
	for _, c := range cases {
		if got := roundLabel(c.round, c.total); got != c.want {
			t.Errorf("roundLabel(%d,%d) = %q, want %q", c.round, c.total, got, c.want)
		}
	}
}
