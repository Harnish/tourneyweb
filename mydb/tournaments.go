package mydb

import (
	"database/sql"
	"log"
	"time"
)

type Tournament struct {
	ID        int
	Name      string
	Sport     string
	Location  string
	StartDate time.Time
	Notes     string
}

func scanTournaments(rows *sql.Rows) []Tournament {
	var out []Tournament
	for rows.Next() {
		var t Tournament
		if err := rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes); err != nil {
			log.Println("scanTournaments:", err)
			continue
		}
		out = append(out, t)
	}
	rows.Close()
	return out
}

func (me *MyDB) AddTournament(name, sport, location, notes string, date time.Time) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO tournaments (name, sport, location, start_date, notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		name, sport, location, date, notes,
	).Scan(&id)
	if err != nil {
		log.Println("AddTournament:", err)
	}
	return id
}

func (me *MyDB) ReturnTournaments() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments ORDER BY start_date DESC`,
	)
	if err != nil {
		log.Println("ReturnTournaments:", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentByID(id int) Tournament {
	var t Tournament
	err := me.DB.QueryRow(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE id=$1`, id,
	).Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes)
	if err != nil && err != sql.ErrNoRows {
		log.Println("ReturnTournamentByID:", err)
	}
	return t
}

func (me *MyDB) ReturnTournamentsComingUp() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date >= CURRENT_DATE AND start_date <= CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC`,
	)
	if err != nil {
		log.Println("ReturnTournamentsComingUp:", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentsRecent() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date >= CURRENT_DATE - INTERVAL '7 days' AND start_date < CURRENT_DATE ORDER BY start_date DESC`,
	)
	if err != nil {
		log.Println("ReturnTournamentsRecent:", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentsFuture(page int) ([]Tournament, int) {
	if page < 1 {
		page = 1
	}
	var total int
	if err := me.DB.QueryRow(`SELECT COUNT(*) FROM tournaments WHERE start_date > CURRENT_DATE + INTERVAL '7 days'`).Scan(&total); err != nil {
		log.Println("ReturnTournamentsFuture count:", err)
	}
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date > CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC LIMIT 20 OFFSET $1`,
		(page-1)*20,
	)
	if err != nil {
		log.Println("ReturnTournamentsFuture:", err)
		return nil, total
	}
	return scanTournaments(rows), total
}

func (me *MyDB) ReturnTournamentsPast(page int) ([]Tournament, int) {
	if page < 1 {
		page = 1
	}
	var total int
	if err := me.DB.QueryRow(`SELECT COUNT(*) FROM tournaments WHERE start_date < CURRENT_DATE - INTERVAL '7 days'`).Scan(&total); err != nil {
		log.Println("ReturnTournamentsPast count:", err)
	}
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date < CURRENT_DATE - INTERVAL '7 days' ORDER BY start_date DESC LIMIT 20 OFFSET $1`,
		(page-1)*20,
	)
	if err != nil {
		log.Println("ReturnTournamentsPast:", err)
		return nil, total
	}
	return scanTournaments(rows), total
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
		log.Println("UpdateTournament:", err)
	}
}
