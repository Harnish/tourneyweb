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
	Phase           string
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

func (me *MyDB) AddDivision(tournamentID int, name string) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO divisions (tournament_id, name) VALUES ($1,$2) RETURNING id`,
		tournamentID, name,
	).Scan(&id)
	if err != nil {
		slog.Error("AddDivision", "err", err)
	}
	return id
}

func (me *MyDB) DelDivision(id int) {
	me.DB.Exec(`DELETE FROM divisions WHERE id=$1`, id)
}

func (me *MyDB) UpdateDivision(id int, name string, criteria []string) {
	var critVal interface{}
	if !criteriaEqualDefault(criteria) {
		b, _ := json.Marshal(criteria)
		critVal = string(b)
	}
	_, err := me.DB.Exec(
		`UPDATE divisions SET name=$1, ranking_criteria=$2 WHERE id=$3`,
		name, critVal, id,
	)
	if err != nil {
		slog.Error("UpdateDivision", "err", err)
	}
}

func criteriaEqualDefault(c []string) bool {
	if len(c) != len(DefaultRankingCriteria) {
		return false
	}
	for i := range c {
		if c[i] != DefaultRankingCriteria[i] {
			return false
		}
	}
	return true
}

func (me *MyDB) ReturnDivisions(tournamentID int) []Division {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, rules_html, ranking_criteria, COALESCE(phase,'pool') FROM divisions WHERE tournament_id=$1 ORDER BY name`,
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
		if err := rows.Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML, &crit, &d.Phase); err != nil {
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
		`SELECT id, tournament_id, name, rules_html, ranking_criteria, COALESCE(phase,'pool') FROM divisions WHERE id=$1`, id,
	).Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML, &crit, &d.Phase)
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

func (me *MyDB) SetDivisionPhase(id int, phase string) {
	_, err := me.DB.Exec(`UPDATE divisions SET phase=$1 WHERE id=$2`, phase, id)
	if err != nil {
		slog.Error("SetDivisionPhase", "err", err)
	}
}
