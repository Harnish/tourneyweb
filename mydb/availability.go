package mydb

import (
	"database/sql"
	"log/slog"
	"time"
)

// AvailabilityNotifyCooldown: after a withdrawal, volunteering again within this window does not notify
// the directors again, so a withdraw/volunteer loop can't spam them.
const AvailabilityNotifyCooldown = 24 * time.Hour

// Availability is an umpire's "I'm available" for a tournament, with an optional note.
// Withdrawn rows are kept as tombstones so the notification cooldown survives a withdrawal.
type Availability struct {
	UserID, TournamentID int
	Note                 string
	CreatedAt            time.Time
	Withdrawn            bool
	NotifiedAt           time.Time
}

// SetUmpireAvailability records availability and reports whether the directors should be notified:
// true for a first volunteer, or one returning after a withdrawal once the cooldown has passed.
func (me *MyDB) SetUmpireAvailability(userID, tournamentID int, note string) bool {
	tx, err := me.DB.Begin()
	if err != nil {
		slog.Error("SetUmpireAvailability", "err", err)
		return false
	}
	defer tx.Rollback()
	var withdrawn bool
	var notifiedAt time.Time
	err = tx.QueryRow(`SELECT withdrawn, notified_at FROM umpire_availability WHERE user_id=$1 AND tournament_id=$2 FOR UPDATE`,
		userID, tournamentID).Scan(&withdrawn, &notifiedAt)
	notify := false
	switch err {
	case sql.ErrNoRows:
		notify = true
		_, err = tx.Exec(`INSERT INTO umpire_availability (user_id, tournament_id, note) VALUES ($1,$2,$3)`, userID, tournamentID, note)
	case nil:
		notify = withdrawn && time.Since(notifiedAt) > AvailabilityNotifyCooldown
		_, err = tx.Exec(`UPDATE umpire_availability SET note=$3, withdrawn=false,
			notified_at = CASE WHEN $4 THEN NOW() ELSE notified_at END WHERE user_id=$1 AND tournament_id=$2`,
			userID, tournamentID, note, notify)
	}
	if err != nil {
		slog.Error("SetUmpireAvailability", "err", err)
		return false
	}
	if err := tx.Commit(); err != nil {
		slog.Error("SetUmpireAvailability commit", "err", err)
		return false
	}
	return notify
}

// ClearUmpireAvailability withdraws; the row stays as a tombstone for the notification cooldown.
func (me *MyDB) ClearUmpireAvailability(userID, tournamentID int) {
	if _, err := me.DB.Exec(`UPDATE umpire_availability SET withdrawn=true WHERE user_id=$1 AND tournament_id=$2`, userID, tournamentID); err != nil {
		slog.Error("ClearUmpireAvailability", "err", err)
	}
}

func (me *MyDB) queryAvailability(where string, arg int, key func(Availability) int) map[int]Availability {
	out := map[int]Availability{}
	rows, err := me.DB.Query(`SELECT user_id, tournament_id, note, created_at FROM umpire_availability WHERE NOT withdrawn AND `+where, arg)
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
		`SELECT id, name, sport, location, start_date, notes, extras_html, rules_html, status, default_ranking_criteria, kind, end_date, COALESCE(league_id,0)
		 FROM tournaments WHERE status <> 'draft' AND start_date >= $1::date ORDER BY start_date ASC, id ASC`,
		from.Format("2006-01-02"))
	if err != nil {
		slog.Error("ReturnTournamentsOpenForUmpires", "err", err)
		return nil
	}
	return scanTournaments(rows)
}
