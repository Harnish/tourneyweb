package mydb

import (
	"database/sql"
	"log/slog"
)

type Player struct {
	ID       int
	TeamID   int
	Number   string
	First    string
	Last     string
	Handed   string
	Position string
}

func (p Player) DisplayName() string {
	if len([]rune(p.First)) == 0 {
		return ". " + p.Last
	}
	return string([]rune(p.First)[:1]) + ". " + p.Last
}

func (me *MyDB) AddPlayer(teamID int, number, first, last, handed, position string) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO players (team_id, number, first_name, last_name, handed, position) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		teamID, number, first, last, handed, position,
	).Scan(&id)
	if err != nil {
		slog.Error("AddPlayer", "err", err)
	}
	return id
}

func (me *MyDB) GetPlayersByTeamID(teamID int) []Player {
	rows, err := me.DB.Query(
		`SELECT id, team_id, number, first_name, last_name, handed, position FROM players WHERE team_id=$1 ORDER BY last_name, first_name`,
		teamID,
	)
	if err != nil {
		slog.Error("GetPlayersByTeamID", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Number, &p.First, &p.Last, &p.Handed, &p.Position); err != nil {
			slog.Error("GetPlayersByTeamID scan", "err", err)
			continue
		}
		out = append(out, p)
	}
	return out
}

func (me *MyDB) GetPlayerByID(id int) (Player, bool) {
	var p Player
	err := me.DB.QueryRow(
		`SELECT id, team_id, number, first_name, last_name, handed, position FROM players WHERE id=$1`,
		id,
	).Scan(&p.ID, &p.TeamID, &p.Number, &p.First, &p.Last, &p.Handed, &p.Position)
	if err == sql.ErrNoRows {
		return Player{}, false
	}
	if err != nil {
		slog.Error("GetPlayerByID", "err", err)
		return Player{}, false
	}
	return p, true
}

func (me *MyDB) UpdatePlayer(id int, number, first, last, handed, position string) {
	_, err := me.DB.Exec(
		`UPDATE players SET number=$1, first_name=$2, last_name=$3, handed=$4, position=$5 WHERE id=$6`,
		number, first, last, handed, position, id,
	)
	if err != nil {
		slog.Error("UpdatePlayer", "err", err)
	}
}

func (me *MyDB) DeletePlayer(id int) {
	_, err := me.DB.Exec(`DELETE FROM players WHERE id=$1`, id)
	if err != nil {
		slog.Error("DeletePlayer", "err", err)
	}
}
