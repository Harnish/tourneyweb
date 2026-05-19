package mydb

import (
	"testing"
	"time"
)

func TestSetTournamentDefaultRanking(t *testing.T) {
	db := NewFakeDB()
	id := db.AddTournament("Test", "Baseball", "Here", "", time.Time{}, "draft")
	criteria := []string{"wins", "runs_against"}
	db.SetTournamentDefaultRanking(id, criteria)
	got := db.ReturnTournamentByID(id)
	if len(got.DefaultRankingCriteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d: %v", len(got.DefaultRankingCriteria), got.DefaultRankingCriteria)
	}
	if got.DefaultRankingCriteria[0] != "wins" || got.DefaultRankingCriteria[1] != "runs_against" {
		t.Errorf("expected [wins runs_against], got %v", got.DefaultRankingCriteria)
	}
}

func TestSetTournamentDefaultRankingEmpty(t *testing.T) {
	db := NewFakeDB()
	id := db.AddTournament("Test", "Baseball", "Here", "", time.Time{}, "draft")
	got := db.ReturnTournamentByID(id)
	if len(got.DefaultRankingCriteria) != 0 {
		t.Errorf("expected nil/empty DefaultRankingCriteria on new tournament, got %v", got.DefaultRankingCriteria)
	}
}
