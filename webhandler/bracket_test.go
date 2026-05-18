package webhandler

import (
	"reflect"
	"testing"
	"time"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
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

func TestAdvanceBracket_BasicWin(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID

	for _, name := range []string{"Eagles", "Cubs", "Yankees", "Mets"} {
		db.AddTeam(tid, did, name, "")
	}
	teamList := db.ReturnTeamsByDivisionID(did)

	env := &Env{DB: db}

	bid := db.CreateBracket(did, "single_elimination", 4)
	db.SetBracketStatus(bid, "active")

	// Round 1, position 1: team0 vs team1
	bg1 := db.AddBracketGame(bid, 1, 1)
	db.SetBracketGameTeams(bg1, teamList[0].ID, teamList[1].ID, false, false)
	// Round 1, position 2: team2 vs team3
	db.AddBracketGame(bid, 1, 2)
	// Round 2 (final), position 1: TBD
	finalID := db.AddBracketGame(bid, 2, 1)

	gid := db.AddGame(tid, did, teamList[0].ID, teamList[1].ID, "", time.Time{}, "")
	db.SetBracketGameGameID(bg1, gid)
	db.ScoreGame(gid, 5, 2) // teamList[0] wins (home)

	env.AdvanceBracket(gid, teamList[0].ID)

	// bg1 should have winner set
	bg1state := db.GetBracketGameByID(bg1)
	if bg1state.WinnerTeamID != teamList[0].ID {
		t.Errorf("bg1 winner: got %d, want %d", bg1state.WinnerTeamID, teamList[0].ID)
	}

	// Final's top slot should be filled (position 1 is odd → top)
	finalState := db.GetBracketGameByID(finalID)
	if finalState.TopTeamID != teamList[0].ID {
		t.Errorf("final top team: got %d, want %d", finalState.TopTeamID, teamList[0].ID)
	}
}

func TestAdvanceBracket_FinalCompletes(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	teamList := db.ReturnTeamsByDivisionID(did)
	env := &Env{DB: db}

	// 2-team bracket: single final game
	bid := db.CreateBracket(did, "single_elimination", 2)
	db.SetBracketStatus(bid, "active")
	finalBGID := db.AddBracketGame(bid, 1, 1)
	db.SetBracketGameTeams(finalBGID, teamList[0].ID, teamList[1].ID, false, false)
	gid := db.AddGame(tid, did, teamList[0].ID, teamList[1].ID, "", time.Time{}, "")
	db.SetBracketGameGameID(finalBGID, gid)
	db.ScoreGame(gid, 3, 1)

	env.AdvanceBracket(gid, teamList[0].ID)

	if db.GetBracketByID(bid).Status != "complete" {
		t.Fatal("bracket should be complete after final is scored")
	}
}
