package mydb_test

import (
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func reportFixture(t *testing.T) (*mydb.FakeDB, int) {
	t.Helper()
	db, _, add, _, _ := slotFixture(t)
	return db, add(t10)
}

func TestSaveScoreReportKeepsReportedAt(t *testing.T) {
	db, g := reportFixture(t)
	db.SaveScoreReport(g, "coach_home", 7, 5, 3)
	first := db.ScoreReportsForGame(g)[0]
	time.Sleep(2 * time.Millisecond)
	db.SaveScoreReport(g, "coach_home", 8, 6, 3)
	got := db.ScoreReportsForGame(g)
	if len(got) != 1 {
		t.Fatalf("one row per reporter, got %d", len(got))
	}
	r := got[0]
	if !r.ReportedAt.Equal(first.ReportedAt) {
		t.Fatal("ReportedAt must never change")
	}
	if !r.UpdatedAt.After(first.UpdatedAt) || r.HomeScore != 6 || r.AwayScore != 3 || r.UserID != 8 || r.GameID != g || r.Reporter != "coach_home" {
		t.Fatalf("edit not applied: %+v", r)
	}
}

func TestScoreReportsOrderedByFirstReported(t *testing.T) {
	db, g := reportFixture(t)
	db.SaveScoreReport(g, "coach_away", 1, 2, 4)
	time.Sleep(2 * time.Millisecond)
	db.SaveScoreReport(g, "coach_home", 2, 4, 2)
	time.Sleep(2 * time.Millisecond)
	db.SaveScoreReport(g, "coach_away", 1, 9, 9) // edit keeps its place
	got := db.ScoreReportsForGame(g)
	if len(got) != 2 || got[0].Reporter != "coach_away" || got[1].Reporter != "coach_home" {
		t.Fatalf("order by ReportedAt: %+v", got)
	}
}

func TestScoreStatusRoundTrip(t *testing.T) {
	db, g := reportFixture(t)
	if db.ReturnGameByID(g).ScoreStatus != "" {
		t.Fatal("default is unscored")
	}
	db.SetGameScoreStatus(g, "disputed")
	if db.ReturnGameByID(g).ScoreStatus != "disputed" {
		t.Fatal("status not stored")
	}
}

func TestDelGameRemovesReports(t *testing.T) {
	db, g := reportFixture(t)
	db.SaveScoreReport(g, "coach_home", 1, 1, 0)
	db.DelGame(g)
	if len(db.ScoreReportsForGame(g)) != 0 {
		t.Fatal("reports go with the game")
	}
}

var _ mydb.DB = (*mydb.FakeDB)(nil)

func TestClearScoreReports(t *testing.T) {
	db, g := reportFixture(t)
	db.SaveScoreReport(g, "coach_home", 1, 5, 3)
	db.SaveScoreReport(g, "coach_away", 2, 4, 3)
	db.SetGameScoreStatus(g, "disputed")
	db.ClearScoreReports(g)
	if len(db.ScoreReportsForGame(g)) != 0 || db.ReturnGameByID(g).ScoreStatus != "" {
		t.Fatalf("reports and status must reset: %+v", db.ReturnGameByID(g))
	}
	db.SaveScoreReport(g, "coach_home", 1, 5, 3)
	db.SetGameScoreStatus(g, "final")
	db.ClearScoreReports(g)
	if len(db.ScoreReportsForGame(g)) != 0 || db.ReturnGameByID(g).ScoreStatus != "final" {
		t.Fatalf("final status stays: %+v", db.ReturnGameByID(g))
	}
}
