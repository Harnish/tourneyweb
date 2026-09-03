package mydb

import (
	"testing"
	"time"
)

func TestSetTeamGameChanger(t *testing.T) {
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

	if got := db.ReturnTeamByID(teamID).GameChangerURL; got != "" {
		t.Fatalf("new team should have empty GameChangerURL, got %q", got)
	}

	db.SetTeamGameChanger(teamID, "https://web.gamechanger.io/teams/abc")
	if got := db.ReturnTeamByID(teamID).GameChangerURL; got != "https://web.gamechanger.io/teams/abc" {
		t.Errorf("after set: want URL, got %q", got)
	}

	db.SetTeamGameChanger(teamID, "")
	if got := db.ReturnTeamByID(teamID).GameChangerURL; got != "" {
		t.Errorf("after clear: want empty, got %q", got)
	}
}
