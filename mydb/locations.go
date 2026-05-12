package mydb

import (
	"database/sql"
	"log/slog"
)

type Location struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	Latitude     float64 `json:"lat"`
	Longitude    float64 `json:"lng"`
	AvailableFor string  `json:"available_for"`
	TournamentID int     `json:"tournament_id"`
}

func (me *MyDB) AddLocation(name, address, availableFor string, lat, lng float64, tournamentID int) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO locations (name, address, available_for, latitude, longitude, tournament_id)
		 VALUES ($1,$2,$3,$4,$5, NULLIF($6, 0)) RETURNING id`,
		name, address, availableFor, lat, lng, tournamentID,
	).Scan(&id)
	if err != nil {
		slog.Error("AddLocation", "err", err)
	}
	return id
}

func (me *MyDB) GetLocations() []Location {
	rows, err := me.DB.Query(
		`SELECT id, name, address, latitude, longitude, available_for, COALESCE(tournament_id, 0) FROM locations ORDER BY name`)
	if err != nil {
		slog.Error("GetLocations", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude, &l.AvailableFor, &l.TournamentID); err != nil {
			slog.Error("GetLocations scan", "err", err)
			continue
		}
		out = append(out, l)
	}
	return out
}

func (me *MyDB) GetLocationByID(id int) Location {
	var l Location
	err := me.DB.QueryRow(
		`SELECT id, name, address, latitude, longitude, available_for, COALESCE(tournament_id, 0) FROM locations WHERE id=$1`, id,
	).Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude, &l.AvailableFor, &l.TournamentID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetLocationByID", "err", err)
	}
	return l
}

func (me *MyDB) GetLocationsByTournamentID(tournamentID int) []Location {
	rows, err := me.DB.Query(
		`SELECT id, name, address, latitude, longitude, available_for, tournament_id FROM locations WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		slog.Error("GetLocationsByTournamentID", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude, &l.AvailableFor, &l.TournamentID); err != nil {
			slog.Error("GetLocationsByTournamentID scan", "err", err)
			continue
		}
		out = append(out, l)
	}
	return out
}

func (me *MyDB) UpdateLocation(id int, name, address, availableFor string, lat, lng float64) {
	_, err := me.DB.Exec(
		`UPDATE locations SET name=$1, address=$2, available_for=$3, latitude=$4, longitude=$5 WHERE id=$6`,
		name, address, availableFor, lat, lng, id,
	)
	if err != nil {
		slog.Error("UpdateLocation", "err", err)
	}
}

func (me *MyDB) DelLocation(id int) {
	me.DB.Exec(`DELETE FROM locations WHERE id=$1`, id)
}
