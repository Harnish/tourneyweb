package mydb

import (
	"database/sql"
	"log"
)

type Team struct {
	ID           int
	TournamentID int
	Name         string
	Coach        string
	Division     Division
	Wins         int
	Losses       int
	RunsAgainst  int
	RunsFor      int
}

func (me *MyDB) AddTeam(tournamentID, divisionID int, name, coach string) {
	_, err := me.DB.Exec(
		`INSERT INTO teams (tournament_id, division_id, name, coach) VALUES ($1,$2,$3,$4)`,
		tournamentID, divisionID, name, coach,
	)
	if err != nil {
		log.Println("AddTeam:", err)
	}
}

func (me *MyDB) DelTeam(id int) {
	me.DB.Exec(`DELETE FROM teams WHERE id=$1`, id)
}

func (me *MyDB) ReturnTeamsByTournamentID(tournamentID int) []Team {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, coach, division_id FROM teams WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		log.Println("ReturnTeamsByTournamentID:", err)
		return nil
	}
	var teams []Team
	for rows.Next() {
		var t Team
		var did int
		if err := rows.Scan(&t.ID, &t.TournamentID, &t.Name, &t.Coach, &did); err != nil {
			log.Println("ReturnTeamsByTournamentID scan:", err)
			continue
		}
		t.Division = me.ReturnDivisionByID(did)
		teams = append(teams, t)
	}
	rows.Close()
	return teams
}

func (me *MyDB) UpdateTeam(id, divisionID int, name, coach string) {
	_, err := me.DB.Exec(
		`UPDATE teams SET division_id=$1, name=$2, coach=$3 WHERE id=$4`,
		divisionID, name, coach, id,
	)
	if err != nil {
		log.Println("UpdateTeam:", err)
	}
}

func (me *MyDB) ReturnTeamsByDivisionID(divisionID int) []Team {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, coach, division_id FROM teams WHERE division_id=$1`,
		divisionID,
	)
	if err != nil {
		log.Println("ReturnTeamsByDivisionID:", err)
		return nil
	}
	var teams []Team
	for rows.Next() {
		var t Team
		var did int
		if err := rows.Scan(&t.ID, &t.TournamentID, &t.Name, &t.Coach, &did); err != nil {
			log.Println("ReturnTeamsByDivisionID scan:", err)
			continue
		}
		t.Division = me.ReturnDivisionByID(did)
		teams = append(teams, t)
	}
	rows.Close()
	return teams
}

func (me *MyDB) ReturnTeamsByDivisionIDWithStats(divisionID int) []Team {
	teams := me.ReturnTeamsByDivisionID(divisionID)
	for i := range teams {
		teams[i].Wins = me.TeamWins(teams[i].ID)
		teams[i].Losses = me.TeamLosses(teams[i].ID)
		teams[i].RunsAgainst = me.TeamScoredAgainst(teams[i].ID)
		teams[i].RunsFor = me.TeamScoredFor(teams[i].ID)
	}
	return teams
}

func (me *MyDB) ReturnTeamByID(id int) Team {
	var t Team
	var did int
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name, coach, division_id FROM teams WHERE id=$1`, id,
	).Scan(&t.ID, &t.TournamentID, &t.Name, &t.Coach, &did)
	if err != nil && err != sql.ErrNoRows {
		log.Println("ReturnTeamByID:", err)
		return t
	}
	t.Division = me.ReturnDivisionByID(did)
	return t
}

func (me *MyDB) TeamWins(id int) int {
	var n int
	me.DB.QueryRow(
		`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1 AND team_score > opponent_score`, id,
	).Scan(&n)
	return n
}

func (me *MyDB) TeamLosses(id int) int {
	var n int
	me.DB.QueryRow(
		`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1 AND team_score < opponent_score`, id,
	).Scan(&n)
	return n
}

func (me *MyDB) TeamScoredFor(id int) int {
	var s sql.NullInt64
	me.DB.QueryRow(`SELECT SUM(team_score) FROM games_by_team WHERE team_id=$1`, id).Scan(&s)
	if !s.Valid {
		return 0
	}
	return int(s.Int64)
}

func (me *MyDB) TeamScoredAgainst(id int) int {
	var s sql.NullInt64
	me.DB.QueryRow(`SELECT SUM(opponent_score) FROM games_by_team WHERE team_id=$1`, id).Scan(&s)
	if !s.Valid {
		return 0
	}
	return int(s.Int64)
}

func (me *MyDB) GamesPlayedByTeam(id int) int {
	var n int
	me.DB.QueryRow(`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1`, id).Scan(&n)
	return n
}
