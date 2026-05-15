package mydb

import (
	"database/sql"
	"log/slog"
)

type Division struct {
	ID           int
	TournamentID int
	Name         string
}

func (me *MyDB) AddDivision(tournamentID int, name string) {
	_, err := me.DB.Exec(
		`INSERT INTO divisions (tournament_id, name) VALUES ($1,$2)`,
		tournamentID, name,
	)
	if err != nil {
		slog.Error("AddDivision", "err", err)
	}
}

func (me *MyDB) DelDivision(id int) {
	me.DB.Exec(`DELETE FROM divisions WHERE id=$1`, id)
}

func (me *MyDB) UpdateDivision(id int, name string) {
	_, err := me.DB.Exec(
		`UPDATE divisions SET name=$1 WHERE id=$2`,
		name, id,
	)
	if err != nil {
		slog.Error("UpdateDivision", "err", err)
	}
}

func (me *MyDB) ReturnDivisions(tournamentID int) []Division {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name FROM divisions WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		slog.Error("ReturnDivisions", "err", err)
		return nil
	}
	var out []Division
	for rows.Next() {
		var d Division
		if err := rows.Scan(&d.ID, &d.TournamentID, &d.Name); err != nil {
			slog.Error("ReturnDivisions scan", "err", err)
			continue
		}
		out = append(out, d)
	}
	rows.Close()
	return out
}

func (me *MyDB) ReturnDivisionByID(id int) Division {
	var d Division
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name FROM divisions WHERE id=$1`, id,
	).Scan(&d.ID, &d.TournamentID, &d.Name)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ReturnDivisionByID", "err", err)
	}
	return d
}
