package mydb

import (
	"testing"
	"time"
)

func TestBracketCRUD(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}

	bid := db.CreateBracket(did, "single_elimination", 4)
	if bid == 0 {
		t.Fatal("CreateBracket returned 0")
	}
	b := db.GetBracketByID(bid)
	if b.DivisionID != did || b.Status != "seeding" || b.Size != 4 {
		t.Fatalf("unexpected bracket: %+v", b)
	}
	b2 := db.GetBracketByDivisionID(did)
	if b2.ID != bid {
		t.Fatalf("GetBracketByDivisionID: got %d, want %d", b2.ID, bid)
	}
	db.SetBracketStatus(bid, "active")
	if db.GetBracketByID(bid).Status != "active" {
		t.Fatal("SetBracketStatus did not update")
	}
}

func TestBracketSeeds(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	var teams []int
	for id := range db.teams {
		teams = append(teams, id)
	}

	bid := db.CreateBracket(did, "single_elimination", 4)
	db.AddBracketSeed(bid, 1, teams[0])
	db.AddBracketSeed(bid, 2, teams[1])
	db.AddBracketSeed(bid, 3, 0) // bye
	db.AddBracketSeed(bid, 4, 0) // bye

	seeds := db.GetBracketSeeds(bid)
	if len(seeds) != 4 {
		t.Fatalf("want 4 seeds, got %d", len(seeds))
	}
	if seeds[0].Seed != 1 || seeds[1].Seed != 2 {
		t.Fatal("seeds not ordered by seed number")
	}
	if seeds[2].TeamID != 0 {
		t.Fatal("bye slot should have TeamID 0")
	}

	db.UpdateBracketSeeds(bid, []int{teams[1], teams[0], 0, 0})
	seeds = db.GetBracketSeeds(bid)
	if seeds[0].TeamID != teams[1] {
		t.Fatal("UpdateBracketSeeds did not reorder")
	}
}

func TestBracketGames(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	var teams []int
	for id := range db.teams {
		teams = append(teams, id)
	}

	bid := db.CreateBracket(did, "single_elimination", 4)
	bgid := db.AddBracketGame(bid, 1, 1, "winners")
	if bgid == 0 {
		t.Fatal("AddBracketGame returned 0")
	}

	db.SetBracketGameTeams(bgid, teams[0], teams[1], false, false)
	bg := db.GetBracketGameByID(bgid)
	if bg.TopTeamID != teams[0] || bg.BottomTeamID != teams[1] {
		t.Fatalf("SetBracketGameTeams: got %+v", bg)
	}

	db.SetBracketGameWinner(bgid, teams[0])
	bg = db.GetBracketGameByID(bgid)
	if bg.WinnerTeamID != teams[0] {
		t.Fatal("SetBracketGameWinner did not update")
	}

	gid := db.AddGame(tid, did, teams[0], teams[1], "", time.Time{}, "")
	db.SetBracketGameGameID(bgid, gid)
	bg = db.GetBracketGameByGameID(gid)
	if bg.ID != bgid {
		t.Fatal("GetBracketGameByGameID returned wrong game")
	}
}

func TestDivisionPhase(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	d := db.ReturnDivisionByID(did)
	if d.Phase != "pool" {
		t.Fatalf("default phase should be 'pool', got %q", d.Phase)
	}
	db.SetDivisionPhase(did, "bracket")
	d = db.ReturnDivisionByID(did)
	if d.Phase != "bracket" {
		t.Fatalf("phase should be 'bracket', got %q", d.Phase)
	}
}

func TestAddGameReturnsID(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	var teams []int
	for id := range db.teams {
		teams = append(teams, id)
	}
	gid := db.AddGame(tid, did, teams[0], teams[1], "Field 1", time.Time{}, "")
	if gid == 0 {
		t.Fatal("AddGame should return non-zero ID")
	}
	g := db.ReturnGameByID(gid)
	if g.ID != gid {
		t.Fatalf("ReturnGameByID: got %d, want %d", g.ID, gid)
	}
}
