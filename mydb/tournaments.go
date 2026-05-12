package mydb

import (
	"database/sql"
	"log/slog"
	"time"
)

type Tournament struct {
	ID         int
	Name       string
	Sport      string
	Location   string
	StartDate  time.Time
	Notes      string
	ExtrasHTML string
	Status     string
}

func scanTournaments(rows *sql.Rows) []Tournament {
	var out []Tournament
	for rows.Next() {
		var t Tournament
		if err := rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.Status); err != nil {
			slog.Error("scanTournaments", "err", err)
			continue
		}
		out = append(out, t)
	}
	rows.Close()
	return out
}

func (me *MyDB) AddTournament(name, sport, location, notes string, date time.Time, status string) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO tournaments (name, sport, location, start_date, notes, status) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		name, sport, location, date, notes, status,
	).Scan(&id)
	if err != nil {
		slog.Error("AddTournament", "err", err)
	}
	return id
}

func (me *MyDB) ReturnTournaments() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments ORDER BY start_date DESC`,
	)
	if err != nil {
		slog.Error("ReturnTournaments", "err", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentByID(id int) Tournament {
	var t Tournament
	err := me.DB.QueryRow(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE id=$1`, id,
	).Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.Status)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ReturnTournamentByID", "err", err)
	}
	return t
}

func (me *MyDB) ReturnTournamentsComingUp() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE status='published' AND start_date >= CURRENT_DATE AND start_date <= CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC`,
	)
	if err != nil {
		slog.Error("ReturnTournamentsComingUp", "err", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentsRecent() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE status='published' AND start_date >= CURRENT_DATE - INTERVAL '7 days' AND start_date < CURRENT_DATE ORDER BY start_date DESC`,
	)
	if err != nil {
		slog.Error("ReturnTournamentsRecent", "err", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentsFuture(page int) ([]Tournament, int) {
	if page < 1 {
		page = 1
	}
	var total int
	if err := me.DB.QueryRow(`SELECT COUNT(*) FROM tournaments WHERE status='published' AND start_date > CURRENT_DATE + INTERVAL '7 days'`).Scan(&total); err != nil {
		slog.Error("ReturnTournamentsFuture count", "err", err)
	}
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE status='published' AND start_date > CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC LIMIT 20 OFFSET $1`,
		(page-1)*20,
	)
	if err != nil {
		slog.Error("ReturnTournamentsFuture", "err", err)
		return nil, total
	}
	return scanTournaments(rows), total
}

func (me *MyDB) ReturnTournamentsPast(page int) ([]Tournament, int) {
	if page < 1 {
		page = 1
	}
	var total int
	if err := me.DB.QueryRow(`SELECT COUNT(*) FROM tournaments WHERE status='published' AND start_date < CURRENT_DATE - INTERVAL '7 days'`).Scan(&total); err != nil {
		slog.Error("ReturnTournamentsPast count", "err", err)
	}
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE status='published' AND start_date < CURRENT_DATE - INTERVAL '7 days' ORDER BY start_date DESC LIMIT 20 OFFSET $1`,
		(page-1)*20,
	)
	if err != nil {
		slog.Error("ReturnTournamentsPast", "err", err)
		return nil, total
	}
	return scanTournaments(rows), total
}

func (me *MyDB) ReturnDraftTournaments() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE status='draft' ORDER BY start_date ASC`,
	)
	if err != nil {
		slog.Error("ReturnDraftTournaments", "err", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) DelTournament(id int) {
	me.DB.Exec(`DELETE FROM tournaments WHERE id=$1`, id)
}

func (me *MyDB) UpdateTournament(id int, name, sport, location, notes string, date time.Time) {
	_, err := me.DB.Exec(
		`UPDATE tournaments SET name=$1, sport=$2, location=$3, notes=$4, start_date=$5 WHERE id=$6`,
		name, sport, location, notes, date, id,
	)
	if err != nil {
		slog.Error("UpdateTournament", "err", err)
	}
}

func (me *MyDB) SetTournamentExtras(id int, html string) {
	_, err := me.DB.Exec(`UPDATE tournaments SET extras_html=$1 WHERE id=$2`, html, id)
	if err != nil {
		slog.Error("SetTournamentExtras", "err", err)
	}
}
