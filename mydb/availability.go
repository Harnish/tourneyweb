package mydb

import (
	"log/slog"
	"time"
)

// Availability is an umpire's "I'm available" for a tournament, with an optional note.
type Availability struct {
	UserID, TournamentID int
	Note                 string
	CreatedAt            time.Time
}

// SetUmpireAvailability upserts the volunteer row; isNew is false when only the note changed.
func (me *MyDB) SetUmpireAvailability(userID, tournamentID int, note string) bool {
	var isNew bool
	err := me.DB.QueryRow(`INSERT INTO umpire_availability (user_id, tournament_id, note) VALUES ($1,$2,$3)
		ON CONFLICT (user_id, tournament_id) DO UPDATE SET note = EXCLUDED.note
		RETURNING (xmax = 0)`, userID, tournamentID, note).Scan(&isNew)
	if err != nil {
		slog.Error("SetUmpireAvailability", "err", err)
		return false
	}
	return isNew
}

func (me *MyDB) ClearUmpireAvailability(userID, tournamentID int) {
	if _, err := me.DB.Exec(`DELETE FROM umpire_availability WHERE user_id=$1 AND tournament_id=$2`, userID, tournamentID); err != nil {
		slog.Error("ClearUmpireAvailability", "err", err)
	}
}

func (me *MyDB) queryAvailability(where string, arg int, key func(Availability) int) map[int]Availability {
	out := map[int]Availability{}
	rows, err := me.DB.Query(`SELECT user_id, tournament_id, note, created_at FROM umpire_availability WHERE `+where, arg)
	if err != nil {
		slog.Error("queryAvailability", "err", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var a Availability
		if rows.Scan(&a.UserID, &a.TournamentID, &a.Note, &a.CreatedAt) == nil {
			out[key(a)] = a
		}
	}
	return out
}

func (me *MyDB) UmpireAvailabilityForUser(userID int) map[int]Availability {
	return me.queryAvailability(`user_id=$1`, userID, func(a Availability) int { return a.TournamentID })
}

func (me *MyDB) UmpireAvailabilityForTournament(tournamentID int) map[int]Availability {
	return me.queryAvailability(`tournament_id=$1`, tournamentID, func(a Availability) int { return a.UserID })
}

// ReturnTournamentsOpenForUmpires lists non-draft tournaments starting on or after from's date, soonest first.
func (me *MyDB) ReturnTournamentsOpenForUmpires(from time.Time) []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, rules_html, status, default_ranking_criteria
		 FROM tournaments WHERE status <> 'draft' AND start_date >= $1::date ORDER BY start_date ASC, id ASC`,
		from.Format("2006-01-02"))
	if err != nil {
		slog.Error("ReturnTournamentsOpenForUmpires", "err", err)
		return nil
	}
	return scanTournaments(rows)
}
