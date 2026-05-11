package mydb

import (
	"database/sql"
	"log"
)

type Location struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

func (me *MyDB) AddLocation(name, address string, lat, lng float64) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO locations (name, address, latitude, longitude) VALUES ($1,$2,$3,$4) RETURNING id`,
		name, address, lat, lng,
	).Scan(&id)
	if err != nil {
		log.Println("AddLocation:", err)
	}
	return id
}

func (me *MyDB) GetLocations() []Location {
	rows, err := me.DB.Query(`SELECT id, name, address, latitude, longitude FROM locations ORDER BY name`)
	if err != nil {
		log.Println("GetLocations:", err)
		return nil
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude); err != nil {
			log.Println("GetLocations scan:", err)
			continue
		}
		out = append(out, l)
	}
	return out
}

func (me *MyDB) GetLocationByID(id int) Location {
	var l Location
	err := me.DB.QueryRow(
		`SELECT id, name, address, latitude, longitude FROM locations WHERE id=$1`, id,
	).Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude)
	if err != nil && err != sql.ErrNoRows {
		log.Println("GetLocationByID:", err)
	}
	return l
}

func (me *MyDB) UpdateLocation(id int, name, address string, lat, lng float64) {
	_, err := me.DB.Exec(
		`UPDATE locations SET name=$1, address=$2, latitude=$3, longitude=$4 WHERE id=$5`,
		name, address, lat, lng, id,
	)
	if err != nil {
		log.Println("UpdateLocation:", err)
	}
}

func (me *MyDB) DelLocation(id int) {
	me.DB.Exec(`DELETE FROM locations WHERE id=$1`, id)
}
