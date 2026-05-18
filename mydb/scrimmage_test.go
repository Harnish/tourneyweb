package mydb

import (
	"testing"
	"time"
)

func TestSetGameScrimmage_SkipsStatsForScrimmageTeam(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	var teamIDs []int
	for id := range db.teams {
		teamIDs = append(teamIDs, id)
	}
	homeID, awayID := teamIDs[0], teamIDs[1]

	gid := db.AddGame(tid, did, homeID, awayID, "", time.Time{}, "")
	db.SetGameScrimmage(gid, awayID) // away team's game doesn't count

	db.ScoreGame(gid, 5, 2) // home team wins

	allTeams := db.ReturnTeamsByDivisionIDWithStats(did)
	var homeWins, awayLosses int
	for _, tm := range allTeams {
		if tm.ID == homeID {
			homeWins = tm.Wins
		}
		if tm.ID == awayID {
			awayLosses = tm.Losses
		}
	}
	if homeWins != 1 {
		t.Errorf("home team: want 1 win, got %d", homeWins)
	}
	if awayLosses != 0 {
		t.Errorf("scrimmage team: want 0 losses, got %d (game should not count)", awayLosses)
	}
}

func TestSetGameScrimmage_BothTeamsCountWhenNoScrimmage(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	var did int
	for id := range db.divisions {
		did = id
	}
	db.AddTeam(tid, did, "Eagles", "")
	db.AddTeam(tid, did, "Cubs", "")
	var teamIDs []int
	for id := range db.teams {
		teamIDs = append(teamIDs, id)
	}
	homeID, awayID := teamIDs[0], teamIDs[1]

	gid := db.AddGame(tid, did, homeID, awayID, "", time.Time{}, "")
	// No SetGameScrimmage call — game counts for both

	db.ScoreGame(gid, 5, 2)

	allTeams := db.ReturnTeamsByDivisionIDWithStats(did)
	var homeWins, awayLosses int
	for _, tm := range allTeams {
		if tm.ID == homeID {
			homeWins = tm.Wins
		}
		if tm.ID == awayID {
			awayLosses = tm.Losses
		}
	}
	if homeWins != 1 {
		t.Errorf("home team: want 1 win, got %d", homeWins)
	}
	if awayLosses != 1 {
		t.Errorf("away team: want 1 loss, got %d", awayLosses)
	}
}
