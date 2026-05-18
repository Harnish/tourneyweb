package webhandler

import (
	"testing"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func makeTeams(n int) []mydb.Team {
	teams := make([]mydb.Team, n)
	for i := range teams {
		teams[i] = mydb.Team{ID: i + 1}
	}
	return teams
}

// countingGames returns per-team counts of games that count (not scrimmage for that team).
func countingGames(specs []gameSpec) map[int]int {
	counts := make(map[int]int)
	for _, s := range specs {
		if s.scrimmageFor != s.homeTeamID {
			counts[s.homeTeamID]++
		}
		if s.scrimmageFor != s.awayTeamID {
			counts[s.awayTeamID]++
		}
	}
	return counts
}

func TestCappedSchedule_EvenDistribution(t *testing.T) {
	// 4 teams, cap 2: 4*2=8 (even) — everyone gets exactly 2 counting games
	teams := makeTeams(4)
	specs := cappedSchedule(teams, 2)
	counts := countingGames(specs)
	for _, tm := range teams {
		if counts[tm.ID] != 2 {
			t.Errorf("team %d: got %d counting games, want 2", tm.ID, counts[tm.ID])
		}
	}
}

func TestCappedSchedule_NoDuplicatePairs(t *testing.T) {
	// No team should play the same opponent twice
	teams := makeTeams(6)
	specs := cappedSchedule(teams, 3)
	seen := make(map[[2]int]bool)
	for _, s := range specs {
		a, b := s.homeTeamID, s.awayTeamID
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		if seen[key] {
			t.Errorf("duplicate pair (%d, %d)", s.homeTeamID, s.awayTeamID)
		}
		seen[key] = true
	}
}

func TestCappedSchedule_OddCase_AllTeamsGetCapCountingGames(t *testing.T) {
	// 5 teams, cap 3: 5*3=15 (odd) — one team gets an extra scrimmage game,
	// but all teams still get exactly 3 counting games.
	teams := makeTeams(5)
	specs := cappedSchedule(teams, 3)
	counts := countingGames(specs)
	for _, tm := range teams {
		if counts[tm.ID] != 3 {
			t.Errorf("team %d: got %d counting games, want 3", tm.ID, counts[tm.ID])
		}
	}
	// Exactly one scrimmage game
	var scrimmageGames int
	for _, s := range specs {
		if s.scrimmageFor != 0 {
			scrimmageGames++
		}
	}
	if scrimmageGames != 1 {
		t.Errorf("want 1 scrimmage game, got %d", scrimmageGames)
	}
}

func TestCappedSchedule_CapHigherThanPossible_UsesAllPairs(t *testing.T) {
	// 3 teams, cap 10: max possible is 2 games per team (n-1=2)
	teams := makeTeams(3)
	specs := cappedSchedule(teams, 10)
	if len(specs) != 3 { // 3*(3-1)/2 = 3 unique pairs
		t.Errorf("want 3 games (full round-robin), got %d", len(specs))
	}
}
