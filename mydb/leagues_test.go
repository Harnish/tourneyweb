package mydb_test

import (
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func day(offset int) time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, offset)
}

func TestTournamentDefaultsToKindTournament(t *testing.T) {
	db := mydb.NewFakeDB()
	id := db.AddTournament("T", "baseball", "L", "", day(0), "published")
	got := db.ReturnTournamentByID(id)
	if got.Kind != "tournament" || got.IsLeague() || !got.EndDate.IsZero() || got.LeagueID != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestSetTournamentLeagueRoundTrip(t *testing.T) {
	db := mydb.NewFakeDB()
	lg := db.AddTournament("League", "baseball", "L", "", day(0), "published")
	po := db.AddTournament("Playoffs", "baseball", "L", "", day(30), "published")
	end := day(60)
	db.SetTournamentLeague(lg, "league", end, 0)
	db.SetTournamentLeague(po, "tournament", time.Time{}, lg)
	if got := db.ReturnTournamentByID(lg); !got.IsLeague() || !got.EndDate.Equal(end) {
		t.Fatalf("league: %+v", got)
	}
	if got := db.ReturnTournamentByID(po); got.IsLeague() || got.LeagueID != lg {
		t.Fatalf("playoff: %+v", got)
	}
	db.SetTournamentLeague(lg, "league", time.Time{}, 0)
	if got := db.ReturnTournamentByID(lg); !got.EndDate.IsZero() {
		t.Fatalf("zero end date clears: %+v", got)
	}
}

func TestStartedBoundary(t *testing.T) {
	now := time.Now()
	for off, want := range map[int]bool{0: true, 1: false, -1: true} {
		tt := mydb.Tournament{StartDate: day(off).Add(15 * time.Hour)}
		if got := tt.Started(now); got != want {
			t.Errorf("start offset %d: got %v want %v", off, got, want)
		}
	}
}

func TestListsExcludeLeagues(t *testing.T) {
	db := mydb.NewFakeDB()
	soon := db.AddTournament("Soon Cup", "baseball", "L", "", day(2), "published")
	recent := db.AddTournament("Recent Cup", "baseball", "L", "", day(-2), "published")
	far := db.AddTournament("Far Cup", "baseball", "L", "", day(60), "published")
	old := db.AddTournament("Old Cup", "baseball", "L", "", day(-60), "published")
	for _, id := range []int{soon, recent, far, old} {
		db.AddTournament("Lg", "baseball", "L", "", db.ReturnTournamentByID(id).StartDate, "published")
	}
	var leagues []int
	for _, tt := range db.ReturnTournaments() {
		if tt.Name == "Lg" {
			db.SetTournamentLeague(tt.ID, "league", time.Time{}, 0)
			leagues = append(leagues, tt.ID)
		}
	}
	isLeague := func(ts []mydb.Tournament) bool {
		for _, tt := range ts {
			if tt.IsLeague() {
				return true
			}
		}
		return false
	}
	fut, _ := db.ReturnTournamentsFuture(1)
	past, _ := db.ReturnTournamentsPast(1)
	if isLeague(db.ReturnTournamentsComingUp()) || isLeague(db.ReturnTournamentsRecent()) || isLeague(fut) || isLeague(past) {
		t.Fatal("tournament lists must not hold leagues")
	}
	if len(db.ReturnTournamentsComingUp()) != 1 || len(db.ReturnTournamentsRecent()) != 1 || len(fut) != 1 || len(past) != 1 {
		t.Fatal("tournaments must still be listed")
	}
	if len(db.ReturnTournaments()) != 8 {
		t.Fatal("admin list is kind-agnostic")
	}
	if got := db.SearchTournaments("lg"); len(got) != len(leagues) {
		t.Fatalf("search must find leagues: %d", len(got))
	}
}

func TestReturnLeagues(t *testing.T) {
	db := mydb.NewFakeDB()
	mk := func(name string, start, end time.Time, status string) int {
		id := db.AddTournament(name, "baseball", "L", "", start, status)
		db.SetTournamentLeague(id, "league", end, 0)
		return id
	}
	mk("Draft", day(-5), day(30), "draft")
	mk("Ended", day(-90), day(-1), "published")
	mk("EndsToday", day(-90), day(0), "published")
	mk("OpenEnded11", day(-330), time.Time{}, "published")
	mk("OpenEnded13", day(-400), time.Time{}, "published")
	mk("Upcoming", day(10), day(90), "published")
	db.AddTournament("Plain", "baseball", "L", "", day(1), "published")
	var names []string
	for _, tt := range db.ReturnLeagues(time.Now()) {
		names = append(names, tt.Name)
	}
	want := []string{"Upcoming", "EndsToday", "OpenEnded11"}
	if len(names) != len(want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v want %v (newest start first)", names, want)
		}
	}
}

func TestLeaguePlayoffs(t *testing.T) {
	db := mydb.NewFakeDB()
	lg := db.AddTournament("L", "baseball", "L", "", day(-10), "published")
	db.SetTournamentLeague(lg, "league", time.Time{}, 0)
	a := db.AddTournament("A Playoffs", "baseball", "L", "", day(5), "draft")
	b := db.AddTournament("B Playoffs", "baseball", "L", "", day(9), "published")
	other := db.AddTournament("Other", "baseball", "L", "", day(7), "published")
	db.SetTournamentLeague(a, "tournament", time.Time{}, lg)
	db.SetTournamentLeague(b, "tournament", time.Time{}, lg)
	_ = other
	got := db.LeaguePlayoffs(lg)
	if len(got) != 2 || got[0].ID != b || got[1].ID != a {
		t.Fatalf("playoffs (all statuses, newest first): %+v", got)
	}
}
