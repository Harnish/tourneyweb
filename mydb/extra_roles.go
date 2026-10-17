package mydb

import (
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"
)

var (
	ErrRoleNameInvalid = errors.New("role name must be 1-40 characters")
	ErrRoleNameTaken   = errors.New("a role with that name already exists in this tournament")
)

// ExtraRole is a free-form per-tournament game role (scorekeeper, scoreboard runner, safety...).
type ExtraRole struct {
	ID, TournamentID int
	Name             string
}

func cleanRoleName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 1 || n > 40 {
		return "", ErrRoleNameInvalid
	}
	return s, nil
}

func clampRoleNeeded(n int) int {
	if n < 0 {
		return 0
	}
	if n > 5 {
		return 5
	}
	return n
}

func (me *MyDB) AddExtraRole(tournamentID int, name string) (int, error) {
	name, err := cleanRoleName(name)
	if err != nil {
		return 0, err
	}
	var id int
	err = me.DB.QueryRow(`INSERT INTO extra_roles (tournament_id, name) VALUES ($1,$2)
		ON CONFLICT DO NOTHING RETURNING id`, tournamentID, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrRoleNameTaken
	}
	return id, err
}

func (me *MyDB) RenameExtraRole(tournamentID, roleID int, name string) error {
	name, err := cleanRoleName(name)
	if err != nil {
		return err
	}
	var taken bool
	me.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM extra_roles WHERE tournament_id=$1 AND lower(name)=lower($2) AND id<>$3)`,
		tournamentID, name, roleID).Scan(&taken)
	if taken {
		return ErrRoleNameTaken
	}
	_, err = me.DB.Exec(`UPDATE extra_roles SET name=$1 WHERE id=$2 AND tournament_id=$3`, name, roleID, tournamentID)
	return err
}

func (me *MyDB) DelExtraRole(tournamentID, roleID int) {
	if _, err := me.DB.Exec(`DELETE FROM extra_roles WHERE id=$1 AND tournament_id=$2`, roleID, tournamentID); err != nil {
		slog.Error("DelExtraRole", "err", err)
	}
}

func (me *MyDB) TournamentExtraRoles(tournamentID int) []ExtraRole {
	rows, err := me.DB.Query(`SELECT id, tournament_id, name FROM extra_roles WHERE tournament_id=$1 ORDER BY lower(name), id`, tournamentID)
	if err != nil {
		slog.Error("TournamentExtraRoles", "err", err)
		return nil
	}
	defer rows.Close()
	var out []ExtraRole
	for rows.Next() {
		var r ExtraRole
		if rows.Scan(&r.ID, &r.TournamentID, &r.Name) == nil {
			out = append(out, r)
		}
	}
	return out
}

func (me *MyDB) ExtraRoleByID(roleID int) ExtraRole {
	var r ExtraRole
	me.DB.QueryRow(`SELECT id, tournament_id, name FROM extra_roles WHERE id=$1`, roleID).Scan(&r.ID, &r.TournamentID, &r.Name)
	return r
}

func (me *MyDB) DivisionRoleNeeds(divisionID int) map[int]int {
	out := map[int]int{}
	rows, err := me.DB.Query(`SELECT role_id, needed FROM division_role_needs WHERE division_id=$1 AND needed>0`, divisionID)
	if err != nil {
		slog.Error("DivisionRoleNeeds", "err", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var rid, n int
		if rows.Scan(&rid, &n) == nil {
			out[rid] = n
		}
	}
	return out
}

func (me *MyDB) SetDivisionRoleNeeded(divisionID, roleID, n int) {
	n = clampRoleNeeded(n)
	var err error
	if n == 0 {
		_, err = me.DB.Exec(`DELETE FROM division_role_needs WHERE division_id=$1 AND role_id=$2`, divisionID, roleID)
	} else {
		_, err = me.DB.Exec(`INSERT INTO division_role_needs (division_id, role_id, needed) VALUES ($1,$2,$3)
			ON CONFLICT (division_id, role_id) DO UPDATE SET needed=$3`, divisionID, roleID, n)
	}
	if err != nil {
		slog.Error("SetDivisionRoleNeeded", "err", err)
	}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchUsers finds users whose name contains q (at least 2 characters). Email is deliberately not searchable:
// matching on it would let a caller probe which addresses have accounts.
func (me *MyDB) SearchUsers(q string, limit int) []User {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) < 2 || limit <= 0 {
		return nil
	}
	rows, err := me.DB.Query(`SELECT id, email, name FROM users
		WHERE name ILIKE $1 ORDER BY lower(name), id LIMIT $2`, "%"+likeEscaper.Replace(q)+"%", limit)
	if err != nil {
		slog.Error("SearchUsers", "err", err)
		return nil
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Email, &u.Name) == nil {
			out = append(out, u)
		}
	}
	return out
}
