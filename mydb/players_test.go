package mydb

import (
	"testing"
	"time"
)

func setupPlayerDB(t *testing.T) (*FakeDB, int) {
	t.Helper()
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	db.AddTeam(tid, did, "Eagles", "Smith")
	var teamID int
	for id := range db.teams {
		teamID = id
	}
	return db, teamID
}

func TestAddPlayer_GetPlayersByTeamID(t *testing.T) {
	db, teamID := setupPlayerDB(t)
	id := db.AddPlayer(teamID, "42", "John", "Doe", "R", "Pitcher")
	if id == 0 {
		t.Fatal("AddPlayer returned 0")
	}
	players := db.GetPlayersByTeamID(teamID)
	if len(players) != 1 {
		t.Fatalf("want 1 player, got %d", len(players))
	}
	p := players[0]
	if p.ID != id {
		t.Errorf("ID: want %d, got %d", id, p.ID)
	}
	if p.TeamID != teamID {
		t.Errorf("TeamID: want %d, got %d", teamID, p.TeamID)
	}
	if p.Number != "42" {
		t.Errorf("Number: want 42, got %s", p.Number)
	}
	if p.First != "John" {
		t.Errorf("First: want John, got %s", p.First)
	}
	if p.Last != "Doe" {
		t.Errorf("Last: want Doe, got %s", p.Last)
	}
	if p.Handed != "R" {
		t.Errorf("Handed: want R, got %s", p.Handed)
	}
	if p.Position != "Pitcher" {
		t.Errorf("Position: want Pitcher, got %s", p.Position)
	}
}

func TestDisplayName(t *testing.T) {
	p := Player{First: "John", Last: "Doe"}
	if got := p.DisplayName(); got != "J. Doe" {
		t.Errorf("DisplayName: want 'J. Doe', got %q", got)
	}
}

func TestGetPlayerByID(t *testing.T) {
	db, teamID := setupPlayerDB(t)
	id := db.AddPlayer(teamID, "7", "Alice", "Smith", "L", "Catcher")
	p, ok := db.GetPlayerByID(id)
	if !ok {
		t.Fatal("GetPlayerByID: not found")
	}
	if p.Last != "Smith" {
		t.Errorf("Last: want Smith, got %s", p.Last)
	}
	_, ok2 := db.GetPlayerByID(99999)
	if ok2 {
		t.Error("GetPlayerByID: expected not found for unknown ID")
	}
}

func TestUpdatePlayer(t *testing.T) {
	db, teamID := setupPlayerDB(t)
	id := db.AddPlayer(teamID, "1", "Alice", "Smith", "L", "Catcher")
	db.UpdatePlayer(id, "99", "Alice", "Jones", "R", "Pitcher")
	p, ok := db.GetPlayerByID(id)
	if !ok {
		t.Fatal("not found after update")
	}
	if p.Number != "99" {
		t.Errorf("Number: want 99, got %s", p.Number)
	}
	if p.Last != "Jones" {
		t.Errorf("Last: want Jones, got %s", p.Last)
	}
	if p.Handed != "R" {
		t.Errorf("Handed: want R, got %s", p.Handed)
	}
	if p.Position != "Pitcher" {
		t.Errorf("Position: want Pitcher, got %s", p.Position)
	}
}

func TestDeletePlayer(t *testing.T) {
	db, teamID := setupPlayerDB(t)
	id := db.AddPlayer(teamID, "5", "Bob", "Lee", "", "Shortstop")
	db.DeletePlayer(id)
	players := db.GetPlayersByTeamID(teamID)
	if len(players) != 0 {
		t.Errorf("want 0 players after delete, got %d", len(players))
	}
	_, ok := db.GetPlayerByID(id)
	if ok {
		t.Error("GetPlayerByID: should return not-found after delete")
	}
}
