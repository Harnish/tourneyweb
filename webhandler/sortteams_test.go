package webhandler

import (
	"testing"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// makeTeam builds a Team with pre-populated stats. ID is used as a stable
// identifier for assertions and for DidTeamABeatTeamB lookups.
func makeTeam(id, wins, runsFor, runsAgainst int) mydb.Team {
	return mydb.Team{ID: id, Wins: wins, RunsFor: runsFor, RunsAgainst: runsAgainst}
}

func makeEnv() *Env {
	return &Env{DB: mydb.NewFakeDB()}
}

// ids extracts the ID order from a sorted slice for easy assertion.
func ids(teams []mydb.Team) []int {
	out := make([]int, len(teams))
	for i, t := range teams {
		out[i] = t.ID
	}
	return out
}

// recordH2H records that teamA beat teamB (score 1-0) in the FakeDB.
func recordH2H(db *mydb.FakeDB, teamA, teamB int) {
	db.AddTeamScore(0, 0, teamA, teamB, teamA*100+teamB, 1, 0)
	db.AddTeamScore(0, 0, teamB, teamA, teamA*100+teamB, 0, 1)
}

// --- Helper function unit tests ---

func TestWins(t *testing.T) {
	a := makeTeam(1, 3, 0, 0)
	b := makeTeam(2, 2, 0, 0)
	c := makeTeam(3, 3, 0, 0)

	eq, better := Wins(a, b)
	if eq || !better {
		t.Errorf("Wins(more, fewer): got (%v,%v), want (false,true)", eq, better)
	}
	eq, better = Wins(b, a)
	if eq || better {
		t.Errorf("Wins(fewer, more): got (%v,%v), want (false,false)", eq, better)
	}
	eq, better = Wins(a, c)
	if !eq || !better {
		t.Errorf("Wins(equal, equal): got (%v,%v), want (true,true)", eq, better)
	}
}

func TestRunsAgainst(t *testing.T) {
	fewer := makeTeam(1, 0, 0, 5)
	more := makeTeam(2, 0, 0, 10)
	same := makeTeam(3, 0, 0, 5)

	eq, better := RunsAgainst(fewer, more)
	if eq || !better {
		t.Errorf("RunsAgainst(fewer, more): got (%v,%v), want (false,true)", eq, better)
	}
	eq, better = RunsAgainst(more, fewer)
	if eq || better {
		t.Errorf("RunsAgainst(more, fewer): got (%v,%v), want (false,false)", eq, better)
	}
	eq, better = RunsAgainst(fewer, same)
	if !eq || !better {
		t.Errorf("RunsAgainst(equal, equal): got (%v,%v), want (true,true)", eq, better)
	}
}

func TestRunsFor(t *testing.T) {
	more := makeTeam(1, 0, 15, 0)
	fewer := makeTeam(2, 0, 8, 0)
	same := makeTeam(3, 0, 15, 0)

	eq, better := RunsFor(more, fewer)
	if eq || !better {
		t.Errorf("RunsFor(more, fewer): got (%v,%v), want (false,true)", eq, better)
	}
	eq, better = RunsFor(fewer, more)
	if eq || better {
		t.Errorf("RunsFor(fewer, more): got (%v,%v), want (false,false)", eq, better)
	}
	eq, better = RunsFor(more, same)
	if !eq || !better {
		t.Errorf("RunsFor(equal, equal): got (%v,%v), want (true,true)", eq, better)
	}
}

// --- WinsRunsAgainstRunsEarnedHead2Head ---
// Tiebreaker order: wins → runs-against (fewer=better) → runs-for (more=better) → head-to-head

func TestWinsRunsAgainstHead2Head_ByWins(t *testing.T) {
	env := makeEnv()
	teams := []mydb.Team{
		makeTeam(3, 1, 10, 8),
		makeTeam(1, 3, 10, 8),
		makeTeam(2, 2, 10, 8),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "runs_against", "runs_for", "head_to_head"}))
	want := []int{1, 2, 3}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got ID %d, want %d (full order: %v)", i, got[i], id, got)
		}
	}
}

func TestWinsRunsAgainstHead2Head_ByRunsAgainst(t *testing.T) {
	env := makeEnv()
	// All tied on wins; fewer runs against ranks higher
	teams := []mydb.Team{
		makeTeam(3, 2, 10, 12),
		makeTeam(1, 2, 10, 6),
		makeTeam(2, 2, 10, 9),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "runs_against", "runs_for", "head_to_head"}))
	want := []int{1, 2, 3}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got ID %d, want %d (full order: %v)", i, got[i], id, got)
		}
	}
}

func TestWinsRunsAgainstHead2Head_ByRunsFor(t *testing.T) {
	env := makeEnv()
	// Tied on wins and runs-against; more runs for ranks higher.
	// IDs 1,3,2 have runsFor 15,12,8 so expected order is [1,3,2].
	teams := []mydb.Team{
		makeTeam(2, 2, 8, 6),
		makeTeam(3, 2, 12, 6),
		makeTeam(1, 2, 15, 6),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "runs_against", "runs_for", "head_to_head"}))
	want := []int{1, 3, 2}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got ID %d, want %d (full order: %v)", i, got[i], id, got)
		}
	}
}

func TestWinsRunsAgainstHead2Head_ByHead2Head(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Tied on all stats; team 1 beat team 2 directly
	recordH2H(db, 1, 2)
	teams := []mydb.Team{
		makeTeam(2, 2, 10, 6),
		makeTeam(1, 2, 10, 6),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "runs_against", "runs_for", "head_to_head"}))
	if got[0] != 1 {
		t.Errorf("head-to-head winner should rank first; got order %v", got)
	}
}

func TestWinsRunsAgainstHead2Head_ThreeTeams(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Team 1: most wins; team 2: tied wins with 3 but fewer runs against
	teams := []mydb.Team{
		makeTeam(3, 2, 10, 10),
		makeTeam(2, 2, 10, 7),
		makeTeam(1, 3, 10, 5),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "runs_against", "runs_for", "head_to_head"}))
	want := []int{1, 2, 3}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got ID %d, want %d (full order: %v)", i, got[i], id, got)
		}
	}
}

// --- WinsHead2HeadRunsAgainstRunsEarned ---
// Tiebreaker order: wins → head-to-head (if played) → runs-against → runs-for

func TestWinsHead2Head_ByWins(t *testing.T) {
	env := makeEnv()
	teams := []mydb.Team{
		makeTeam(2, 1, 10, 8),
		makeTeam(3, 3, 10, 8),
		makeTeam(1, 2, 10, 8),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"}))
	want := []int{3, 1, 2}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got ID %d, want %d (full order: %v)", i, got[i], id, got)
		}
	}
}

func TestWinsHead2Head_ByHead2Head(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Tied wins; team 1 beat team 2 directly
	recordH2H(db, 1, 2)
	teams := []mydb.Team{
		makeTeam(2, 2, 10, 6),
		makeTeam(1, 2, 10, 6),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"}))
	if got[0] != 1 {
		t.Errorf("head-to-head winner should rank first; got order %v", got)
	}
}

func TestWinsHead2Head_ByRunsAgainst(t *testing.T) {
	env := makeEnv()
	// Tied wins, didn't play each other; fewer runs against ranks higher
	teams := []mydb.Team{
		makeTeam(2, 2, 10, 12),
		makeTeam(1, 2, 10, 5),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"}))
	if got[0] != 1 {
		t.Errorf("fewer runs-against should rank first; got order %v", got)
	}
}

func TestWinsHead2Head_ByRunsFor(t *testing.T) {
	env := makeEnv()
	// Tied wins + runs-against, didn't play; more runs for ranks higher
	teams := []mydb.Team{
		makeTeam(2, 2, 8, 6),
		makeTeam(1, 2, 15, 6),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"}))
	if got[0] != 1 {
		t.Errorf("more runs-for should rank first; got order %v", got)
	}
}

func TestWinsHead2Head_ThreeTeamsAllDifferentWins(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	teams := []mydb.Team{
		makeTeam(2, 2, 12, 6),
		makeTeam(1, 3, 15, 4),
		makeTeam(3, 1, 8, 9),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"}))
	want := []int{1, 2, 3}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got ID %d, want %d (full order: %v)", i, got[i], id, got)
		}
	}
}

func TestWinsHead2Head_ThreeWayTieResolvesViaHead2Head(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// All tied on wins/runs; 1 beat 2, 1 beat 3, 2 beat 3
	recordH2H(db, 1, 2)
	recordH2H(db, 1, 3)
	recordH2H(db, 2, 3)
	teams := []mydb.Team{
		makeTeam(3, 2, 10, 6),
		makeTeam(1, 2, 10, 6),
		makeTeam(2, 2, 10, 6),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"}))
	if got[0] != 1 {
		t.Errorf("team 1 beat everyone, should rank first; got order %v", got)
	}
	if got[2] != 3 {
		t.Errorf("team 3 lost to everyone, should rank last; got order %v", got)
	}
}

// makeTeamL builds a Team with wins, losses, and run stats.
func makeTeamL(id, wins, losses, runsFor, runsAgainst int) mydb.Team {
	return mydb.Team{ID: id, Wins: wins, Losses: losses, RunsFor: runsFor, RunsAgainst: runsAgainst}
}

func TestCriterionWinPct(t *testing.T) {
	env := makeEnv()
	// Team 1: 2W 0L = 1.0; Team 2: 1W 1L = 0.5
	teams := []mydb.Team{
		makeTeamL(2, 1, 1, 5, 5),
		makeTeamL(1, 2, 0, 5, 5),
	}
	got := ids(env.SortTeams(teams, []string{"win_pct"}))
	if got[0] != 1 {
		t.Errorf("higher win_pct should rank first; got %v", got)
	}
}

func TestCriterionRunDifferential(t *testing.T) {
	env := makeEnv()
	// Team 1: +10 diff; Team 2: +3 diff
	teams := []mydb.Team{
		makeTeamL(2, 2, 0, 8, 5),
		makeTeamL(1, 2, 0, 15, 5),
	}
	got := ids(env.SortTeams(teams, []string{"run_differential"}))
	if got[0] != 1 {
		t.Errorf("higher run diff should rank first; got %v", got)
	}
}

func TestCriterionRunsAgainstPerGame(t *testing.T) {
	env := makeEnv()
	// Team 1: 6 RA / 2G = 3.0; Team 2: 12 RA / 2G = 6.0
	teams := []mydb.Team{
		makeTeamL(2, 1, 1, 10, 12),
		makeTeamL(1, 1, 1, 10, 6),
	}
	got := ids(env.SortTeams(teams, []string{"runs_against_per_game"}))
	if got[0] != 1 {
		t.Errorf("fewer RA/game should rank first; got %v", got)
	}
}

func TestCriterionHeadToHeadRunsAgainst(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Team 1 allowed 2 runs vs Team 2; Team 2 allowed 5 runs vs Team 1
	db.AddTeamScore(0, 0, 1, 2, 999, 5, 2) // game 999: T1 scored 5, T2 scored 2
	db.AddTeamScore(0, 0, 2, 1, 999, 2, 5)
	teams := []mydb.Team{
		makeTeam(2, 2, 10, 5),
		makeTeam(1, 2, 10, 5),
	}
	got := ids(env.SortTeams(teams, []string{"head_to_head_runs_against"}))
	if got[0] != 1 {
		t.Errorf("fewer H2H runs allowed should rank first; got %v", got)
	}
}

func TestCriterionCoinFlip(t *testing.T) {
	env := makeEnv()
	// Stable sort by team ID ascending
	teams := []mydb.Team{makeTeam(3, 2, 10, 5), makeTeam(1, 2, 10, 5), makeTeam(2, 2, 10, 5)}
	got := ids(env.SortTeams(teams, []string{"coin_flip"}))
	want := []int{1, 2, 3}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("coin_flip position %d: got %d, want %d (full: %v)", i, got[i], id, got)
		}
	}
}

func TestSortTeams_MultiCriteria(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Team 1 and 2 tied on wins; 1 beat 2 head-to-head
	recordH2H(db, 1, 2)
	teams := []mydb.Team{
		makeTeam(2, 3, 10, 5),
		makeTeam(1, 3, 10, 5),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against"}))
	if got[0] != 1 {
		t.Errorf("h2h winner should rank first; got %v", got)
	}
}

func TestSortTeams_UnknownKeyIgnored(t *testing.T) {
	env := makeEnv()
	teams := []mydb.Team{makeTeam(2, 3, 10, 5), makeTeam(1, 2, 10, 5)}
	// Unknown key should be silently skipped; falls through to no result → stable order
	got := ids(env.SortTeams(teams, []string{"wins", "nonexistent_key"}))
	if got[0] != 2 {
		t.Errorf("wins criterion should still apply; got %v", got)
	}
}
