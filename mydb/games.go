package mydb

import (
	"database/sql"
	"log/slog"
	"time"
)

type Game struct {
	ID           int
	TournamentID int
	Division     Division
	HomeTeam     Team
	AwayTeam     Team
	Location     string
	Start        time.Time
	Umpire       string
	AwayScore        int
	HomeScore        int
	Scored           bool
	ScrimmageTeamID  int // team ID for whom this game doesn't count; 0 = counts for both
}

const gameSelect = `SELECT id, tournament_id, division_id, home_team_id, away_team_id, location, start_time, umpire, home_score, away_score, COALESCE(scrimmage_team_id,0) FROM games`

func (me *MyDB) scanGames(rows *sql.Rows) []Game {
	var out []Game
	for rows.Next() {
		var g Game
		var hid, aid, did int
		var homeScore, awayScore sql.NullInt64
		var startTime sql.NullTime
		if err := rows.Scan(&g.ID, &g.TournamentID, &did, &hid, &aid, &g.Location, &startTime, &g.Umpire, &homeScore, &awayScore, &g.ScrimmageTeamID); err != nil {
			slog.Error("scanGames", "err", err)
			continue
		}
		if startTime.Valid {
			g.Start = startTime.Time
		}
		g.Division = me.ReturnDivisionByID(did)
		g.HomeTeam = me.ReturnTeamByID(hid)
		g.AwayTeam = me.ReturnTeamByID(aid)
		if homeScore.Valid && awayScore.Valid {
			g.HomeScore = int(homeScore.Int64)
			g.AwayScore = int(awayScore.Int64)
			g.Scored = true
		}
		out = append(out, g)
	}
	rows.Close()
	return out
}

func (me *MyDB) AddGame(tournamentID, divisionID, homeTeamID, awayTeamID int, location string, startTime time.Time, umpire string) int {
	var st interface{}
	if !startTime.IsZero() {
		st = startTime
	}
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO games (tournament_id, division_id, home_team_id, away_team_id, location, start_time, umpire) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		tournamentID, divisionID, homeTeamID, awayTeamID, location, st, umpire,
	).Scan(&id)
	if err != nil {
		slog.Error("AddGame", "err", err)
	}
	return id
}

func (me *MyDB) AllGamesByDivision(divisionID int) []Game {
	rows, err := me.DB.Query(gameSelect+` WHERE division_id=$1`, divisionID)
	if err != nil {
		slog.Error("AllGamesByDivision", "err", err)
		return nil
	}
	return me.scanGames(rows)
}

func (me *MyDB) AllGamesByTeam(teamID int) []Game {
	rows, err := me.DB.Query(gameSelect+` WHERE home_team_id=$1 OR away_team_id=$1`, teamID)
	if err != nil {
		slog.Error("AllGamesByTeam", "err", err)
		return nil
	}
	return me.scanGames(rows)
}

func (me *MyDB) ReturnGameByID(gameID int) Game {
	rows, err := me.DB.Query(gameSelect+` WHERE id=$1`, gameID)
	if err != nil {
		slog.Error("ReturnGameByID", "err", err)
		return Game{}
	}
	games := me.scanGames(rows)
	if len(games) == 0 {
		return Game{}
	}
	return games[0]
}

func (me *MyDB) DelGame(id int) {
	me.DB.Exec(`DELETE FROM games WHERE id=$1`, id)
}

func (me *MyDB) SetGameScrimmage(gameID, teamID int) {
	_, err := me.DB.Exec(`UPDATE games SET scrimmage_team_id=$1 WHERE id=$2`, teamID, gameID)
	if err != nil {
		slog.Error("SetGameScrimmage", "err", err)
	}
}

func (me *MyDB) UpdateGame(id, divisionID, homeTeamID, awayTeamID int, location string, startTime time.Time, umpire string) {
	var st interface{}
	if !startTime.IsZero() {
		st = startTime
	}
	_, err := me.DB.Exec(
		`UPDATE games SET division_id=$1, home_team_id=$2, away_team_id=$3, location=$4, start_time=$5, umpire=$6 WHERE id=$7`,
		divisionID, homeTeamID, awayTeamID, location, st, umpire, id,
	)
	if err != nil {
		slog.Error("UpdateGame", "err", err)
		return
	}
	// Re-fetch to get updated team/division references before re-syncing scores.
	game := me.ReturnGameByID(id)
	if game.Scored {
		me.DeleteTeamScore(id)
		me.AddTeamScore(game.TournamentID, game.Division.ID, game.HomeTeam.ID, game.AwayTeam.ID, game.ID, game.HomeScore, game.AwayScore)
		me.AddTeamScore(game.TournamentID, game.Division.ID, game.AwayTeam.ID, game.HomeTeam.ID, game.ID, game.AwayScore, game.HomeScore)
	}
}

func (me *MyDB) AllGames(tournamentID int) []Game {
	rows, err := me.DB.Query(gameSelect+` WHERE tournament_id=$1 ORDER BY start_time`, tournamentID)
	if err != nil {
		slog.Error("AllGames", "err", err)
		return nil
	}
	return me.scanGames(rows)
}

func (me *MyDB) ScoreGame(gid, hscore, ascore int) {
	_, err := me.DB.Exec(
		`UPDATE games SET home_score=$1, away_score=$2 WHERE id=$3`,
		hscore, ascore, gid,
	)
	if err != nil {
		slog.Error("ScoreGame", "err", err)
		return
	}
	game := me.ReturnGameByID(gid)
	me.DeleteTeamScore(game.ID)
	if game.ScrimmageTeamID != game.HomeTeam.ID {
		me.AddTeamScore(game.TournamentID, game.Division.ID, game.HomeTeam.ID, game.AwayTeam.ID, game.ID, hscore, ascore)
	}
	if game.ScrimmageTeamID != game.AwayTeam.ID {
		me.AddTeamScore(game.TournamentID, game.Division.ID, game.AwayTeam.ID, game.HomeTeam.ID, game.ID, ascore, hscore)
	}
}

func (me *MyDB) DeleteTeamScore(gameID int) {
	me.DB.Exec(`DELETE FROM games_by_team WHERE game_id=$1`, gameID)
}

func (me *MyDB) DidTeamABeatTeamB(teamAID, teamBID int) (bool, bool) {
	var teamAScore, teamBScore int
	err := me.DB.QueryRow(
		`SELECT team_score, opponent_score FROM games_by_team WHERE team_id=$1 AND opponent_id=$2 LIMIT 1`,
		teamAID, teamBID,
	).Scan(&teamAScore, &teamBScore)
	if err == sql.ErrNoRows {
		return false, false
	}
	if err != nil {
		slog.Error("DidTeamABeatTeamB", "err", err)
		return false, false
	}
	return true, teamAScore > teamBScore
}

func (me *MyDB) RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool) {
	var total int
	err := me.DB.QueryRow(
		`SELECT COALESCE(SUM(opponent_score),0) FROM games_by_team WHERE team_id=$1 AND opponent_id=$2`,
		teamAID, teamBID,
	).Scan(&total)
	if err == sql.ErrNoRows {
		return 0, false
	}
	if err != nil {
		slog.Error("RunsAllowedInHeadToHead", "err", err)
		return 0, false
	}
	// SUM returns 0 with no rows via COALESCE; we need to check actual existence.
	var count int
	me.DB.QueryRow(
		`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1 AND opponent_id=$2`,
		teamAID, teamBID,
	).Scan(&count)
	return total, count > 0
}

func (me *MyDB) IsGameScored(id int) bool {
	var n int
	me.DB.QueryRow(`SELECT COUNT(*) FROM games_by_team WHERE game_id=$1`, id).Scan(&n)
	return n == 2
}
