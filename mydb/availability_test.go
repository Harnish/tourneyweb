package mydb_test

import (
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestAvailabilityNewThenNoteEditIsNotNew(t *testing.T) {
	db := mydb.NewFakeDB()
	u := newUmpire(t, db, "u@x.com", "Una")
	if !db.SetUmpireAvailability(u, 1, "weekends") {
		t.Fatal("first volunteer is new")
	}
	if db.SetUmpireAvailability(u, 1, "weekends and Fridays") {
		t.Fatal("editing the note is not new")
	}
	got := db.UmpireAvailabilityForUser(u)
	if len(got) != 1 || got[1].Note != "weekends and Fridays" || got[1].CreatedAt.IsZero() || got[1].UserID != u || got[1].TournamentID != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestAvailabilityClear(t *testing.T) {
	db := mydb.NewFakeDB()
	u := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireAvailability(u, 1, "")
	db.ClearUmpireAvailability(u, 1)
	if len(db.UmpireAvailabilityForUser(u)) != 0 || len(db.UmpireAvailabilityForTournament(1)) != 0 {
		t.Fatal("clear removes the row")
	}
	if !db.SetUmpireAvailability(u, 1, "") {
		t.Fatal("volunteering again after withdrawing is new (notifies again)")
	}
}

func TestAvailabilityPerTournament(t *testing.T) {
	db := mydb.NewFakeDB()
	a := newUmpire(t, db, "a@x.com", "A")
	b := newUmpire(t, db, "b@x.com", "B")
	db.SetUmpireAvailability(a, 1, "x")
	db.SetUmpireAvailability(b, 2, "y")
	db.SetUmpireAvailability(b, 1, "z")
	t1 := db.UmpireAvailabilityForTournament(1)
	if len(t1) != 2 || t1[a].Note != "x" || t1[b].Note != "z" {
		t.Fatalf("tournament 1: %+v", t1)
	}
	if t2 := db.UmpireAvailabilityForTournament(2); len(t2) != 1 || t2[b].Note != "y" {
		t.Fatalf("tournament 2: %+v", t2)
	}
}

func TestOpenForUmpiresFiltersDraftAndPast(t *testing.T) {
	db := mydb.NewFakeDB()
	day := 24 * time.Hour
	now := time.Now()
	db.AddTournament("future", "baseball", "L", "", now.Add(30*day), "published")
	db.AddTournament("soon", "baseball", "L", "", now.Add(2*day), "published")
	db.AddTournament("started 3 days ago", "baseball", "L", "", now.Add(-3*day), "active")
	db.AddTournament("started 10 days ago", "baseball", "L", "", now.Add(-10*day), "published")
	db.AddTournament("draft", "baseball", "L", "", now.Add(5*day), "draft")
	got := db.ReturnTournamentsOpenForUmpires(now.Add(-7 * day))
	var names []string
	for _, tr := range got {
		names = append(names, tr.Name)
	}
	want := []string{"started 3 days ago", "soon", "future"}
	if len(names) != len(want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v want %v (ordered by start date)", names, want)
		}
	}
}
