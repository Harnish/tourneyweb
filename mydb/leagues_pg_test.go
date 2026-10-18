package mydb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestLeaguesPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	lg := d.AddTournament("League "+sfx, "baseball", "L", "", day(-10), "published")
	end := day(40)
	d.SetTournamentLeague(lg, "league", end, 0)
	got := d.ReturnTournamentByID(lg)
	if !got.IsLeague() || got.EndDate.Format("2006-01-02") != end.Format("2006-01-02") || got.LeagueID != 0 || !got.Started(time.Now()) {
		t.Fatalf("league: %+v", got)
	}
	po := d.AddTournament("Playoffs "+sfx, "baseball", "L", "", day(45), "draft")
	d.SetTournamentLeague(po, "tournament", time.Time{}, lg)
	if g := d.ReturnTournamentByID(po); g.IsLeague() || g.LeagueID != lg || !g.EndDate.IsZero() {
		t.Fatalf("playoff: %+v", g)
	}
	if pl := d.LeaguePlayoffs(lg); len(pl) != 1 || pl[0].ID != po {
		t.Fatalf("playoffs: %+v", pl)
	}
	found := false
	for _, l := range d.ReturnLeagues(time.Now()) {
		if l.ID == lg {
			found = true
		}
		if l.ID == po {
			t.Fatal("playoff is not a league")
		}
	}
	if !found {
		t.Fatal("running league must be listed")
	}
	for _, l := range d.ReturnTournamentsComingUp() {
		if l.ID == lg {
			t.Fatal("league leaked into a tournament list")
		}
	}
	d.SetTournamentLeague(lg, "league", day(-1), 0)
	for _, l := range d.ReturnLeagues(time.Now()) {
		if l.ID == lg {
			t.Fatal("ended league must be hidden")
		}
	}
	if s := d.SearchTournaments("league " + sfx); len(s) != 1 || !s[0].IsLeague() {
		t.Fatalf("search finds leagues: %+v", s)
	}
}
