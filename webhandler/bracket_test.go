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
	bg1 := db.AddBracketGame(bid, 1, 1, "winners")
	db.SetBracketGameTeams(bg1, teamList[0].ID, teamList[1].ID, false, false)
	// Round 1, position 2: team2 vs team3
	db.AddBracketGame(bid, 1, 2, "winners")
	// Round 2 (final), position 1: TBD
	finalID := db.AddBracketGame(bid, 2, 1, "winners")

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
	finalBGID := db.AddBracketGame(bid, 1, 1, "winners")
	db.SetBracketGameTeams(finalBGID, teamList[0].ID, teamList[1].ID, false, false)
	gid := db.AddGame(tid, did, teamList[0].ID, teamList[1].ID, "", time.Time{}, "")
	db.SetBracketGameGameID(finalBGID, gid)
	db.ScoreGame(gid, 3, 1)

	env.AdvanceBracket(gid, teamList[0].ID)

	if db.GetBracketByID(bid).Status != "complete" {
		t.Fatal("bracket should be complete after final is scored")
	}
}

func setupDoubleElimTeams(t *testing.T, db *mydb.FakeDB, names ...string) (tid, did int, teams []mydb.Team) {
	t.Helper()
	tid = db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did = db.ReturnDivisions(tid)[0].ID
	for _, name := range names {
		db.AddTeam(tid, did, name, "")
	}
	teams = db.ReturnTeamsByDivisionID(did)
	return
}

func TestGenerateDoubleEliminationBracket_4Team(t *testing.T) {
	db := mydb.NewFakeDB()
	_, did, teams := setupDoubleElimTeams(t, db, "A", "B", "C", "D")

	bid := db.CreateBracket(did, "double_elimination", 4)
	for i, team := range teams {
		db.AddBracketSeed(bid, i+1, team.ID)
	}

	generateBracket(db, bid)

	games := db.GetBracketGames(bid)
	var winners, losers, final []mydb.BracketGame
	for _, bg := range games {
		switch bg.Side {
		case "winners":
			winners = append(winners, bg)
		case "losers":
			losers = append(losers, bg)
		case "final":
			final = append(final, bg)
		}
	}
	if len(winners) != 3 {
		t.Fatalf("want 3 winners-bracket games (2 round1 + 1 final), got %d", len(winners))
	}
	if len(losers) != 2 {
		t.Fatalf("want 2 losers-bracket games, got %d", len(losers))
	}
	if len(final) != 1 {
		t.Fatalf("want 1 grand-final game at generation time, got %d", len(final))
	}

	wb1p1 := db.GetBracketGameByRoundPosition(bid, "winners", 1, 1)
	if wb1p1.TopTeamID != teams[0].ID || wb1p1.BottomTeamID != teams[3].ID {
		t.Fatalf("WB1 pos1: want seed1 vs seed4, got top=%d bottom=%d", wb1p1.TopTeamID, wb1p1.BottomTeamID)
	}
	lb1 := db.GetBracketGameByRoundPosition(bid, "losers", 1, 1)
	if lb1.TopTeamID != 0 || lb1.BottomTeamID != 0 {
		t.Fatalf("losers round1 should start empty (TBD), got top=%d bottom=%d", lb1.TopTeamID, lb1.BottomTeamID)
	}
}

func TestAdvanceBracket_DoubleElimination_4Team_WBChampWinsOutright(t *testing.T) {
	db := mydb.NewFakeDB()
	tid, did, teams := setupDoubleElimTeams(t, db, "A", "B", "C", "D")
	env := &Env{DB: db}
	a, b, c, d := teams[0], teams[1], teams[2], teams[3]

	bid := db.CreateBracket(did, "double_elimination", 4)
	for i, team := range teams {
		db.AddBracketSeed(bid, i+1, team.ID)
	}
	generateBracket(db, bid)
	db.SetBracketStatus(bid, "active")

	play := func(side string, round, pos int, home, away mydb.Team, homeWins bool) {
		bg := db.GetBracketGameByRoundPosition(bid, side, round, pos)
		gid := db.AddGame(tid, did, home.ID, away.ID, "", time.Time{}, "")
		db.SetBracketGameGameID(bg.ID, gid)
		if homeWins {
			db.ScoreGame(gid, 5, 2)
			env.AdvanceBracket(gid, home.ID)
		} else {
			db.ScoreGame(gid, 2, 5)
			env.AdvanceBracket(gid, away.ID)
		}
	}

	// WB1: A beats D, B beats C.
	play("winners", 1, 1, a, d, true)
	play("winners", 1, 2, b, c, true)

	// LB round1: D (loser of A) vs C (loser of B). D wins.
	lb1 := db.GetBracketGameByRoundPosition(bid, "losers", 1, 1)
	if lb1.TopTeamID != d.ID || lb1.BottomTeamID != c.ID {
		t.Fatalf("LB1: want D vs C, got top=%d bottom=%d", lb1.TopTeamID, lb1.BottomTeamID)
	}
	play("losers", 1, 1, d, c, true)

	// WB final: A vs B. B wins (A drops to losers bracket).
	play("winners", 2, 1, a, b, false)

	// LB round2 (LB final): D (LB1 winner) vs A (WB final loser).
	lb2 := db.GetBracketGameByRoundPosition(bid, "losers", 2, 1)
	if lb2.TopTeamID != d.ID || lb2.BottomTeamID != a.ID {
		t.Fatalf("LB2: want D vs A, got top=%d bottom=%d", lb2.TopTeamID, lb2.BottomTeamID)
	}
	play("losers", 2, 1, d, a, true)

	// Grand final: WB champ B vs LB champ D.
	final := db.GetBracketGameByRoundPosition(bid, "final", 1, 1)
	if final.TopTeamID != b.ID || final.BottomTeamID != d.ID {
		t.Fatalf("grand final: want B vs D, got top=%d bottom=%d", final.TopTeamID, final.BottomTeamID)
	}
	play("final", 1, 1, b, d, true) // B (WB champ) wins outright.

	if db.GetBracketByID(bid).Status != "complete" {
		t.Fatal("bracket should be complete after WB champ wins the grand final")
	}
	reset := db.GetBracketGameByRoundPosition(bid, "final", 2, 1)
	if reset.ID != 0 {
		t.Fatal("no bracket-reset game should be created when the WB champ wins outright")
	}
}

func TestAdvanceBracket_DoubleElimination_4Team_BracketReset(t *testing.T) {
	db := mydb.NewFakeDB()
	tid, did, teams := setupDoubleElimTeams(t, db, "A", "B", "C", "D")
	env := &Env{DB: db}
	a, b, c, d := teams[0], teams[1], teams[2], teams[3]

	bid := db.CreateBracket(did, "double_elimination", 4)
	for i, team := range teams {
		db.AddBracketSeed(bid, i+1, team.ID)
	}
	generateBracket(db, bid)
	db.SetBracketStatus(bid, "active")

	play := func(side string, round, pos int, home, away mydb.Team, homeWins bool) {
		bg := db.GetBracketGameByRoundPosition(bid, side, round, pos)
		gid := db.AddGame(tid, did, home.ID, away.ID, "", time.Time{}, "")
		db.SetBracketGameGameID(bg.ID, gid)
		if homeWins {
			db.ScoreGame(gid, 5, 2)
			env.AdvanceBracket(gid, home.ID)
		} else {
			db.ScoreGame(gid, 2, 5)
			env.AdvanceBracket(gid, away.ID)
		}
	}

	play("winners", 1, 1, a, d, true)  // A beats D
	play("winners", 1, 2, b, c, true)  // B beats C
	play("losers", 1, 1, d, c, true)   // D beats C
	play("winners", 2, 1, a, b, false) // B beats A (WB final)
	play("losers", 2, 1, d, a, true)   // D beats A (LB final)

	final := db.GetBracketGameByRoundPosition(bid, "final", 1, 1)
	play("final", 1, 1, b, d, false) // D (LB champ) upsets B: bracket reset required.

	if db.GetBracketByID(bid).Status == "complete" {
		t.Fatal("bracket should not be complete yet: LB champ beating an undefeated WB champ forces a reset game")
	}
	reset := db.GetBracketGameByRoundPosition(bid, "final", 2, 1)
	if reset.ID == 0 {
		t.Fatal("bracket-reset game should have been created")
	}
	if reset.TopTeamID != final.TopTeamID || reset.BottomTeamID != final.BottomTeamID {
		t.Fatalf("reset game should replay the same two teams: got top=%d bottom=%d", reset.TopTeamID, reset.BottomTeamID)
	}

	// Reset game: whoever wins is champion regardless of side.
	play("final", 2, 1, b, d, true)
	if db.GetBracketByID(bid).Status != "complete" {
		t.Fatal("bracket should be complete after the reset game is decided")
	}
}

func TestAdvanceBracket_DoubleElimination_8Team_LosersConsolidationMerge(t *testing.T) {
	db := mydb.NewFakeDB()
	tid, did, teams := setupDoubleElimTeams(t, db, "S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8")
	env := &Env{DB: db}

	bid := db.CreateBracket(did, "double_elimination", 8)
	for i, team := range teams {
		db.AddBracketSeed(bid, i+1, team.ID)
	}
	generateBracket(db, bid)
	db.SetBracketStatus(bid, "active")

	play := func(side string, round, pos int, home, away mydb.Team) {
		bg := db.GetBracketGameByRoundPosition(bid, side, round, pos)
		gid := db.AddGame(tid, did, home.ID, away.ID, "", time.Time{}, "")
		db.SetBracketGameGameID(bg.ID, gid)
		db.ScoreGame(gid, 5, 2) // home always wins
		env.AdvanceBracket(gid, home.ID)
	}

	// bracketPositions(8) = [1,8,4,5,2,7,3,6] -> WB1: (s1,s8) (s4,s5) (s2,s7) (s3,s6).
	s := teams // s[0]=seed1 ... s[7]=seed8
	play("winners", 1, 1, s[0], s[7])  // seed1 beats seed8
	play("winners", 1, 2, s[3], s[4])  // seed4 beats seed5
	play("winners", 1, 3, s[1], s[6])  // seed2 beats seed7
	play("winners", 1, 4, s[2], s[5])  // seed3 beats seed6

	// LB1 pos1: loser(WB1p1)=seed8 vs loser(WB1p2)=seed5. LB1 pos2: seed7 vs seed6.
	play("losers", 1, 1, s[7], s[4]) // seed8 beats seed5
	play("losers", 1, 2, s[6], s[5]) // seed7 beats seed6

	// WB2 pos1: winner(WB1p1)=seed1 vs winner(WB1p2)=seed4. WB2 pos2: seed2 vs seed3.
	play("winners", 2, 1, s[0], s[3]) // seed1 beats seed4
	play("winners", 2, 2, s[1], s[2]) // seed2 beats seed3

	// LB2 pos1: top=seed8 (LB1p1 winner), bottom=seed4 (loser WB2p1).
	lb2p1 := db.GetBracketGameByRoundPosition(bid, "losers", 2, 1)
	if lb2p1.TopTeamID != s[7].ID || lb2p1.BottomTeamID != s[3].ID {
		t.Fatalf("LB2 pos1: want seed8 vs seed4, got top=%d bottom=%d", lb2p1.TopTeamID, lb2p1.BottomTeamID)
	}
	// LB2 pos2: top=seed7 (LB1p2 winner), bottom=seed3 (loser WB2p2).
	lb2p2 := db.GetBracketGameByRoundPosition(bid, "losers", 2, 2)
	if lb2p2.TopTeamID != s[6].ID || lb2p2.BottomTeamID != s[2].ID {
		t.Fatalf("LB2 pos2: want seed7 vs seed3, got top=%d bottom=%d", lb2p2.TopTeamID, lb2p2.BottomTeamID)
	}

	play("losers", 2, 1, s[7], s[3]) // seed8 beats seed4
	play("losers", 2, 2, s[6], s[2]) // seed7 beats seed3

	// LB2 is an even (cross) round, not the last LB round: its winners must merge
	// into LB round3 position 1 (pos1->top, pos2->bottom), not pass straight through.
	lb3 := db.GetBracketGameByRoundPosition(bid, "losers", 3, 1)
	if lb3.TopTeamID != s[7].ID || lb3.BottomTeamID != s[6].ID {
		t.Fatalf("LB3 pos1: want seed8 vs seed7 (merged), got top=%d bottom=%d", lb3.TopTeamID, lb3.BottomTeamID)
	}
}

func TestAdvanceBracket_BottomSlot(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	teamList := db.ReturnTeamsByDivisionID(did)
	env := &Env{DB: db}

	bid := db.CreateBracket(did, "single_elimination", 4)
	db.SetBracketStatus(bid, "active")

	// Round 1, position 2 (even → bottom slot of parent)
	bg2 := db.AddBracketGame(bid, 1, 2, "winners")
	db.SetBracketGameTeams(bg2, teamList[0].ID, teamList[1].ID, false, false)
	// Final (round 2, position 1)
	finalID := db.AddBracketGame(bid, 2, 1, "winners")

	gid := db.AddGame(tid, did, teamList[0].ID, teamList[1].ID, "", time.Time{}, "")
	db.SetBracketGameGameID(bg2, gid)
	db.ScoreGame(gid, 5, 2)

	env.AdvanceBracket(gid, teamList[0].ID)

	finalState := db.GetBracketGameByID(finalID)
	if finalState.BottomTeamID != teamList[0].ID {
		t.Errorf("final bottom team: got %d, want %d", finalState.BottomTeamID, teamList[0].ID)
	}
	if finalState.TopTeamID != 0 {
		t.Errorf("final top team should still be 0 (TBD), got %d", finalState.TopTeamID)
	}
}
