package mydb

import (
	"log/slog"
	"time"
)

func (me *MyDB) SetTournamentLeague(id int, kind string, endDate time.Time, leagueID int) {
	var end, lg any
	if !endDate.IsZero() {
		end = endDate
	}
	if leagueID != 0 {
		lg = leagueID
	}
	if _, err := me.DB.Exec(`UPDATE tournaments SET kind=$1, end_date=$2, league_id=$3 WHERE id=$4`, kind, end, lg, id); err != nil {
		slog.Error("SetTournamentLeague", "err", err)
	}
}

const tournamentSelect = `SELECT id, name, sport, location, start_date, notes, extras_html, rules_html, status, default_ranking_criteria, kind, end_date, COALESCE(league_id,0) FROM tournaments`

func (me *MyDB) queryTournaments(name, where string, args ...any) []Tournament {
	rows, err := me.DB.Query(tournamentSelect+" "+where, args...)
	if err != nil {
		slog.Error(name, "err", err)
		return nil
	}
	return scanTournaments(rows)
}

// ReturnLeagues lists published leagues that are running or upcoming: the end date is today or later, or
// there is no end date and the league started within the last 12 months.
func (me *MyDB) ReturnLeagues(today time.Time) []Tournament {
	d := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return me.queryTournaments("ReturnLeagues", `WHERE kind='league' AND status='published' AND
		((end_date IS NOT NULL AND end_date >= $1) OR (end_date IS NULL AND start_date >= $2))
		ORDER BY start_date DESC, id`, d, d.AddDate(0, -12, 0))
}

func (me *MyDB) LeaguePlayoffs(leagueID int) []Tournament {
	return me.queryTournaments("LeaguePlayoffs", `WHERE league_id=$1 ORDER BY start_date DESC, id`, leagueID)
}
