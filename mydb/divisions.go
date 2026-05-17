package mydb

import (
	"database/sql"
	"encoding/json"
	"log/slog"
)

// DefaultRankingCriteria is the fallback order used when a division has no
// stored ranking configuration.
var DefaultRankingCriteria = []string{"wins", "head_to_head", "runs_against", "runs_for"}

type Division struct {
	ID              int
	TournamentID    int
	Name            string
	RulesHTML       string
	RankingCriteria []string
}

func parseCriteria(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return DefaultRankingCriteria
	}
	var out []string
	if err := json.Unmarshal([]byte(s.String), &out); err != nil || len(out) == 0 {
		return DefaultRankingCriteria
	}
	return out
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

func (me *MyDB) UpdateDivision(id int, name string, criteria []string) {
	b, _ := json.Marshal(criteria)
	_, err := me.DB.Exec(
		`UPDATE divisions SET name=$1, ranking_criteria=$2 WHERE id=$3`,
		name, string(b), id,
	)
	if err != nil {
		slog.Error("UpdateDivision", "err", err)
	}
}

func (me *MyDB) ReturnDivisions(tournamentID int) []Division {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, rules_html, ranking_criteria FROM divisions WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		slog.Error("ReturnDivisions", "err", err)
		return nil
	}
	var out []Division
	for rows.Next() {
		var d Division
		var crit sql.NullString
		if err := rows.Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML, &crit); err != nil {
			slog.Error("ReturnDivisions scan", "err", err)
			continue
		}
		d.RankingCriteria = parseCriteria(crit)
		out = append(out, d)
	}
	rows.Close()
	return out
}

func (me *MyDB) ReturnDivisionByID(id int) Division {
	var d Division
	var crit sql.NullString
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name, rules_html, ranking_criteria FROM divisions WHERE id=$1`, id,
	).Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML, &crit)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ReturnDivisionByID", "err", err)
	}
	d.RankingCriteria = parseCriteria(crit)
	return d
}

func (me *MyDB) SetDivisionRules(id int, html string) {
	_, err := me.DB.Exec(`UPDATE divisions SET rules_html=$1 WHERE id=$2`, html, id)
	if err != nil {
		slog.Error("SetDivisionRules", "err", err)
	}
}
